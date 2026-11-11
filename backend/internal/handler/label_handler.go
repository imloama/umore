package handler

import (
	"net/http"
	"strconv"

	"github.com/imloama/umore/internal/model"

	"github.com/gin-gonic/gin"
)

type LabelHandler struct {
	Handler
}

func (h *LabelHandler) GetAll(c *gin.Context) {
	labels, err := h.Repo.GetLabels(userID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, labels)
}

func (h *LabelHandler) Create(c *gin.Context) {
	var label model.Label
	if err := c.ShouldBindJSON(&label); err != nil {
		fail(c, http.StatusBadRequest, codeInvalidParams, err.Error())
		return
	}
	if label.Name == "" {
		fail(c, http.StatusBadRequest, codeLabelNameRequired, "name is required")
		return
	}
	if err := h.Repo.CreateLabel(userID(c), &label); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, label)
}

func (h *LabelHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		fail(c, http.StatusBadRequest, codeInvalidID, "invalid id")
		return
	}
	var label model.Label
	if err := c.ShouldBindJSON(&label); err != nil {
		fail(c, http.StatusBadRequest, codeInvalidParams, err.Error())
		return
	}
	label.ID = id
	if err := h.Repo.UpdateLabel(userID(c), &label); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, label)
}

func (h *LabelHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		fail(c, http.StatusBadRequest, codeInvalidID, "invalid id")
		return
	}
	if err := h.Repo.DeleteLabel(userID(c), id); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
