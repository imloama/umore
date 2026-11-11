// Package migrate 放「旧数据搬家」类的一次性迁移：数据库结构由 gorm 的
// AutoMigrate 管，这里只管结构管不了的事 —— 比如把老的上传文件从公开目录
// 搬到按人隔离的目录，并把正文里写死的 /uploads/ 地址换成鉴权接口。
package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/imloama/umore/internal/model"
	"github.com/imloama/umore/internal/repository"
)

// legacyURLRe 老版本的上传地址：/uploads/2026/11/1762xxx.png
// 抓的是 relPath，即相对 uploads 根目录的路径。
var legacyURLRe = regexp.MustCompile(`/uploads/(\d{4}/\d{2}/[A-Za-z0-9._-]+)`)

// AttachmentOptions 迁移参数
type AttachmentOptions struct {
	UploadDir string // 上传根目录，如 data/uploads
	UserID    int    // 归属人：老版本没有隔离，单机自用时这些文件都归一个账号
	DryRun    bool   // 只报告不动手
}

// AttachmentReport 迁移结果，给 CLI 打印用
type AttachmentReport struct {
	Scanned   int      // 待处理的旧文件数（磁盘上的 + 库里路径还是老格式的）
	Moved     int      // 搬成功的文件
	Missing   int      // 库里/正文里有，磁盘上找不到
	Rewritten int      // 被改写的正文（任务内容 + 评论）
	DryRun    bool     //
	Details   []string // 每个文件一行，便于核对
	Problems  []string // 没能处理干净的，人肉看一下
}

