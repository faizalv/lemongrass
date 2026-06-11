package handler

import (
	"net/http"

	"github.com/faizalv/lemongrass/modules/lg/entity"
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
		ProjectID int64  `json:"project_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Cmd == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cmd is required"})
		return
	}
	var text string
	switch {
	case req.ProjectID > 0 && req.SessionID != "":
		text = h.uc.HandleOrCreateSession(req.ProjectID, req.SessionID, req.Cmd, req.Args, req.Blocking)
	case req.ProjectID > 0:
		text = h.uc.HandleByProject(req.ProjectID, req.Cmd, req.Args, req.Blocking)
	default:
		text = h.uc.Handle(req.SessionID, req.Cmd, req.Args, req.Blocking)
	}
	c.JSON(http.StatusOK, gin.H{"text": text})
}

func (h *LgHandler) Calls(c *gin.Context) {
	var calls []entity.Call
	if ws := c.Query("workspace"); ws != "" {
		calls = h.uc.ListCallsByWorkspace(ws)
	} else {
		calls = h.uc.ListCalls()
	}
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

func (h *LgHandler) Usage(c *gin.Context) {
	data := h.uc.GetUsage(c.Request.Context())
	c.JSON(http.StatusOK, data)
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
