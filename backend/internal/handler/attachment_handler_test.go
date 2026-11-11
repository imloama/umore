package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/imloama/umore/internal/auth"
	"github.com/imloama/umore/internal/cache"
	"github.com/imloama/umore/internal/model"
	"github.com/imloama/umore/internal/repository"

	"github.com/gin-gonic/gin"
)

// newAttachmentRouter 两个用户各自的 token + 一个真实落盘目录。
// 附件隔离是这次安全整改的核心，所以按「A 传、B 取」的路子测。
func newAttachmentRouter(t *testing.T) (*gin.Engine, *AttachmentHandler, string, []string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	repo, err := repository.NewRepository(filepath.Join(t.TempDir(), "attachments.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { repo.Close() })
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	uploadDir := t.TempDir()
	if err := EnsureUploadDir(uploadDir); err != nil {
		t.Fatalf("upload dir: %v", err)
	}
	h := &AttachmentHandler{Handler: Handler{Repo: repo}, UploadDir: uploadDir}

	store := cache.NewMemory()
	t.Cleanup(func() { store.Close() })
	sessions := auth.NewManager(store, time.Hour)

	// 两个账号：u1 传，u2 不该看得见
	tokens := make([]string, 0, 2)
	for _, email := range []string{"a@example.com", "b@example.com"} {
		user, err := repo.CreateUser(email, "hash", "salt", true)
		if err != nil {
			t.Fatalf("create user %s: %v", email, err)
		}
		token, _, err := sessions.Issue(context.Background(), user.ID, user.Email)
		if err != nil {
			t.Fatalf("issue token: %v", err)
		}
		tokens = append(tokens, token)
	}

	r := gin.New()
	api := r.Group("/api", RequireAuth(sessions))
	api.POST("/attachments", h.Upload)
	api.GET("/attachments/:id", h.Serve)
	// 老版本的公开地址：迁移后仍要能打开，但必须还是本人的
	r.GET("/uploads/:year/:month/:file", RequireAuth(sessions), h.ServeLegacy)
	return r, h, uploadDir, tokens
}

// pngBytes 现造一张 1x1 的 PNG：DetectContentType 只认文件头，够用
func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func uploadAs(t *testing.T, r *gin.Engine, token, filename string, content []byte) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write part: %v", err)
	}
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/attachments", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w, out
}

func getAttachment(r *gin.Engine, token, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/attachments/"+id, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// 上传成功：URL 指向鉴权接口，文件落在自己的目录里
func TestUploadAttachment(t *testing.T) {
	r, _, uploadDir, tokens := newAttachmentRouter(t)

	w, body := uploadAs(t, r, tokens[0], "shot.png", pngBytes(t))
	if w.Code != http.StatusCreated {
		t.Fatalf("upload want 201, got %d: %s", w.Code, w.Body.String())
	}
	id := int(body["id"].(float64))
	if body["url"] != fmt.Sprintf("/api/attachments/%d", id) {
		t.Fatalf("附件 URL 不该是公开地址: %v", body["url"])
	}

	rel, _ := body["path"].(string)
	if rel == "" {
		t.Fatal("落盘路径没返回")
	}
	// 路径必须是 <user_id>/...：按人分目录，扫一个目录只看到一个用户的东西
	if !strings.HasPrefix(filepath.ToSlash(rel), "1/") {
		t.Fatalf("附件应按用户分目录，got %s", rel)
	}
	if _, err := os.Stat(filepath.Join(uploadDir, filepath.FromSlash(rel))); err != nil {
		t.Fatalf("文件没落盘: %v", err)
	}
}

// 只有上传者本人能取；别人拿到 id 也是 404，连存在性都不暴露
func TestAttachmentOnlyOwnerCanRead(t *testing.T) {
	r, _, _, tokens := newAttachmentRouter(t)

	_, body := uploadAs(t, r, tokens[0], "shot.png", pngBytes(t))
	id := strconv.Itoa(int(body["id"].(float64)))

	own := getAttachment(r, tokens[0], id)
	if own.Code != http.StatusOK {
		t.Fatalf("本人取附件 want 200, got %d", own.Code)
	}
	if own.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("图片应内联返回 image/png, got %q", own.Header().Get("Content-Type"))
	}
	if cc := own.Header().Get("Cache-Control"); cc != "private, max-age=86400" {
		t.Fatalf("附件不能被中间层当公开资源缓存, got %q", cc)
	}

	other := getAttachment(r, tokens[1], id)
	if other.Code != http.StatusNotFound {
		t.Fatalf("别人的附件要 404, got %d", other.Code)
	}

	// 没登录更不行
	anon := getAttachment(r, "", id)
	if anon.Code != http.StatusUnauthorized {
		t.Fatalf("匿名访问要 401, got %d", anon.Code)
	}
}

