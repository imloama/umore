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

// respondError 把仓储层错误翻成 HTTP 状态码，别把「记录不存在」也报成 500
func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case errors.Is(err, repository.ErrProjectNameTaken):
		c.JSON(http.StatusConflict, gin.H{"error": "同名项目已存在"})
	case errors.Is(err, repository.ErrLabelNameTaken):
		c.JSON(http.StatusConflict, gin.H{"error": "同名标签已存在"})
	case errors.Is(err, repository.ErrProjectNotFound):
		c.JSON(http.StatusBadRequest, gin.H{"error": "关联的项目不存在"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
