package handler

import (
	"net/http"

	"github.com/faizalv/lemongrass/modules/lg/internal/usecase"
	transporter "github.com/faizalv/lemongrass/modules/lg/transporter/http"
	"github.com/gin-gonic/gin"
)

type LgHandler struct {
	uc *usecase.LgUsecase
}

func New(uc *usecase.LgUsecase) *LgHandler {
	return &LgHandler{uc: uc}
}

func (h *LgHandler) Receive(c *gin.Context) {
	var req struct {
		Cmd  string `json:"cmd"`
		Args string `json:"args"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Cmd == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cmd is required"})
		return
	}
	h.uc.RecordCall(req.Cmd, req.Args)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *LgHandler) Calls(c *gin.Context) {
	calls := h.uc.ListCalls()
	resp := make([]transporter.CallResponse, len(calls))
	for i, call := range calls {
		resp[i] = transporter.ToCallResponse(call)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *LgHandler) Send(c *gin.Context) {
	var req struct {
		Message string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message is required"})
		return
	}
	h.uc.Send(req.Message)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
