package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/faizalv/lemongrass/modules/pty/internal/usecase"
)

type PtyHandler struct {
	uc *usecase.PtyUsecase
}

func New(uc *usecase.PtyUsecase) *PtyHandler {
	return &PtyHandler{uc: uc}
}

func (h *PtyHandler) Test(c *gin.Context) {
	session, err := h.uc.RunTest()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"session_id": session.ID, "output": session.Output})
}

func (h *PtyHandler) Send(c *gin.Context) {
	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Prompt == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prompt is required"})
		return
	}
	session, err := h.uc.Run(req.Prompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"session_id": session.ID, "output": session.Output})
}
