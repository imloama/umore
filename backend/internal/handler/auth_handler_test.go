package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/imloama/umore/internal/auth"
	"github.com/imloama/umore/internal/cache"
	"github.com/imloama/umore/internal/config"
	"github.com/imloama/umore/internal/repository"
	"github.com/imloama/umore/internal/verification"

	"github.com/gin-gonic/gin"
)

// captureSender 假的发信器：把验证码留在内存里，测试就能拿到用户「收到的」码
type captureSender struct {
	mu   sync.Mutex
	code map[string]string
	fail bool
}

func newCaptureSender() *captureSender {
	return &captureSender{code: map[string]string{}}
}

func (s *captureSender) SendCode(_ context.Context, to, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return context.DeadlineExceeded
	}
	s.code[to] = code
	return nil
}

func (s *captureSender) lastCode(to string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.code[to]
}

func testResendConfig() config.Resend {
	return config.Resend{
		APIKey:                "re_test_key",
		From:                  "umore <no-reply@example.com>",
		CodeLength:            6,
		CodeTTLMinutes:        10,
		ResendIntervalSeconds: 60,
		MaxAttempts:           5,
	}
}

func newAuthRouter(t *testing.T) (*gin.Engine, *AuthHandler, *captureSender) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	repo, err := repository.NewRepository(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { repo.Close() })

	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	sender := newCaptureSender()
	store := cache.NewMemory()
	t.Cleanup(func() { store.Close() })

	h := &AuthHandler{
		Handler:  Handler{Repo: repo},
		Verify:   verification.New(repo, store, testResendConfig(), sender),
		Sessions: auth.NewManager(store, time.Hour),
	}
	r := gin.New()
	r.POST("/api/auth/send-code", h.SendCode)
	r.POST("/api/auth/register", h.Register)
	r.POST("/api/auth/login", h.Login)
	r.GET("/api/auth/check-setup", h.CheckSetup)
	r.POST("/api/auth/logout", h.Logout)
	r.GET("/api/auth/me", RequireAuth(h.Sessions), h.Me)
	r.PUT("/api/auth/profile", RequireAuth(h.Sessions), h.UpdateProfile)
	r.POST("/api/auth/password", RequireAuth(h.Sessions), h.ChangePassword)
	return r, h, sender
}

func postJSON(r *gin.Engine, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w, out
}

// 发码 → 带码注册 → 登录，注册成功后邮箱应为已验证
func TestSendCodeThenRegister(t *testing.T) {
	r, _, sender := newAuthRouter(t)

	w, body := postJSON(r, "/api/auth/send-code", gin.H{"email": " Max@Example.com "})
	if w.Code != http.StatusOK {
		t.Fatalf("send-code want 200, got %d: %s", w.Code, w.Body.String())
	}
	if body["resend_after"].(float64) != 60 || body["code_length"].(float64) != 6 {
		t.Fatalf("返回的发码参数不对: %v", body)
	}
	code := sender.lastCode("max@example.com")
	if len(code) != 6 {
		t.Fatalf("应该给归一化后的邮箱发 6 位码, got %q", code)
	}

	w, body = postJSON(r, "/api/auth/register", gin.H{"email": "max@example.com", "password": "12345678", "code": code})
	if w.Code != http.StatusCreated {
		t.Fatalf("register want 201, got %d: %s", w.Code, w.Body.String())
	}
	if _, ok := body["key"]; ok {
		t.Fatal("业务字段不再做客户端加密，不该再返回 key")
	}
	if body["token"] == nil || body["token"] == "" {
		t.Fatal("注册后应直接签发 token")
	}
	user, _ := body["user"].(map[string]any)
	if user["email"] != "max@example.com" {
		t.Fatalf("邮箱应存小写并去空格, got %v", user["email"])
	}
	if user["email_verified"] != true {
		t.Fatalf("验证码通过后邮箱应标记为已验证, got %v", user["email_verified"])
	}
	if nickname, _ := user["nickname"].(string); nickname == "" || !strings.HasPrefix(nickname, "MORE") {
		t.Fatalf("注册应自动生成 MORE+ID 昵称, got %v", user["nickname"])
	}
	if _, ok := user["password_hash"]; ok {
		t.Fatal("密码哈希不应该返回给前端")
	}

	w, body = postJSON(r, "/api/auth/login", gin.H{"email": "max@example.com", "password": "12345678"})
	if w.Code != http.StatusOK {
		t.Fatalf("login want 200, got %d: %s", w.Code, w.Body.String())
	}
	if body["user"] == nil {
		t.Fatalf("登录应返回账号信息: %v", body)
	}
}

