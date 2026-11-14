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

// GetAll GET /api/tasks?category=&status=&priority=&project_id=&label=&keyword=&today=1&date=&include_archived=1
func (h *TaskHandler) GetAll(c *gin.Context) {
	filter := model.TaskFilter{
		Category:  c.Query("category"),
		Status:    c.Query("status"),
		Priority:  c.Query("priority"),
		Label:     c.Query("label"),
		Keyword:   c.Query("keyword"),
		TodayOnly: c.Query("today") == "1" || c.Query("today") == "true",
		// today=forget：今日页的「已过期 / 未开始」两栏，同一个口径取反
		Today:     c.Query("today_date"),
		Scheduled: c.Query("date"),
		StartDate: c.Query("start_date"),
		EndDate:   c.Query("end_date"),
		// 归档项目下的任务默认不出现，显式带 include_archived 才查
		IncludeArchived: c.Query("include_archived") == "1" || c.Query("include_archived") == "true",
	}
	if idStr := c.Query("project_id"); idStr != "" {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			fail(c, http.StatusBadRequest, codeProjectNotFound, "invalid project_id")
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
		fail(c, http.StatusBadRequest, codeInvalidID, "invalid id")
		return
	}
	task, err := h.Repo.GetTask(userID(c), id)
	if err != nil {
		respondError(c, err)
		return
	}
	if task == nil {
		fail(c, http.StatusNotFound, codeTaskNotFound, "task not found")
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) Create(c *gin.Context) {
	var task model.Task
	if err := c.ShouldBindJSON(&task); err != nil {
		fail(c, http.StatusBadRequest, codeInvalidParams, err.Error())
		return
	}
	if task.Title == "" {
		fail(c, http.StatusBadRequest, codeTaskTitleRequired, "title is required")
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
		fail(c, http.StatusBadRequest, codeInvalidID, "invalid id")
		return
	}
	existing, err := h.Repo.GetTask(userID(c), id)
	if err != nil {
		respondError(c, err)
		return
	}
	if existing == nil {
		fail(c, http.StatusNotFound, codeTaskNotFound, "task not found")
		return
	}

	var task model.Task
	if err := c.ShouldBindJSON(&task); err != nil {
		fail(c, http.StatusBadRequest, codeInvalidParams, err.Error())
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
		fail(c, http.StatusBadRequest, codeInvalidID, "invalid id")
		return
	}
	if err := h.Repo.DeleteTask(userID(c), id); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// Today GET /api/tasks/today 用户时区里的今天：日期 + 时区。
// 前端别拿浏览器的 new Date() 算今天，那跟用户设置的时区可能差一天。
func (h *TaskHandler) Today(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"today":    h.Repo.TodayFor(userID(c)),
		"timezone": h.Repo.TimezoneFor(userID(c)),
	})
}

// SetToday POST /api/tasks/:id/today {is_today: true}
func (h *TaskHandler) SetToday(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		fail(c, http.StatusBadRequest, codeInvalidID, "invalid id")
		return
	}
	var req struct {
		IsToday bool `json:"is_today"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, codeInvalidParams, err.Error())
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
		fail(c, http.StatusInternalServerError, codeReloadFailed, "failed to reload task")
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
		fail(c, http.StatusBadRequest, codeInvalidParams, err.Error())
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
		fail(c, http.StatusBadRequest, codeInvalidID, "invalid id")
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
		fail(c, http.StatusBadRequest, codeInvalidID, "invalid id")
		return
	}
	var comment model.Comment
	if err := c.ShouldBindJSON(&comment); err != nil {
		fail(c, http.StatusBadRequest, codeInvalidParams, err.Error())
		return
	}
	if comment.Content == "" {
		fail(c, http.StatusBadRequest, codeCommentRequired, "content is required")
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
		fail(c, http.StatusBadRequest, codeInvalidID, "invalid id")
		return
	}
	if err := h.Repo.DeleteComment(userID(c), id); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
