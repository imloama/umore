package handler

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/imloama/umore/internal/auth"
	"github.com/imloama/umore/internal/crypto"
	"github.com/imloama/umore/internal/repository"
	"github.com/imloama/umore/internal/verification"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	Handler
	// Verify 发码与校验，nil 时按「没配邮件服务」处理
	Verify *verification.Service
	// Sessions 登录 token 的签发与撤销，nil 时登录接口不可用
	Sessions *auth.Manager
	// UploadDir 头像落盘根目录，与附件共用 data/uploads
	UploadDir string
}

type RegisterRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
	// Code 邮箱验证码，必须是发到该邮箱、还没用掉的那个
	Code string `json:"code" binding:"required"`
}

// SendCodeRequest 发验证码的入参
type SendCodeRequest struct {
	Email string `json:"email" binding:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// SendCode 注册前先把验证码发到邮箱，没通过验证建不了号
func (h *AuthHandler) SendCode(c *gin.Context) {
	var req SendCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, codeEmailRequired, "email is required")
		return
	}

	email := normalizeEmail(req.Email)
	if !isEmail(email) {
		fail(c, http.StatusBadRequest, codeEmailInvalid, "invalid email")
		return
	}
	if h.Verify == nil || !h.Verify.Available() {
		fail(c, http.StatusServiceUnavailable, codeMailNotConfigured, "mail service not configured")
		return
	}

	err := h.Verify.SendRegisterCode(c.Request.Context(), email)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{
			"code":         "ok",
			"message":      "verification code sent",
			"resend_after": int(h.Verify.Interval.Seconds()),
			"expires_in":   int(h.Verify.TTL.Seconds()),
			"code_length":  h.Verify.Digits,
			"delivery":     h.Verify.DeliveryMode(),
		})
	case errors.Is(err, repository.ErrEmailTaken):
		fail(c, http.StatusConflict, codeEmailTaken, "email already registered")
	case errors.Is(err, verification.ErrTooFrequent):
		fail(c, http.StatusTooManyRequests, codeCodeTooFrequent, err.Error())
	case errors.Is(err, verification.ErrNotConfigured):
		fail(c, http.StatusServiceUnavailable, codeMailNotConfigured, "mail service not configured")
	default:
		// 发信失败的具体原因（key 无效、域名未验证等）只写日志，别糊给用户
		log.Printf("send-code %s failed: %v", email, err)
		fail(c, http.StatusBadGateway, codeMailSendFailed, "failed to send verification code")
	}
}

// Register 邮箱 + 密码 + 验证码注册。
// 验证码不对一律不建号；通过后落库即 email_verified = 1。
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, codeMissingField, "email, password and code are required")
		return
	}

	email := normalizeEmail(req.Email)
	if !isEmail(email) {
		fail(c, http.StatusBadRequest, codeEmailInvalid, "invalid email")
		return
	}

	existing, err := h.Repo.GetUserByEmail(email)
	if err != nil {
		fail(c, http.StatusInternalServerError, codeInternal, "database error")
		return
	}
	if existing != nil {
		fail(c, http.StatusConflict, codeEmailTaken, "email already registered")
		return
	}

	// 先校验证书，再动密码哈希：码不对不该在库里留下任何东西
	if h.Verify == nil {
		fail(c, http.StatusServiceUnavailable, codeMailNotConfigured, "mail service not configured")
		return
	}
	if err := h.Verify.VerifyRegisterCode(c.Request.Context(), email, req.Code); err != nil {
		fail(c, http.StatusBadRequest, codeCodeWrong, err.Error())
		return
	}

	salt, err := crypto.GenerateSalt()
	if err != nil {
		fail(c, http.StatusInternalServerError, codeInternal, "failed to generate salt")
		return
	}
	user, err := h.Repo.CreateUser(email, crypto.HashPassword(req.Password, salt), salt, true)
	if err != nil {
		// 并发重复提交时唯一索引会挡下第二次，这里翻译成「已注册」而不是 500
		if existing, e := h.Repo.GetUserByEmail(email); e == nil && existing != nil {
			fail(c, http.StatusConflict, codeEmailTaken, "email already registered")
			return
		}
		fail(c, http.StatusInternalServerError, codeInternal, "failed to create user")
		return
	}

	if h.Sessions == nil {
		fail(c, http.StatusServiceUnavailable, codeInternal, "session service unavailable")
		return
	}
	// 注册完直接签发 token，前端不必再登一次
	token, expiresAt, err := h.Sessions.Issue(c.Request.Context(), user.ID, user.Email)
	if err != nil {
		fail(c, http.StatusInternalServerError, codeInternal, "failed to issue session")
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":       "ok",
		"message":    "register successful",
		"user":       user,
		"token":      token,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
}

// Login 邮箱 + 密码登录，校验通过后返回加密密钥
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, codeMissingField, "email and password are required")
		return
	}

	email := normalizeEmail(req.Email)
	user, err := h.Repo.GetUserByEmail(email)
	if err != nil {
		fail(c, http.StatusInternalServerError, codeInternal, "database error")
		return
	}

	if user == nil {
		// 邮箱不存在与密码错误返回同一句话，避免把注册情况漏出去
		fail(c, http.StatusUnauthorized, codePasswordWrong, "email or password is incorrect")
		return
	}
	if crypto.HashPassword(req.Password, user.PasswordSalt) != user.PasswordHash {
		fail(c, http.StatusUnauthorized, codePasswordWrong, "email or password is incorrect")
		return
	}

	if h.Sessions == nil {
		fail(c, http.StatusServiceUnavailable, codeInternal, "session service unavailable")
		return
	}
	token, expiresAt, err := h.Sessions.Issue(c.Request.Context(), user.ID, user.Email)
	if err != nil {
		fail(c, http.StatusInternalServerError, codeInternal, "failed to issue session")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":       "ok",
		"message":    "login successful",
		"user":       user,
		"token":      token,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
}

// CheckSetup 前端用它决定默认停在注册页还是登录页
func (h *AuthHandler) CheckSetup(c *gin.Context) {
	count, err := h.Repo.CountUsers()
	if err != nil {
		fail(c, http.StatusInternalServerError, codeInternal, "database error")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":            "ok",
		"setup_completed": count > 0,
		"has_users":       count > 0,
	})
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// isEmail 只做基本形状校验，够挡住误输入即可
func isEmail(email string) bool {
	at := strings.IndexByte(email, '@')
	if at <= 0 || at == len(email)-1 {
		return false
	}
	domain := email[at+1:]
	if !strings.Contains(domain, ".") || strings.ContainsAny(email, " \t\r\n") {
		return false
	}
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return false
	}
	return len(email) <= 254
}

// Logout POST /api/auth/logout 退出登录：把缓存里那条会话删掉，token 立刻作废。
// 前端删掉本地 token 只是「不想再看」，服务端这一步才是真的登出。
func (h *AuthHandler) Logout(c *gin.Context) {
	if h.Sessions == nil {
		c.JSON(http.StatusOK, gin.H{"message": "logged out"})
		return
	}
	token := bearerToken(c.GetHeader("Authorization"))
	if err := h.Sessions.Revoke(c.Request.Context(), token); err != nil {
		fail(c, http.StatusInternalServerError, codeInternal, "failed to revoke session")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// Me GET /api/auth/me 用 token 换回当前账号，前端刷新页面时用它确认登录态还在
func (h *AuthHandler) Me(c *gin.Context) {
	user, err := h.Repo.GetUser(userID(c))
	if err != nil {
		fail(c, http.StatusInternalServerError, codeInternal, "database error")
		return
	}
	if user == nil {
		fail(c, http.StatusUnauthorized, codeUnauthorized, "invalid or expired token")
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}

// UpdateProfileRequest 个人资料：昵称与头像地址（头像走上传接口拿 URL，这里只存地址）
type UpdateProfileRequest struct {
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

// UpdateProfile PUT /api/auth/profile 改昵称 / 头像。
// 两个字段都可以单独改：没带的那个保持原样。
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, codeInvalidParams, "invalid request body")
		return
	}

	user, err := h.Repo.GetUser(userID(c))
	if err != nil {
		fail(c, http.StatusInternalServerError, codeInternal, "database error")
		return
	}
	if user == nil {
		fail(c, http.StatusUnauthorized, codeUnauthorized, "invalid or expired token")
		return
	}

	// 没传的字段保持原样：昵称不允许清空，头像要清就调上传接口覆盖
	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" {
		nickname = user.Nickname
	} else if n := utf8.RuneCountInString(nickname); n > 32 {
		fail(c, http.StatusBadRequest, codeNicknameTooLong, "nickname is too long (max 32)")
		return
	}
	avatar := req.Avatar
	if avatar == "" {
		avatar = user.Avatar
	}

	if err := h.Repo.UpdateProfile(user.ID, nickname, avatar); err != nil {
		respondError(c, err)
		return
	}

	user.Nickname = nickname
	user.Avatar = avatar
	c.JSON(http.StatusOK, gin.H{"user": user})
}

// ChangePasswordRequest 改密码
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// ChangePassword POST /api/auth/password 先验旧密码，再换哈希与盐。
// 换完密码当前 token 仍然有效：token 是会话号，本来就跟密码无关。
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, codePasswordTooShort, "old password required, new password must be at least 8 characters")
		return
	}

	user, err := h.Repo.GetUser(userID(c))
	if err != nil {
		fail(c, http.StatusInternalServerError, codeInternal, "database error")
		return
	}
	if user == nil {
		fail(c, http.StatusUnauthorized, codeUnauthorized, "invalid or expired token")
		return
	}
	if crypto.HashPassword(req.OldPassword, user.PasswordSalt) != user.PasswordHash {
		fail(c, http.StatusBadRequest, codePasswordWrong, "old password is incorrect")
		return
	}
	if req.OldPassword == req.NewPassword {
		fail(c, http.StatusBadRequest, codePasswordSame, "new password is the same as the old one")
		return
	}

	salt, err := crypto.GenerateSalt()
	if err != nil {
		fail(c, http.StatusInternalServerError, codeInternal, "failed to generate salt")
		return
	}
	if err := h.Repo.UpdatePassword(user.ID, crypto.HashPassword(req.NewPassword, salt), salt); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "password updated", "code": "ok"})
}

// UploadAvatar POST /api/auth/avatar (multipart, 字段名 file)
// 头像跟着用户走：落盘后在 users.avatar 里记 URL，旧的图不删（当附件留着也无妨）。
func (h *AuthHandler) UploadAvatar(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, codeFileRequired, "file is required")
		return
	}
	if file.Size > maxUploadBytes {
		fail(c, http.StatusBadRequest, codeFileTooLarge, "file too large (max 10MB)")
		return
	}

	src, err := file.Open()
	if err != nil {
		fail(c, http.StatusInternalServerError, codeFileReadFailed, "failed to read file")
		return
	}
	defer src.Close()

	// 只认文件头，不信任客户端给的扩展名（顺手摘掉 "; charset=..."）
	head := make([]byte, 512)
	n, _ := src.Read(head)
	mime := baseMimeType(http.DetectContentType(head[:n]))
	ext, ok := allowedImageTypes[mime]
	if !ok {
		fail(c, http.StatusBadRequest, codeFileTypeRejected, "only png/jpeg/gif/webp images are allowed")
		return
	}
	if _, err := src.Seek(0, 0); err != nil {
		fail(c, http.StatusInternalServerError, codeFileReadFailed, "failed to read file")
		return
	}

	// 头像按用户分目录：uploads/avatars/<user_id>/xxx.png。
	// 这里不覆盖旧文件，换了头像旧图留在原地（没有引用它的地方了，留着也不碍事）。
	uid := userID(c)
	sub := filepath.Join("avatars", strconv.Itoa(uid))
	dir := filepath.Join(h.UploadDir, sub)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fail(c, http.StatusInternalServerError, codeFileSaveFailed, "failed to create avatar dir")
		return
	}
	name := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	if err := c.SaveUploadedFile(file, filepath.Join(dir, name)); err != nil {
		fail(c, http.StatusInternalServerError, codeFileSaveFailed, "failed to save file")
		return
	}

	// 头像仍是公开静态资源（<img> 带不了鉴权头），但它不是「附件」，走 /uploads/avatars
	urlPath := "/uploads/avatars/" + strconv.Itoa(uid) + "/" + name
	if err := h.Repo.SetAvatar(userID(c), urlPath); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"avatar": urlPath})
}