// 没有验证码 / 验证码错，都不许建号
func TestRegisterRequiresValidCode(t *testing.T) {
	r, h, _ := newAuthRouter(t)

	// 连码都没发过
	if w, _ := postJSON(r, "/api/auth/register", gin.H{"email": "max@example.com", "password": "12345678", "code": "123456"}); w.Code != http.StatusBadRequest {
		t.Fatalf("没发过码 want 400, got %d", w.Code)
	}
	if user, _ := h.Repo.GetUserByEmail("max@example.com"); user != nil {
		t.Fatal("码不对不该建号")
	}

	// 缺 code 字段
	if w, _ := postJSON(r, "/api/auth/register", gin.H{"email": "max@example.com", "password": "12345678"}); w.Code != http.StatusBadRequest {
		t.Fatalf("缺验证码 want 400, got %d", w.Code)
	}

	// 发了码但填错
	postJSON(r, "/api/auth/send-code", gin.H{"email": "max@example.com"})
	w, body := postJSON(r, "/api/auth/register", gin.H{"email": "max@example.com", "password": "12345678", "code": "000000"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("验证码错 want 400, got %d: %s", w.Code, w.Body.String())
	}
	if body["code"] != "auth_code_wrong" {
		t.Fatalf("错误码不对: %v", body["code"])
	}
	if user, _ := h.Repo.GetUserByEmail("max@example.com"); user != nil {
		t.Fatal("验证码错不该建号")
	}
}

// 验证码用过一次就作废，重放不能二次注册
func TestRegisterCodeSingleUse(t *testing.T) {
	r, h, sender := newAuthRouter(t)

	postJSON(r, "/api/auth/send-code", gin.H{"email": "max@example.com"})
	code := sender.lastCode("max@example.com")

	if w, _ := postJSON(r, "/api/auth/register", gin.H{"email": "max@example.com", "password": "12345678", "code": code}); w.Code != http.StatusCreated {
		t.Fatalf("first register want 201, got %d", w.Code)
	}
	// 换个邮箱也没用：码是绑邮箱的
	if w, _ := postJSON(r, "/api/auth/register", gin.H{"email": "other@example.com", "password": "12345678", "code": code}); w.Code != http.StatusBadRequest {
		t.Fatalf("码不能跨邮箱用 want 400, got %d", w.Code)
	}
	if user, _ := h.Repo.GetUserByEmail("other@example.com"); user != nil {
		t.Fatal("码不能跨邮箱建号")
	}
}

// 同一邮箱连续发码要被拦截
func TestSendCodeRateLimited(t *testing.T) {
	r, _, _ := newAuthRouter(t)

	if w, _ := postJSON(r, "/api/auth/send-code", gin.H{"email": "max@example.com"}); w.Code != http.StatusOK {
		t.Fatalf("first send want 200, got %d", w.Code)
	}
	if w, _ := postJSON(r, "/api/auth/send-code", gin.H{"email": "max@example.com"}); w.Code != http.StatusTooManyRequests {
		t.Fatalf("连续发码 want 429, got %d", w.Code)
	}
}

// 已注册的邮箱不再发码
func TestSendCodeForExistingEmail(t *testing.T) {
	r, _, sender := newAuthRouter(t)

	postJSON(r, "/api/auth/send-code", gin.H{"email": "max@example.com"})
	code := sender.lastCode("max@example.com")
	postJSON(r, "/api/auth/register", gin.H{"email": "max@example.com", "password": "12345678", "code": code})

	if w, body := postJSON(r, "/api/auth/send-code", gin.H{"email": "MAX@example.com"}); w.Code != http.StatusConflict {
		t.Fatalf("已注册邮箱 want 409, got %d: %v", w.Code, body["error"])
	}
}

// 入参校验：邮箱形状、密码长度
func TestRegisterValidation(t *testing.T) {
	r, _, _ := newAuthRouter(t)

	cases := []struct {
		name string
		body gin.H
	}{
		{"邮箱没 @", gin.H{"email": "maxexample.com", "password": "12345678", "code": "123456"}},
		{"邮箱域名没点", gin.H{"email": "max@example", "password": "12345678", "code": "123456"}},
		{"密码太短", gin.H{"email": "max@example.com", "password": "1234567", "code": "123456"}},
	}
	for _, c := range cases {
		if w, _ := postJSON(r, "/api/auth/register", c.body); w.Code != http.StatusBadRequest {
			t.Fatalf("%s want 400, got %d: %s", c.name, w.Code, w.Body.String())
		}
	}
}

// 发码接口的邮箱形状校验
func TestSendCodeValidation(t *testing.T) {
	r, _, _ := newAuthRouter(t)

	if w, _ := postJSON(r, "/api/auth/send-code", gin.H{"email": "maxexample.com"}); w.Code != http.StatusBadRequest {
		t.Fatalf("邮箱格式不对 want 400, got %d", w.Code)
	}
}

// 密码错与邮箱不存在回同一句话，别把注册情况漏出去
func TestLoginWrongCredentials(t *testing.T) {
	r, _, sender := newAuthRouter(t)

	postJSON(r, "/api/auth/send-code", gin.H{"email": "max@example.com"})
	code := sender.lastCode("max@example.com")
	if w, _ := postJSON(r, "/api/auth/register", gin.H{"email": "max@example.com", "password": "12345678", "code": code}); w.Code != http.StatusCreated {
		t.Fatalf("register want 201, got %d", w.Code)
	}

	wrongPassword, body := postJSON(r, "/api/auth/login", gin.H{"email": "max@example.com", "password": "bad-password"})
	unknownEmail, body2 := postJSON(r, "/api/auth/login", gin.H{"email": "nobody@example.com", "password": "12345678"})

	if wrongPassword.Code != http.StatusUnauthorized || unknownEmail.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d / %d", wrongPassword.Code, unknownEmail.Code)
	}
	// 文案交给前端按错误码渲染，所以这两处必须回同一个码，别把注册情况漏出去
	if body["code"] != body2["code"] {
		t.Fatalf("两种失败应给出同样的错误码: %v vs %v", body["code"], body2["code"])
	}
}

