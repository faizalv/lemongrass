package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os/exec"
	"time"

	"github.com/faizalv/lemongrass/modules/debug/internal/usecase"
	"github.com/gin-gonic/gin"
)

type DebugHandler struct {
	uc *usecase.DebugUsecase
}

func New(uc *usecase.DebugUsecase) *DebugHandler {
	return &DebugHandler{uc: uc}
}

func (h *DebugHandler) Send(c *gin.Context) {
	var req struct {
		Message string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message is required"})
		return
	}
	h.uc.Send(req.Message)
	c.Status(http.StatusNoContent)
}

func (h *DebugHandler) ExecHook(c *gin.Context) {
	var req struct {
		WorkspaceID string `json:"workspace_id"`
		Command     string `json:"command"`
		SessionType string `json:"session_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Command == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "command is required"})
		return
	}
	sessionType := req.SessionType
	if sessionType == "" {
		sessionType = "grooming"
	}

	event, _ := json.Marshal(map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": req.Command},
	})

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "exec", "-i",
		"-e", "LG_SESSION_ID="+req.WorkspaceID,
		"-e", "LG_SESSION_TYPE="+sessionType,
		"lg-runner",
		"lg-hook",
	)
	cmd.Stdin = bytes.NewReader(event)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Run()

	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	c.JSON(http.StatusOK, gin.H{
		"output":    stdout.String(),
		"stderr":    stderr.String(),
		"exit_code": exitCode,
	})
}
