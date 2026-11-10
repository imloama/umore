package handler

import (
	"net/http"
	"strconv"

	"github.com/imloama/umore/internal/model"

	"github.com/gin-gonic/gin"
)

type FinanceHandler struct {
	Handler
}

func (h *FinanceHandler) GetByRange(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date and end_date required"})
		return
	}

	finances, err := h.Repo.GetFinances(userID(c), startDate, endDate)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, finances)
}

func (h *FinanceHandler) GetSummary(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date and end_date required"})
		return
	}

	summary, err := h.Repo.GetFinanceSummary(userID(c), startDate, endDate)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, summary)
}

func (h *FinanceHandler) Create(c *gin.Context) {
	var finance model.Finance
	if err := c.ShouldBindJSON(&finance); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.Repo.CreateFinance(userID(c), &finance); err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, finance)
}

func (h *FinanceHandler) Update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var finance model.Finance
	if err := c.ShouldBindJSON(&finance); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	finance.ID = id

	if err := h.Repo.UpdateFinance(userID(c), &finance); err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, finance)
}

func (h *FinanceHandler) Delete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.Repo.DeleteFinance(userID(c), id); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