// MigrateAttachments 把老版本的上传文件搬进按人隔离的目录。
//
// 老版本（23e8ae9 之前）是这样落盘的：
//
//	文件   uploads/<年>/<月>/<纳秒>.<ext>      ← 整个目录被 r.Static 裸奔
//	表里   attachments.path = "<年>/<月>/<纳秒>.<ext>"，URL = "/uploads/" + path
//	正文   ![截图](/uploads/2026/11/1762xxx.png)
//
// 现在要求 uploads/<user_id>/<年>/<月>/<随机名>，读走 GET /api/attachments/:id。
// 所以一次要做三件事：
//
//  1. 建 attachments 行（老表没有 user_id，现在必须补上），按 年/月/文件名 对齐
//  2. 文件搬到 uploads/<user_id>/<年>/<月>/ 下，库里 path 跟着改
//  3. 任务内容、任务评论里写死的 /uploads/... 换成 /api/attachments/<id>
//
// 重复执行是安全的：已经搬过的文件在源位置就找不到了，只会算进 Missing 而不会重复插入。
func MigrateAttachments(repo *repository.Repository, opt AttachmentOptions) (*AttachmentReport, error) {
	report := &AttachmentReport{DryRun: opt.DryRun}
	uid := opt.UserID
	if uid <= 0 {
		return nil, fmt.Errorf("必须指定 user id：老版本的上传没有归属人，需要你决定这些文件算谁的")
	}

	// 老格式的路径 → 附件记录，正文改写时按这个查 id
	byLegacy := map[string]*model.Attachment{}

	// 1) 库里已有、但 path 还是老格式的记录（上次迁移中途失败之类）
	var legacyRows []model.Attachment
	if err := repo.DB().Where("user_id = ? AND path NOT LIKE ?", uid, strconv.Itoa(uid)+"/%").
		Order("id").Find(&legacyRows).Error; err != nil {
		return nil, fmt.Errorf("查询旧附件记录失败: %w", err)
	}

	// 2) 磁盘上还没记录的旧文件：uploads/<年>/<月>/<文件名>
	loose, err := looseFiles(opt.UploadDir)
	if err != nil {
		return nil, err
	}
	report.Scanned = len(loose) + len(legacyRows)

	for _, rel := range loose {
		byLegacy[rel] = &model.Attachment{
			UserID:       uid,
			Filename:     filepath.Base(rel),
			OriginalName: filepath.Base(rel), // 老版本没留原始名，只能用落盘名
			MimeType:     mimeOfPath(rel),
			Path:         rel,
		}
	}
	for i := range legacyRows {
		row := &legacyRows[i]
		if _, ok := byLegacy[row.Path]; !ok {
			byLegacy[row.Path] = row
		}
	}

	if len(byLegacy) == 0 {
		// 没有旧文件也不用改正文，直接返回
		return report, nil
	}

	// 3) 逐个搬：落新位置 → 写库 → 删旧文件
	// 顺序是「先有新文件，再删旧文件」，中途挂掉不会丢数据，重跑接着来
	index := legacyIndex(repo, uid) // 老路径 → 附件 id
	for legacyPath, att := range byLegacy {
		src := filepath.Join(opt.UploadDir, filepath.FromSlash(legacyPath))
		info, err := os.Stat(src)
		if err != nil || info.IsDir() {
			report.Missing++
			report.Problems = append(report.Problems, fmt.Sprintf("%s 磁盘上找不到，跳过", legacyPath))
			continue
		}
		if att.ID == 0 {
			att.Size = info.Size()
			att.MimeType = mimeOfPath(legacyPath)
		}

		dir := filepath.Join(strconv.Itoa(uid), info.ModTime().Format("2006/01"))
		newPath := filepath.ToSlash(filepath.Join(dir, filepath.Base(legacyPath)))
		dst := filepath.Join(opt.UploadDir, filepath.FromSlash(newPath))

		if opt.DryRun {
			report.Moved++
			report.Details = append(report.Details, fmt.Sprintf("%s → %s", legacyPath, newPath))
			continue
		}

		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			report.Problems = append(report.Problems, fmt.Sprintf("%s 建目录失败: %v", legacyPath, err))
			continue
		}
		if err := moveFile(src, dst); err != nil {
			report.Problems = append(report.Problems, fmt.Sprintf("%s 移动失败: %v", legacyPath, err))
			continue
		}

		if att.ID == 0 {
			att.Path = newPath
			if err := repo.CreateAttachment(uid, att); err != nil {
				// 入库失败就把文件挪回去，别让库和磁盘对不上
				_ = moveFile(dst, src)
				report.Problems = append(report.Problems, fmt.Sprintf("%s 入库失败: %v", legacyPath, err))
				continue
			}
		} else {
			// 已有记录：只更新路径，归属人保持
			if err := repo.DB().Model(&model.Attachment{}).
				Where("id = ?", att.ID).Update("path", newPath).Error; err != nil {
				_ = moveFile(dst, src)
				report.Problems = append(report.Problems, fmt.Sprintf("附件 %d 更新路径失败: %v", att.ID, err))
				continue
			}
		}

		index[legacyPath] = att.ID
		report.Moved++
		report.Details = append(report.Details, fmt.Sprintf("%s → %s (#%d)", legacyPath, newPath, att.ID))
	}

	if opt.DryRun {
		return report, nil
	}

	// 4) 删空的年月目录，免得 uploads 里留一堆空壳
	pruneEmptyDirs(opt.UploadDir)

	// 5) 改正文：任务内容 + 评论里的 /uploads/<年>/<月>/xxx
	if err := rewriteBodies(repo, index, report); err != nil {
		return report, err
	}
	return report, nil
}

// looseFiles 找出 uploads 根下老格式的文件（<年>/<月>/<文件名>，且不在用户子目录里）。
// 用户目录是纯数字（<user_id>），年月也是纯数字，靠层级区分：老格式正好两层。
func looseFiles(root string) ([]string, error) {
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	years, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, y := range years {
		if !y.IsDir() || len(y.Name()) != 4 || !isDigits(y.Name()) {
			continue
		}
		months, err := os.ReadDir(filepath.Join(root, y.Name()))
		if err != nil {
			continue
		}
		for _, m := range months {
			if !m.IsDir() || len(m.Name()) != 2 || !isDigits(m.Name()) {
				continue
			}
			files, err := os.ReadDir(filepath.Join(root, y.Name(), m.Name()))
			if err != nil {
				continue
			}
			for _, f := range files {
				if f.IsDir() {
					continue
				}
				out = append(out, filepath.ToSlash(filepath.Join(y.Name(), m.Name(), f.Name())))
			}
		}
	}
	return out, nil
}