// 非图片附件：强制下载，别让浏览器当页面渲染
func TestUploadNonImageAttachment(t *testing.T) {
	r, _, _, tokens := newAttachmentRouter(t)

	w, body := uploadAs(t, r, tokens[0], "notes.txt", []byte("hello attachment"))
	if w.Code != http.StatusCreated {
		t.Fatalf("上传 txt want 201, got %d: %s", w.Code, w.Body.String())
	}
	id := strconv.Itoa(int(body["id"].(float64)))
	got := getAttachment(r, tokens[0], id)
	if got.Code != http.StatusOK {
		t.Fatalf("取 txt want 200, got %d", got.Code)
	}
	if disposition := got.Header().Get("Content-Disposition"); disposition == "inline" || disposition == "" {
		t.Fatalf("非图片附件要强制下载, got %q", disposition)
	}
}

// 不认的类型直接拒（html/svg 一律不收，免得变 XSS 跳板）
func TestUploadRejectsUnsupportedType(t *testing.T) {
	r, _, _, tokens := newAttachmentRouter(t)

	w, body := uploadAs(t, r, tokens[0], "evil.html", []byte("<html><script>alert(1)</script></html>"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("html 应被拒, got %d", w.Code)
	}
	if body["code"] != codeFileTypeRejected {
		t.Fatalf("错误码不对: %v", body["code"])
	}
}

// 路径穿越：库里被塞了 ../ 也读不到上传目录外的东西
func TestAttachmentRejectsPathTraversal(t *testing.T) {
	r, h, _, tokens := newAttachmentRouter(t)

	att := createAttachmentRecord(t, h, 1, "../secret.txt")
	if att == 0 {
		t.Fatal("准备数据失败")
	}
	req := httptest.NewRequest(http.MethodGet, "/api/attachments/"+strconv.Itoa(att), nil)
	req.Header.Set("Authorization", "Bearer "+tokens[0])
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("越界路径要 404, got %d", w.Code)
	}
}

// createAttachmentRecord 直接落一条元信息，绕开上传校验，专门用来测读取侧
func createAttachmentRecord(t *testing.T, h *AttachmentHandler, userID int, relPath string) int {
	t.Helper()
	att := &model.Attachment{
		Filename: "x.txt",
		MimeType: "text/plain",
		Size:     1,
		Path:     relPath,
	}
	if err := h.Repo.CreateAttachment(userID, att); err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	return att.ID
}

// 老版本的 /uploads/2026/11/xxx.png 链接：迁移搬完文件后，
// 本人带着登录态还能打开，别人打开了必须 404，匿名 401。
func TestServeLegacyUpload(t *testing.T) {
	r, h, uploadDir, tokens := newAttachmentRouter(t)

	// 造一个「已迁移」的附件：文件在 1/2026/11/ 下，库里路径带 uid
	dir := filepath.Join(uploadDir, "1", "2026", "11")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "1762000000000000000.png"), pngBytes(t), 0o644); err != nil {
		t.Fatal(err)
	}
	att := &model.Attachment{
		UserID: 1, Filename: "1762000000000000000.png", OriginalName: "shot.png",
		MimeType: "image/png", Size: 100, Path: "1/2026/11/1762000000000000000.png",
	}
	// 用路由里那个 handler 持有的 repo：用户 1 就是 tokens[0] 的账号
	if err := h.Repo.CreateAttachment(1, att); err != nil {
		t.Fatalf("create attachment: %v", err)
	}

	const legacy = "/uploads/2026/11/1762000000000000000.png"

	// 本人：200 + 图片内联
	req := httptest.NewRequest(http.MethodGet, legacy, nil)
	req.Header.Set("Authorization", "Bearer "+tokens[0])
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("本人取老链接 want 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Disposition"); ct != "inline" {
		t.Fatalf("图片应内联，got %q", ct)
	}

	// 别人：404（连存在都不告诉）
	req = httptest.NewRequest(http.MethodGet, legacy, nil)
	req.Header.Set("Authorization", "Bearer "+tokens[1])
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("别人的老链接 want 404, got %d", w.Code)
	}

	// 匿名：401
	req = httptest.NewRequest(http.MethodGet, legacy, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("匿名 want 401, got %d", w.Code)
	}
}
