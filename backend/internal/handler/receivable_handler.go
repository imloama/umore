package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/imloama/umore/internal/model"

	"github.com/gin-gonic/gin"
)

type ReceivableHandler struct {
	Handler
}

// receivablePayload 收款计划表单：项目可空（私人垫款也能挂一笔），金额必填。
// 金额用 model.Amount 收，前端传数字或字符串都行，落库是 numeric(18,2)。
type receivablePayload struct {
	ProjectID *int         `json:"project_id"`
	Title     string       `json:"title"`
	Amount    model.Amount `json:"amount"`
	DueDate   string       `json:"due_date"`
	Note      string       `json:"note"`
	Status    string       `json:"status"`
}

func (p *receivablePayload) toModel() *model.Receivable {
	title := strings.TrimSpace(p.Title)
	if title == "" {
		title = "待收款"
	}
	return &model.Receivable{
		ProjectID: p.ProjectID,
		Title:     title,
		Amount:    p.Amount,
		DueDate:   optionalDate(p.DueDate),
		Note:      strings.TrimSpace(p.Note),
		Status:    p.Status,
	}
}

// optionalDate 空日期存 NULL，别在库里留半吊子的空串
func optionalDate(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

// GetAll GET /api/receivables?status=pending&project_id=1
func (h *ReceivableHandler) GetAll(c *gin.Context) {
	projectID, _ := strconv.Atoi(c.Query("project_id"))
	list, err := h.Repo.GetReceivables(userID(c), c.Query("status"), projectID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

// GetSummary GET /api/receivables/summary
func (h *ReceivableHandler) GetSummary(c *gin.Context) {
	sum, err := h.Repo.GetReceivableSummary(userID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, sum)
}

// Create POST /api/receivables
func (h *ReceivableHandler) Create(c *gin.Context) {
	var payload receivablePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if payload.Amount.IsNegative() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be >= 0"})
		return
	}

	item := payload.toModel()
	if err := h.Repo.CreateReceivable(userID(c), item); err != nil {
		respondError(c, err)
		return
	}
	created, err := h.Repo.GetReceivable(userID(c), item.ID)
	if err != nil || created == nil {
		c.JSON(http.StatusCreated, item)
		return
	}
	c.JSON(http.StatusCreated, created)
}

// Update PUT /api/receivables/:id —— 备注、金额、到期时间随时改
func (h *ReceivableHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var payload receivablePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if payload.Amount.IsNegative() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be >= 0"})
		return
	}

	item := payload.toModel()
	item.ID = id
	if err := h.Repo.UpdateReceivable(userID(c), item); err != nil {
		respondError(c, err)
		return
	}
	updated, err := h.Repo.GetReceivable(userID(c), id)
	if err != nil || updated == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reload receivable"})
		return
	}
	c.JSON(http.StatusOK, updated)
}

// SetStatus POST /api/receivables/:id/status {done: true}
func (h *ReceivableHandler) SetStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Done bool `json:"done"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Repo.SetReceivableStatus(userID(c), id, req.Done); err != nil {
		respondError(c, err)
		return
	}
	item, err := h.Repo.GetReceivable(userID(c), id)
	if err != nil || item == nil {
		c.JSON(http.StatusOK, gin.H{"message": "updated"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// Delete DELETE /api/receivables/:id
func (h *ReceivableHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.Repo.DeleteReceivable(userID(c), id); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
