package handler

import (
	"net/http"
	"strings"

	"github.com/imloama/umore/internal/auth"

	"github.com/gin-gonic/gin"
)

// 上下文里的键名，别处取会话信息统一用这两个常量
const (
	CtxUserID = "user_id"
	CtxEmail  = "email"
	CtxToken  = "token"
)

// RequireAuth 校验 Authorization: Bearer <token>，通过后把身份放进上下文。
// 登录态存在缓存（Redis）里，退出登录就是删掉那条会话，token 立刻失效。
func RequireAuth(sessions *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c.GetHeader("Authorization"))
		if token == "" {
			fail(c, http.StatusUnauthorized, codeUnauthorized, "invalid or expired token")
			return
		}
		sess, err := sessions.Lookup(c.Request.Context(), token)
		if err != nil {
			fail(c, http.StatusUnauthorized, codeUnauthorized, "invalid or expired token")
			return
		}
		c.Set(CtxUserID, sess.UserID)
		c.Set(CtxEmail, sess.Email)
		c.Set(CtxToken, token)
		c.Next()
	}
}

// bearerToken 从 Authorization 头里取 token，大小写不敏感
func bearerToken(header string) string {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}
