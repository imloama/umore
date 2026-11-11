package handler

import (
	"net/http"
	"strconv"

	"github.com/imloama/umore/internal/model"

	"github.com/gin-gonic/gin"
)

type ProjectHandler struct {
	Handler
}

// projectPayload 项目表单。金额变了可以带一句变更说明，落库时记一条金额变更。
type projectPayload struct {
	model.Project
	AmountChangeReason string `json:"amount_change_reason"`
}

func (h *ProjectHandler) GetAll(c *gin.Context) {
	projects, err := h.Repo.GetProjects(userID(c))
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, projects)
}

func (h *ProjectHandler) GetOne(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	project, err := h.Repo.GetProject(userID(c), id)
	if err != nil {
		respondError(c, err)
		return
	}
	if project == nil {
		fail(c, http.StatusNotFound, codeProjectNotFound, "project not found")
		return
	}

	c.JSON(http.StatusOK, project)
}

func (h *ProjectHandler) Create(c *gin.Context) {
	var payload projectPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		fail(c, http.StatusBadRequest, codeInvalidParams, err.Error())
		return
	}
	project := payload.Project

	if err := h.Repo.CreateProject(userID(c), &project); err != nil {
		respondError(c, err)
		return
	}

	// 回读一次，把汇总字段与时间戳一起给前端
	if created, err := h.Repo.GetProject(userID(c), project.ID); err == nil && created != nil {
		project = *created
	}
	c.JSON(http.StatusCreated, project)
}

func (h *ProjectHandler) Update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	existing, err := h.Repo.GetProject(userID(c), id)
	if err != nil {
		respondError(c, err)
		return
	}
	if existing == nil {
		fail(c, http.StatusNotFound, codeProjectNotFound, "project not found")
		return
	}

	var payload projectPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		fail(c, http.StatusBadRequest, codeInvalidParams, err.Error())
		return
	}
	project := payload.Project
	project.ID = id

	if err := h.Repo.UpdateProject(userID(c), &project, payload.AmountChangeReason); err != nil {
		respondError(c, err)
		return
	}

	updated, err := h.Repo.GetProject(userID(c), id)
	if err != nil || updated == nil {
		fail(c, http.StatusInternalServerError, codeReloadFailed, "failed to reload project")
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *ProjectHandler) Delete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.Repo.DeleteProject(userID(c), id); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// GetAmountChanges GET /api/projects/:id/amount-changes
func (h *ProjectHandler) GetAmountChanges(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		fail(c, http.StatusBadRequest, codeInvalidID, "invalid id")
		return
	}
	changes, err := h.Repo.GetProjectAmountChanges(userID(c), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, changes)
}

// AddAmountChange POST /api/projects/:id/amount-changes {new_amount, reason}
func (h *ProjectHandler) AddAmountChange(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		fail(c, http.StatusBadRequest, codeInvalidID, "invalid id")
		return
	}
	var req struct {
		NewAmount model.Amount `json:"new_amount"`
		Reason    string       `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, codeInvalidParams, err.Error())
		return
	}
	if req.NewAmount.IsNegative() {
		fail(c, http.StatusBadRequest, codeAmountInvalid, "new_amount must be >= 0")
		return
	}

	change, err := h.Repo.AddProjectAmountChange(userID(c), id, req.NewAmount, req.Reason)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, change)
}

// DeleteAmountChange DELETE /api/projects/amount-changes/:id
// 只删记录，不回滚项目当前金额——历史是历史，金额以最新一条为准。
func (h *ProjectHandler) DeleteAmountChange(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		fail(c, http.StatusBadRequest, codeInvalidID, "invalid id")
		return
	}
	if err := h.Repo.DeleteProjectAmountChange(userID(c), id); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// GetComments GET /api/projects/:id/comments
// 项目和任务共用 comments 表，靠 target_type 区分。
func (h *ProjectHandler) GetComments(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		fail(c, http.StatusBadRequest, codeInvalidID, "invalid id")
		return
	}
	comments, err := h.Repo.GetComments(userID(c), "project", id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, comments)
}

// AddComment POST /api/projects/:id/comments {content}
func (h *ProjectHandler) AddComment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		fail(c, http.StatusBadRequest, codeInvalidID, "invalid id")
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, codeInvalidParams, err.Error())
		return
	}
	if req.Content == "" {
		fail(c, http.StatusBadRequest, codeCommentRequired, "content is required")
		return
	}

	comment := model.Comment{TargetType: "project", TargetID: id, Content: req.Content}
	if err := h.Repo.CreateComment(userID(c), &comment); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, comment)
}