// 发信失败时不留下能用的码，用户重发即可
func TestSendCodeMailFailure(t *testing.T) {
	r, h, sender := newAuthRouter(t)
	sender.fail = true

	w, _ := postJSON(r, "/api/auth/send-code", gin.H{"email": "max@example.com"})
	if w.Code != http.StatusBadGateway {
		t.Fatalf("发信失败 want 502, got %d", w.Code)
	}
	// 码没发出去，重发不该被冷却挡住
	w, _ = postJSON(r, "/api/auth/send-code", gin.H{"email": "max@example.com"})
	if w.Code != http.StatusBadGateway {
		t.Fatalf("发信失败后应能立刻重发，got %d: %s", w.Code, w.Body.String())
	}
	_ = h
}

// 没配发信参数时不发码，给出「邮件服务未配置」
func TestSendCodeNotConfigured(t *testing.T) {
	r, h, _ := newAuthRouter(t)
	h.Verify = verification.New(h.Repo, cache.NewMemory(), config.Resend{}, nil)

	w, body := postJSON(r, "/api/auth/send-code", gin.H{"email": "max@example.com"})
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("未配置 want 503, got %d", w.Code)
	}
	if body["code"] != "auth_mail_not_configured" {
		t.Fatalf("错误码不对: %v", body["code"])
	}
}

