package migrate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/imloama/umore/internal/model"
	"github.com/imloama/umore/internal/repository"
)

// 老库长这样：uploads/2026/11/1762xxx.png 直接裸在根目录下，
// attachments.path 存 年/月/文件名，正文里写死 /uploads/2026/11/1762xxx.png。
// 迁移之后应该是：文件在 <uid>/2026/11/ 下、路径带 uid、正文换成 /api/attachments/<id>。
func TestMigrateAttachments(t *testing.T) {
	repo := newRepo(t)
	uid := 1

	uploadDir := t.TempDir()
	oldRel := "2026/11/1762000000000000000.png"
	oldPath := filepath.Join(uploadDir, filepath.FromSlash(oldRel))
	if err := os.MkdirAll(filepath.Dir(oldPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, []byte("\x89PNG\r\n\x1a\nfake"), 0o644); err != nil {
		t.Fatal(err)
	}

	task := model.Task{
		UserID: uid, Title: "带截图的任务",
		Content: "看这里 ![截图](/uploads/" + oldRel + ")，还有 [附件](/uploads/2026/11/999.png)",
	}
	if err := repo.DB().Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	comment := model.Comment{
		UserID: uid, TargetType: "task", TargetID: task.ID,
		Content: "同一张图再贴一次 ![](/uploads/" + oldRel + ")",
	}
	if err := repo.DB().Create(&comment).Error; err != nil {
		t.Fatal(err)
	}

	report, err := MigrateAttachments(repo, AttachmentOptions{UploadDir: uploadDir, UserID: uid})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if report.Moved != 1 || report.Rewritten != 2 {
		t.Fatalf("搬 %d 个、改 %d 处正文，期望 1 和 2；problems=%v", report.Moved, report.Rewritten, report.Problems)
	}

	// 文件到位、旧位置清空
	newRel := "1/2026/11/1762000000000000000.png"
	if _, err := os.Stat(filepath.Join(uploadDir, filepath.FromSlash(newRel))); err != nil {
		t.Fatalf("新位置没有文件: %v", err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("旧文件应该已经被搬走")
	}

	// 库里能按人取到，路径带 uid
	var att model.Attachment
	if err := repo.DB().Where("user_id = ?", uid).First(&att).Error; err != nil {
		t.Fatalf("附件没入库: %v", err)
	}
	if att.Path != newRel {
		t.Fatalf("path = %q，期望 %q", att.Path, newRel)
	}
	if att.MimeType != "image/png" {
		t.Fatalf("mime = %q", att.MimeType)
	}

	// 正文里的地址换成鉴权接口；找不到的那个文件保留原样，并留一条提示
	var got model.Task
	repo.DB().First(&got, task.ID)
	want := "看这里 ![截图](/api/attachments/1)，还有 [附件](/uploads/2026/11/999.png)"
	if got.Content != want {
		t.Fatalf("任务内容 = %q，期望 %q", got.Content, want)
	}
	var gotComment model.Comment
	repo.DB().First(&gotComment, comment.ID)
	if gotComment.Content != "同一张图再贴一次 ![](/api/attachments/1)" {
		t.Fatalf("评论内容 = %q", gotComment.Content)
	}
	if len(report.Problems) != 1 {
		t.Fatalf("缺一个「999.png 找不到」的提示，got %v", report.Problems)
	}
}

// 连着跑两遍不该搬家第二遍、也不该重复插记录（发布脚本手抖重跑是常事）
func TestMigrateAttachmentsIdempotent(t *testing.T) {
	repo := newRepo(t)
	uploadDir := t.TempDir()
	oldRel := "2026/11/1762000000000000001.png"
	src := filepath.Join(uploadDir, filepath.FromSlash(oldRel))
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("\x89PNG\r\n\x1a\nfake"), 0o644); err != nil {
		t.Fatal(err)
	}
	task := model.Task{UserID: 1, Title: "t", Content: "![](/uploads/" + oldRel + ")"}
	repo.DB().Create(&task)

	first, err := MigrateAttachments(repo, AttachmentOptions{UploadDir: uploadDir, UserID: 1})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := MigrateAttachments(repo, AttachmentOptions{UploadDir: uploadDir, UserID: 1})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first.Moved != 1 || second.Moved != 0 {
		t.Fatalf("moved 第一遍 %d、第二遍 %d，期望 1 和 0", first.Moved, second.Moved)
	}
	var count int64
	repo.DB().Model(&model.Attachment{}).Count(&count)
	if count != 1 {
		t.Fatalf("附件记录 %d 条，期望 1 条", count)
	}
}

// dry-run 只报告：文件原地不动、库不写、正文不改
func TestMigrateAttachmentsDryRun(t *testing.T) {
	repo := newRepo(t)
	uploadDir := t.TempDir()
	oldRel := "2026/11/1762000000000000002.png"
	src := filepath.Join(uploadDir, filepath.FromSlash(oldRel))
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("\x89PNG\r\n\x1a\nfake"), 0o644); err != nil {
		t.Fatal(err)
	}
	task := model.Task{UserID: 1, Title: "t", Content: "![](/uploads/" + oldRel + ")"}
	repo.DB().Create(&task)

	report, err := MigrateAttachments(repo, AttachmentOptions{UploadDir: uploadDir, UserID: 1, DryRun: true})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if report.Moved != 1 || !report.DryRun {
		t.Fatalf("dry-run 应该报「会搬 1 个」，got %+v", report)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatal("dry-run 不该动文件")
	}
	var count int64
	repo.DB().Model(&model.Attachment{}).Count(&count)
	if count != 0 {
		t.Fatal("dry-run 不该入库")
	}
	var got model.Task
	repo.DB().First(&got, task.ID)
	if got.Content != task.Content {
		t.Fatal("dry-run 不该改正文")
	}
}

// 没给 --user 就得报错：这些文件算谁的必须人来定
func TestMigrateAttachmentsRequiresUser(t *testing.T) {
	repo := newRepo(t)
	if _, err := MigrateAttachments(repo, AttachmentOptions{UploadDir: t.TempDir()}); err == nil {
		t.Fatal("缺 user id 应该报错")
	}
}

// 正文里的老地址认不出来时不动手 —— 猜错了比不改更难查
func TestReplaceLegacyURLsKeepsUnknown(t *testing.T) {
	src := "![](/uploads/2026/11/1.png) and ![](/uploads/2026/11/2.png)"
	out, changed, lost := replaceLegacyURLs(src, map[string]int{"2026/11/1.png": 7})
	if !changed {
		t.Fatal("应该改了第一条")
	}
	if out != "![](/api/attachments/7) and ![](/uploads/2026/11/2.png)" {
		t.Fatalf("got %q", out)
	}
	if len(lost) != 1 || lost[0] != "2026/11/2.png" {
		t.Fatalf("lost = %v", lost)
	}
}

func newRepo(t *testing.T) *repository.Repository {
	t.Helper()
	repo, err := repository.NewRepository(filepath.Join(t.TempDir(), "migrate.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { repo.Close() })
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}
	return repo
}