// legacyIndex 老路径（年/月/文件名）→ 附件 id。
//
// 取库里 path 结尾是 年/月/文件名 的记录：迁移跑过一遍之后再跑，
// 文件已经在 <user_id>/年/月/ 下了（或者本来就搬过），这时正文还指望着老地址的，
// 也要能对回同一个 id —— 否则重复执行会漏改。
func legacyIndex(repo *repository.Repository, uid int) map[string]int {
	index := map[string]int{}
	var rows []model.Attachment
	if err := repo.DB().Where("user_id = ?", uid).Find(&rows).Error; err != nil {
		return index
	}
	for _, a := range rows {
		rel := strings.TrimPrefix(a.Path, strconv.Itoa(uid)+"/")
		if legacyURLRe.MatchString("/uploads/" + rel) {
			if _, ok := index[rel]; !ok {
				index[rel] = a.ID
			}
		}
	}
	return index
}

// rewriteBodies 把任务内容、任务评论里的老地址换成 /api/attachments/<id>。
// 认不出来的（磁盘没了、库里也没记录）原样留着，改错了比不改更难查。
func rewriteBodies(repo *repository.Repository, index map[string]int, report *AttachmentReport) error {
	for _, table := range []string{"tasks", "comments"} {
		var rows []struct {
			ID      int
			Content string
		}
		if err := repo.DB().Table(table).
			Select("id, content").
			Where("content LIKE ?", "%/uploads/%").
			Find(&rows).Error; err != nil {
			return fmt.Errorf("读取 %s 正文失败: %w", table, err)
		}
		for _, row := range rows {
			next, changed, lost := replaceLegacyURLs(row.Content, index)
			for _, rel := range lost {
				report.Problems = append(report.Problems,
					fmt.Sprintf("%s#%d 里的 %s 找不到对应附件，原样保留", table, row.ID, rel))
			}
			if !changed {
				continue
			}
			if err := repo.DB().Table(table).Where("id = ?", row.ID).
				Update("content", next).Error; err != nil {
				return fmt.Errorf("更新 %s#%d 失败: %w", table, row.ID, err)
			}
			report.Rewritten++
		}
	}
	return nil
}

// replaceLegacyURLs 把一段正文里的 /uploads/<年>/<月>/xxx 换掉，
// 返回新正文、是否有改动、以及没能对上附件的老路径。
func replaceLegacyURLs(src string, index map[string]int) (string, bool, []string) {
	var changed bool
	var lost []string
	out := legacyURLRe.ReplaceAllStringFunc(src, func(m string) string {
		rel := legacyURLRe.FindStringSubmatch(m)[1]
		id, ok := index[rel]
		if !ok || id <= 0 {
			lost = append(lost, rel)
			return m
		}
		changed = true
		return fmt.Sprintf("/api/attachments/%d", id)
	})
	return out, changed, lost
}

// moveFile 跨盘也能用的“移动”：先重命名，不行就复制后删源
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return err
	}
	return os.Remove(src)
}

// pruneEmptyDirs 删掉 uploads 下已经空掉的 <年>/<月>
func pruneEmptyDirs(root string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, y := range entries {
		if !y.IsDir() || len(y.Name()) != 4 || !isDigits(y.Name()) {
			continue
		}
		yDir := filepath.Join(root, y.Name())
		months, err := os.ReadDir(yDir)
		if err != nil {
			continue
		}
		for _, m := range months {
			if !m.IsDir() {
				continue
			}
			mDir := filepath.Join(yDir, m.Name())
			if left, err := os.ReadDir(mDir); err == nil && len(left) == 0 {
				_ = os.Remove(mDir)
			}
		}
		// 年份目录空了也一起删
		if left, err := os.ReadDir(yDir); err == nil && len(left) == 0 {
			_ = os.Remove(yDir)
		}
	}
}

// mimeOfPath 按扩展名猜类型。老版本只收图片，所以这里主要覆盖图片；
// 认识的类型都给上，Serve 时图片才 inline 显示。
func mimeOfPath(p string) string {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case ".md":
		return "text/markdown"
	case ".csv":
		return "text/csv"
	case ".json":
		return "application/json"
	case ".zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
