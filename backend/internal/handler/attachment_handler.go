package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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

const maxUploadBytes = 10 << 20 // 10MB，够贴长截图

type AttachmentHandler struct {
	Handler
	UploadDir string // 落盘根目录，如 data/uploads
}

// Upload POST /api/attachments  (multipart, 字段名 file)
func (h *AttachmentHandler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	if file.Size > maxUploadBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file too large (max 10MB)"})
		return
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
		return
	}
	defer src.Close()

	// 只认文件头，不信任客户端给的扩展名
	head := make([]byte, 512)
	n, _ := src.Read(head)
	mime := http.DetectContentType(head[:n])
	ext, ok := allowedImageTypes[mime]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only png/jpeg/gif/webp images are allowed"})
		return
	}
	if _, err := src.Seek(0, 0); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
		return
	}

	// 按年月分目录，避免单目录堆太多文件
	sub := time.Now().Format("2006/01")
	dir := filepath.Join(h.UploadDir, sub)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create upload dir"})
		return
	}

	name := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	relPath := filepath.ToSlash(filepath.Join(sub, name))
	dst := filepath.Join(dir, name)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}

	attachment := model.Attachment{
		Filename:     name,
		OriginalName: filepath.Base(file.Filename),
		MimeType:     mime,
		Size:         file.Size,
		Path:         relPath,
	}
	if err := h.Repo.CreateAttachment(userID(c), &attachment); err != nil {
		respondError(c, err)
		return
	}

	attachment.URL = "/uploads/" + relPath
	c.JSON(http.StatusCreated, attachment)
}

// 便于测试/排查：判断上传目录是否已就绪
func EnsureUploadDir(dir string) error {
	return os.MkdirAll(strings.TrimRight(dir, "/"), 0o755)
}