// check-setup 用来决定首页停在注册还是登录
func TestCheckSetup(t *testing.T) {
	r, _, sender := newAuthRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/check-setup", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"setup_completed":false`)) {
		t.Fatalf("空库应未初始化: %s", w.Body.String())
	}

	postJSON(r, "/api/auth/send-code", gin.H{"email": "max@example.com"})
	code := sender.lastCode("max@example.com")
	postJSON(r, "/api/auth/register", gin.H{"email": "max@example.com", "password": "12345678", "code": code})

	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if !bytes.Contains(w.Body.Bytes(), []byte(`"setup_completed":true`)) {
		t.Fatalf("注册后应已初始化: %s", w.Body.String())
	}
}

// 注册一个账号并把 token 拿回来，个人资料相关用例共用
func registerAndLogin(t *testing.T, r *gin.Engine, sender *captureSender, email, password string) string {
	t.Helper()
	postJSON(r, "/api/auth/send-code", gin.H{"email": email})
	code := sender.lastCode(strings.ToLower(email))
	w, body := postJSON(r, "/api/auth/register", gin.H{"email": email, "password": password, "code": code})
	if w.Code != http.StatusCreated {
		t.Fatalf("register want 201, got %d: %s", w.Code, w.Body.String())
	}
	token, _ := body["token"].(string)
	if token == "" {
		t.Fatal("注册后应返回 token")
	}
	return token
}

// 昵称能改，空昵称不清空；没登录改不了
func TestUpdateProfile(t *testing.T) {
	r, h, sender := newAuthRouter(t)
	token := registerAndLogin(t, r, sender, "max@example.com", "12345678")

	payload, _ := json.Marshal(gin.H{"nickname": "老马"})
	req := httptest.NewRequest(http.MethodPut, "/api/auth/profile", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("update profile want 200, got %d: %s", w.Code, w.Body.String())
	}
	user, _ := h.Repo.GetUserByEmail("max@example.com")
	if user.Nickname != "老马" {
		t.Fatalf("昵称没改上: %q", user.Nickname)
	}

	// 传空昵称当作「不改」，别把名字清成空白
	payload, _ = json.Marshal(gin.H{"nickname": "  "})
	req = httptest.NewRequest(http.MethodPut, "/api/auth/profile", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if user, _ := h.Repo.GetUserByEmail("max@example.com"); user.Nickname != "老马" {
		t.Fatalf("空昵称不该覆盖原值: %q", user.Nickname)
	}

	// 没 token 一律 401
	req = httptest.NewRequest(http.MethodPut, "/api/auth/profile", bytes.NewReader([]byte(`{"nickname":"x"}`)))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("未登录改资料 want 401, got %d", w.Code)
	}
}

// 改密码：旧密码错了不动库，改完新密码能登录、老密码不行
func TestChangePassword(t *testing.T) {
	r, _, sender := newAuthRouter(t)
	token := registerAndLogin(t, r, sender, "max@example.com", "12345678")

	call := func(oldPwd, newPwd string) *httptest.ResponseRecorder {
		payload, _ := json.Marshal(gin.H{"old_password": oldPwd, "new_password": newPwd})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/password", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	if w := call("wrong-old", "newpassword"); w.Code != http.StatusBadRequest {
		t.Fatalf("旧密码错 want 400, got %d: %s", w.Code, w.Body.String())
	}
	if w := call("12345678", "short"); w.Code != http.StatusBadRequest {
		t.Fatalf("新密码太短 want 400, got %d", w.Code)
	}
	if w := call("12345678", "12345678"); w.Code != http.StatusBadRequest {
		t.Fatalf("新密码和旧的一样 want 400, got %d", w.Code)
	}
	if w := call("12345678", "newpassword"); w.Code != http.StatusOK {
		t.Fatalf("改密码 want 200, got %d: %s", w.Code, w.Body.String())
	}

	if w, _ := postJSON(r, "/api/auth/login", gin.H{"email": "max@example.com", "password": "12345678"}); w.Code != http.StatusUnauthorized {
		t.Fatalf("旧密码应登不进去, got %d", w.Code)
	}
	if w, _ := postJSON(r, "/api/auth/login", gin.H{"email": "max@example.com", "password": "newpassword"}); w.Code != http.StatusOK {
		t.Fatalf("新密码应能登录, got %d: %s", w.Code, w.Body.String())
	}
}
