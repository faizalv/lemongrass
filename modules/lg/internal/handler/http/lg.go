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
		Cmd       string `json:"cmd"`
		Args      string `json:"args"`
		Blocking  bool   `json:"blocking"`
		SessionID string `json:"session_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Cmd == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cmd is required"})
		return
	}
	text := h.uc.Handle(req.SessionID, req.Cmd, req.Args, req.Blocking)
	c.JSON(http.StatusOK, gin.H{"text": text})
}

func (h *LgHandler) Calls(c *gin.Context) {
	calls := h.uc.ListCalls()
	resp := make([]transporter.CallResponse, len(calls))
	for i, call := range calls {
		resp[i] = transporter.ToCallResponse(call)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *LgHandler) WriteTrail(c *gin.Context) {
	var req struct {
		SessionID string `json:"session_id"`
		FilePath  string `json:"file_path"`
		ByteCount int    `json:"byte_count"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.uc.LogWrite(req.SessionID, req.FilePath, req.ByteCount)
	c.Status(http.StatusOK)
}

func (h *LgHandler) GetWriteTrail(c *gin.Context) {
	sessionID := c.Query("session")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session is required"})
		return
	}
	entries := h.uc.GetWriteTrail(sessionID)
	resp := make([]transporter.WriteTrailResponse, len(entries))
	for i, e := range entries {
		resp[i] = transporter.ToWriteTrailResponse(e)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *LgHandler) ExecutionDiff(c *gin.Context) {
	sessionID := c.Query("session")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session is required"})
		return
	}
	diffs := h.uc.GetExecutionDiff(sessionID)
	resp := make([]transporter.FileDiffResponse, len(diffs))
	for i, d := range diffs {
		resp[i] = transporter.ToFileDiffResponse(d)
	}
	c.JSON(http.StatusOK, gin.H{"files": resp})
}
