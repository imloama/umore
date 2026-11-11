package handler

import (
	"errors"
	"net/http"

	"github.com/imloama/umore/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Handler 各业务 handler 的公共部分
type Handler struct {
	Repo *repository.Repository
}

func NewHandler(repo *repository.Repository) *Handler {
	return &Handler{Repo: repo}
}

// userID 当前登录用户，由 RequireAuth 放进上下文。
// 每个仓储调用都必须带上它，数据隔离就靠这条线，别从请求体里取 user_id。
func userID(c *gin.Context) int {
	return c.GetInt(CtxUserID)
}

// 错误响应统一带两个字段：
//   - error: 兜底文案（英文），前端正常只用来排查；界面文案由前端按 code 查表渲染
//   - code:  稳定的错误码，多语言在前端做，后端不维护翻译
//
// 新增错误时在 code_* 里挑一个已有含义的，别把中文塞进 error 里——那是前端的事。
func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		fail(c, http.StatusNotFound, codeNotFound, "not found")
	case errors.Is(err, repository.ErrProjectNameTaken):
		fail(c, http.StatusConflict, codeProjectNameTaken, "project name already exists")
	case errors.Is(err, repository.ErrLabelNameTaken):
		fail(c, http.StatusConflict, codeLabelNameTaken, "label name already exists")
	case errors.Is(err, repository.ErrProjectNotFound):
		fail(c, http.StatusBadRequest, codeProjectNotFound, "related project not found")
	default:
		fail(c, http.StatusInternalServerError, codeInternal, err.Error())
	}
}

// fail 统一的错误出口：状态码 + 错误码 + 兜底文案
func fail(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": message, "code": code})
}
