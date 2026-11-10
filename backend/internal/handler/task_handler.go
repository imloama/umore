package handler

import (
	"net/http"
	"strconv"

	"github.com/imloama/umore/internal/model"

	"github.com/gin-gonic/gin"
)

type TaskHandler struct {
	Handler
}

// GetAll GET /api/tasks?category=&status=&priority=&project_id=&label=&keyword=&today=1&date=
func (h *TaskHandler) GetAll(c *gin.Context) {
	filter := model.TaskFilter{
		Category:  c.Query("category"),
		Status:    c.Query("status"),
		Priority:  c.Query("priority"),
		Label:     c.Query("label"),
		Keyword:   c.Query("keyword"),
		TodayOnly: c.Query("today") == "1" || c.Query("today") == "true",
		Scheduled: c.Query("date"),
		StartDate: c.Query("start_date"),
		EndDate:   c.Query("end_date"),
	}
	if idStr := c.Query("project_id"); idStr != "" {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
			return
		}
		filter.ProjectID = &id
	}

	tasks, err := h.Repo.GetTasks(userID(c), filter)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, tasks)
}

func (h *TaskHandler) GetOne(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	task, err := h.Repo.GetTask(userID(c), id)
	if err != nil {
		respondError(c, err)
		return
	}
	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) Create(c *gin.Context) {
	var task model.Task
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if task.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}
	if task.IsToday {
		order, err := h.Repo.NextTodayOrder(userID(c))
		if err != nil {
			respondError(c, err)
			return
		}
		task.TodayOrder = &order
	}

	if err := h.Repo.CreateTask(userID(c), &task); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, task)
}

func (h *TaskHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	existing, err := h.Repo.GetTask(userID(c), id)
	if err != nil {
		respondError(c, err)
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	var task model.Task
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	task.ID = id
	// 未提交标签时保留原有标签
	if task.Labels == nil {
		task.Labels = existing.Labels
	}

	if err := h.Repo.UpdateTask(userID(c), &task); err != nil {
		respondError(c, err)
		return
	}
	updated, err := h.Repo.GetTask(userID(c), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *TaskHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.Repo.DeleteTask(userID(c), id); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// SetToday POST /api/tasks/:id/today {is_today: true}
func (h *TaskHandler) SetToday(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		IsToday bool `json:"is_today"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	order := 0
	if req.IsToday {
		order, err = h.Repo.NextTodayOrder(userID(c))
		if err != nil {
			respondError(c, err)
			return
		}
	}
	if err := h.Repo.SetTaskToday(userID(c), id, req.IsToday, order); err != nil {
		respondError(c, err)
		return
	}
	task, err := h.Repo.GetTask(userID(c), id)
	if err != nil || task == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reload task"})
		return
	}
	c.JSON(http.StatusOK, task)
}

// ReorderToday PUT /api/tasks/reorder [{id, today_order}]
func (h *TaskHandler) ReorderToday(c *gin.Context) {
	var req []struct {
		ID         int `json:"id"`
		TodayOrder int `json:"today_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	orders := map[int]int{}
	for _, item := range req {
		orders[item.ID] = item.TodayOrder
	}
	if err := h.Repo.ReorderToday(userID(c), orders); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "reordered"})
}

// Stats GET /api/tasks/stats
func (h *TaskHandler) Stats(c *gin.Context) {
	stats, err := h.Repo.GetTaskStats(userID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetComments GET /api/tasks/:id/comments
func (h *TaskHandler) GetComments(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	comments, err := h.Repo.GetComments(userID(c), "task", id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, comments)
}

// AddComment POST /api/tasks/:id/comments
func (h *TaskHandler) AddComment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var comment model.Comment
	if err := c.ShouldBindJSON(&comment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if comment.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}
	comment.TargetType = "task"
	comment.TargetID = id
	if err := h.Repo.CreateComment(userID(c), &comment); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, comment)
}

// DeleteComment DELETE /api/tasks/comments/:id
func (h *TaskHandler) DeleteComment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.Repo.DeleteComment(userID(c), id); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
