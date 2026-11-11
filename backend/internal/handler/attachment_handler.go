package handler

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/imloama/umore/internal/model"

	"github.com/gin-gonic/gin"
)

// 允许粘贴的图片类型：截图基本就是这几种
var allowedImageTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// 富文本里要挂的其它附件。只放行这几种：都是「点开就是本地文件」的格式，
// 不会被浏览器当页面执行（html/svg 一律不收，免得变成 XSS 的跳板）。
var allowedFileTypes = map[string]string{
	"application/pdf":  ".pdf",
	"application/zip":  ".zip",
	"text/plain":       ".txt",
	"text/csv":         ".csv",
	"text/markdown":    ".md",
	"application/json": ".json",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   ".docx",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         ".xlsx",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",
	"application/msword":       ".doc",
	"application/vnd.ms-excel": ".xls",
}

const (
	// maxUploadBytes 头像上限
	maxUploadBytes = 10 << 20
	// maxAttachBytes 附件上限：图片够贴长截图，文档也放得下
	maxAttachBytes = 20 << 20
)

type AttachmentHandler struct {
	Handler
	UploadDir string // 落盘根目录，如 data/uploads
}

// baseMimeType 去掉 "text/plain; charset=utf-8" 里的参数部分
func baseMimeType(full string) string {
	if i := strings.IndexByte(full, ';'); i >= 0 {
		full = full[:i]
	}
	return strings.TrimSpace(strings.ToLower(full))
}

// Upload POST /api/attachments  (multipart, 字段名 file)
// 附件按用户分目录存：uploads/<user_id>/<年>/<月>/<随机名>，
// 文件名里的随机串保证不可猜，读的时候还要再核对一次归属。
func (h *AttachmentHandler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, codeFileRequired, "file is required")
		return
	}
	if file.Size > maxAttachBytes {
		fail(c, http.StatusBadRequest, codeFileTooLarge, "file too large (max 20MB)")
		return
	}

	src, err := file.Open()
	if err != nil {
		fail(c, http.StatusInternalServerError, codeFileReadFailed, "failed to read file")
		return
	}
	defer src.Close()

	// 只认文件头，不信任客户端给的扩展名。
	// DetectContentType 会给纯文本带上 "; charset=utf-8"，查表前要把参数摘掉。
	head := make([]byte, 512)
	n, _ := src.Read(head)
	mime := baseMimeType(http.DetectContentType(head[:n]))
	ext, ok := allowedImageTypes[mime]
	if !ok {
		ext, ok = allowedFileTypes[mime]
	}
	if !ok {
		fail(c, http.StatusBadRequest, codeFileTypeRejected, "unsupported file type")
		return
	}
	if _, err := src.Seek(0, 0); err != nil {
		fail(c, http.StatusInternalServerError, codeFileReadFailed, "failed to read file")
		return
	}

	// 按 用户 / 年月 分目录，避免单目录堆太多文件，也方便按人清理
	uid := userID(c)
	sub := filepath.Join(strconv.Itoa(uid), time.Now().Format("2006/01"))
	dir := filepath.Join(h.UploadDir, sub)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fail(c, http.StatusInternalServerError, codeFileSaveFailed, "failed to create upload dir")
		return
	}

	name := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	relPath := filepath.ToSlash(filepath.Join(sub, name))
	dst := filepath.Join(dir, name)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		fail(c, http.StatusInternalServerError, codeFileSaveFailed, "failed to save file")
		_ = os.Remove(dst)
		return
	}

	attachment := model.Attachment{
		Filename:     name,
		OriginalName: filepath.Base(file.Filename),
		MimeType:     mime,
		Size:         file.Size,
		Path:         relPath,
	}
	if err := h.Repo.CreateAttachment(uid, &attachment); err != nil {
		// 落库失败就把文件删掉，别在磁盘上留孤儿文件
		_ = os.Remove(dst)
		respondError(c, err)
		return
	}

	attachment.URL = attachmentURL(attachment.ID)
	c.JSON(http.StatusCreated, attachment)
}

