package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/imloama/umore/internal/model"
	"github.com/imloama/umore/internal/repository"

	"github.com/gin-gonic/gin"
)

// IdeaHandler 灵感：随手记一条，之后要么转成项目，要么归档
type IdeaHandler struct {
	Handler
}

// GetAll GET /api/ideas?status=inbox|project|archived
func (h *IdeaHandler) GetAll(c *gin.Context) {
	ideas, err := h.Repo.GetIdeas(userID(c), c.Query("status"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, ideas)
}

func (h *IdeaHandler) Create(c *gin.Context) {
	var idea model.Idea
	if err := c.ShouldBindJSON(&idea); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if idea.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "内容不能为空"})
		return
	}
	if err := h.Repo.CreateIdea(userID(c), &idea); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, idea)
}

// Update PUT /api/ideas/:id 改正文或状态
func (h *IdeaHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body struct {
		Content string `json:"content"`
		Status  string `json:"status"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	existing, err := h.Repo.GetIdea(userID(c), id)
	if err != nil {
		respondError(c, err)
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "idea not found"})
		return
	}

	// 没传的字段保持原样：只切状态时不用把正文一起送上来
	idea := model.Idea{ID: id, Content: existing.Content, Status: existing.Status}
	if body.Content != "" || existing.Content == "" {
		idea.Content = body.Content
	}
	if body.Status != "" {
		idea.Status = body.Status
	}
	if err := h.Repo.UpdateIdea(userID(c), &idea); err != nil {
		respondError(c, err)
		return
	}

	updated, err := h.Repo.GetIdea(userID(c), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

// Promote POST /api/ideas/:id/project 转成项目：建一条项目记录，灵感标为已成项目。
// 名称取正文第一行，客户、金额这些留空，进项目页再补。
func (h *IdeaHandler) Promote(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	idea, err := h.Repo.GetIdea(userID(c), id)
	if err != nil {
		respondError(c, err)
		return
	}
	if idea == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "idea not found"})
		return
	}

	// 已经转过的别再转一次，免得同一个想法建出两个项目
	if idea.ProjectID != nil {
		project, err := h.Repo.GetProject(userID(c), *idea.ProjectID)
		if err == nil && project != nil {
			c.JSON(http.StatusOK, gin.H{"idea": idea, "project": project})
			return
		}
	}

	project := model.Project{Name: ideaProjectName(idea.Content), Description: idea.Content}
	if err := h.Repo.CreateProject(userID(c), &project); err != nil {
		respondError(c, err)
		return
	}
	if err := h.Repo.SetIdeaProject(userID(c), id, &project.ID); err != nil {
		respondError(c, err)
		return
	}
	if err := h.Repo.UpdateIdea(userID(c), &model.Idea{
		ID: id, Content: idea.Content, Status: repository.IdeaStatusProject,
	}); err != nil {
		respondError(c, err)
		return
	}

	updated, err := h.Repo.GetIdea(userID(c), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"idea": updated, "project": project})
}

func (h *IdeaHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.Repo.DeleteIdea(userID(c), id); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ideaProjectName 灵感转项目时拿什么当项目名：
// 取第一行（灵感往往一段话，全塞进名字会被截断得看不出重点），太长再切 60 字。
func ideaProjectName(content string) string {
	name := content
	if idx := strings.IndexAny(name, "\r\n"); idx >= 0 {
		name = name[:idx]
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "未命名项目"
	}
	if r := []rune(name); len(r) > 60 {
		name = string(r[:60])
	}
	return name
}