// Serve GET /api/attachments/:id
// 附件不走静态目录，也不允许匿名访问：只有上传人能取到自己的文件。
// 别人拿着 id 来要，返回 404 —— 连「存在」这件事都不告诉对方。
func (h *AttachmentHandler) Serve(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		fail(c, http.StatusNotFound, codeNotFound, "not found")
		return
	}
	att, err := h.Repo.GetAttachment(userID(c), id)
	if err != nil {
		respondError(c, err)
		return
	}
	if att == nil {
		fail(c, http.StatusNotFound, codeNotFound, "not found")
		return
	}

	// Path 是相对路径，拼之前先确认没跳出上传根目录
	clean := path.Clean("/" + att.Path)
	full := filepath.Join(h.UploadDir, filepath.FromSlash(clean))
	root, err := filepath.Abs(h.UploadDir)
	if err != nil {
		fail(c, http.StatusInternalServerError, codeInternal, "failed to resolve upload dir")
		return
	}
	abs, err := filepath.Abs(full)
	if err != nil || !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		fail(c, http.StatusNotFound, codeNotFound, "not found")
		return
	}
	if info, err := os.Stat(abs); err != nil || info.IsDir() {
		fail(c, http.StatusNotFound, codeNotFound, "not found")
		return
	}

	// 这些文件只属于当前用户，别让中间层（CDN、代理）缓存成公开资源
	c.Header("Cache-Control", "private, max-age=86400")
	c.Header("X-Content-Type-Options", "nosniff")
	// 图片内联显示，其它类型强制下载，免得被浏览器当页面渲染
	if _, isImage := allowedImageTypes[att.MimeType]; isImage {
		c.Header("Content-Disposition", "inline")
	} else {
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", att.OriginalName))
	}
	c.File(abs)
}

// legacyUploadPrefix 老版本的公开上传目录：/uploads/<年>/<月>/<文件>
// 那时候 r.Static("/uploads", …) 裸着对外，正文里写死了这种地址。
const legacyUploadPrefix = "/uploads/"

// ServeLegacy GET /uploads/<年>/<月>/<文件>
//
// 老版本的文件已经搬到 uploads/<user_id>/<年>/<月>/ 下（见 migrate attachments），
// 但地址可能还留在别处：浏览器书签、别的系统里贴过的链接、没改到的历史文本。
// 这里接住它们，按文件名在库里反查归属，再走和 /api/attachments/:id 一样的鉴权，
// 不留一条公开读的口子。
//
// 注意它挂在 /uploads/ 下而不是 /api 里：老链接都是这个形状，不能改。
func (h *AttachmentHandler) ServeLegacy(c *gin.Context) {
	// 老地址是 /uploads/2026/11/1762xxx.png
	tail := strings.TrimPrefix(c.Request.URL.Path, legacyUploadPrefix)
	if tail == "" || tail == c.Request.URL.Path {
		fail(c, http.StatusNotFound, codeNotFound, "not found")
		return
	}
	var att model.Attachment
	err := h.Repo.DB().Where("user_id = ? AND path LIKE ?", userID(c), "%"+path.Clean("/"+tail)).
		Order("id").First(&att).Error
	if err != nil {
		fail(c, http.StatusNotFound, codeNotFound, "not found")
		return
	}
	// 复用手上的同一套：路径校验、Cache-Control、Content-Disposition 都在 Serve 里
	c.Params = append(c.Params, gin.Param{Key: "id", Value: strconv.Itoa(att.ID)})
	h.Serve(c)
}

// attachmentURL 前端引用附件的唯一写法，鉴权靠 Authorization 头
func attachmentURL(id int) string {
	return fmt.Sprintf("/api/attachments/%d", id)
}

// 便于测试/排查：判断上传目录是否已就绪
func EnsureUploadDir(dir string) error {
	return os.MkdirAll(strings.TrimRight(dir, "/"), 0o755)
}
