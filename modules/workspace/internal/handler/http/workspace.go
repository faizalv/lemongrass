package handler

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/faizalv/lemongrass/config"
	"github.com/faizalv/lemongrass/modules/workspace/entity"
	transporter "github.com/faizalv/lemongrass/modules/workspace/transporter/http"
	"github.com/gin-gonic/gin"
)

type usecase interface {
	Create(ctx context.Context, ws entity.Workspace) (entity.Workspace, error)
	Get(ctx context.Context, id string) (entity.Workspace, error)
	ListByProject(ctx context.Context, projectID int64) ([]entity.Workspace, error)
	ReplaceRequirement(ctx context.Context, id, text, file, reqType string) error
	IsExecutionLocked(ctx context.Context, projectID int64) (bool, error)
	StartGrooming(ctx context.Context, workspaceID string) error
	GetTasks(ctx context.Context, workspaceID string) ([]entity.Task, error)
	ApproveCheckpoint(ctx context.Context, workspaceID string) error
	SaveTaskDecision(ctx context.Context, workspaceID, taskID string, approved bool, feedback string) error
	GetCheckpointDraft(ctx context.Context, workspaceID string) (map[string]entity.TaskDecision, error)
	SubmitCheckpointReviews(ctx context.Context, workspaceID string) error
}

type WorkspaceHandler struct {
	uc usecase
}

func New(uc usecase) *WorkspaceHandler {
	return &WorkspaceHandler{uc: uc}
}

var allowedExts = map[string]struct {
	reqType string
	maxSize int64
}{
	".txt":  {"text", 500 * 1024},
	".md":   {"text", 500 * 1024},
	".pdf":  {"pdf", 50 * 1024 * 1024},
	".png":  {"image", 20 * 1024 * 1024},
	".jpg":  {"image", 20 * 1024 * 1024},
	".jpeg": {"image", 20 * 1024 * 1024},
	".webp": {"image", 20 * 1024 * 1024},
	".gif":  {"image", 20 * 1024 * 1024},
}

func (h *WorkspaceHandler) Create(c *gin.Context) {
	if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
		h.createWithFile(c)
	} else {
		h.createWithText(c)
	}
}

func (h *WorkspaceHandler) createWithText(c *gin.Context) {
	var req transporter.CreateJSONRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	reqText := strings.TrimSpace(req.Requirement)
	reqType := ""
	if reqText != "" {
		reqType = "text"
	}
	ws, err := h.uc.Create(c.Request.Context(), entity.Workspace{
		ProjectID:       req.ProjectID,
		Name:            strings.TrimSpace(req.Name),
		RequirementText: reqText,
		RequirementType: reqType,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, transporter.ToResponse(ws))
}

func (h *WorkspaceHandler) createWithFile(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	projectID, err := strconv.ParseInt(c.PostForm("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
		return
	}
	fh, err := c.FormFile("requirement_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "requirement_file is required"})
		return
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	meta, ok := allowedExts[ext]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("unsupported file type: %s", ext)})
		return
	}
	if fh.Size > meta.maxSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file exceeds size limit"})
		return
	}

	ws, err := h.uc.Create(c.Request.Context(), entity.Workspace{
		ProjectID: projectID,
		Name:      name,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	filename := "requirement" + ext
	if err := saveFile(ws.ID, filename, fh); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}

	// ReplaceRequirement checks status; workspace was just created so it's idle.
	if err := h.uc.ReplaceRequirement(c.Request.Context(), ws.ID, "", filename, meta.reqType); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ws.RequirementFile = filename
	ws.RequirementType = meta.reqType
	c.JSON(http.StatusCreated, transporter.ToResponse(ws))
}

func (h *WorkspaceHandler) Get(c *gin.Context) {
	ws, err := h.uc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workspace not found"})
		return
	}
	c.JSON(http.StatusOK, transporter.ToResponse(ws))
}

func (h *WorkspaceHandler) ListByProject(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Query("project_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
		return
	}
	list, err := h.uc.ListByProject(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]transporter.WorkspaceResponse, len(list))
	for i, ws := range list {
		out[i] = transporter.ToResponse(ws)
	}
	c.JSON(http.StatusOK, out)
}

func (h *WorkspaceHandler) ReplaceRequirement(c *gin.Context) {
	id := c.Param("id")
	if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
		fh, err := c.FormFile("requirement_file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "requirement_file is required"})
			return
		}
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		meta, ok := allowedExts[ext]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("unsupported file type: %s", ext)})
			return
		}
		if fh.Size > meta.maxSize {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file exceeds size limit"})
			return
		}
		filename := "requirement" + ext
		if err := saveFile(id, filename, fh); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
			return
		}
		if err := h.uc.ReplaceRequirement(c.Request.Context(), id, "", filename, meta.reqType); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
	} else {
		var req struct {
			Requirement string `json:"requirement"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Requirement) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "requirement is required"})
			return
		}
		if err := h.uc.ReplaceRequirement(c.Request.Context(), id, req.Requirement, "", "text"); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
	}
	c.Status(http.StatusOK)
}

func (h *WorkspaceHandler) StartGrooming(c *gin.Context) {
	id := c.Param("id")
	if err := h.uc.StartGrooming(c.Request.Context(), id); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "must be idle") || strings.Contains(err.Error(), "not found") {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}

func (h *WorkspaceHandler) GetTasks(c *gin.Context) {
	id := c.Param("id")
	tasks, err := h.uc.GetTasks(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]transporter.TaskResponse, len(tasks))
	for i, t := range tasks {
		out[i] = transporter.ToTaskResponse(t)
	}
	c.JSON(http.StatusOK, out)
}

func (h *WorkspaceHandler) ApproveCheckpoint(c *gin.Context) {
	if err := h.uc.ApproveCheckpoint(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}

func (h *WorkspaceHandler) SaveTaskDecision(c *gin.Context) {
	var req transporter.TaskDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !req.Approved && strings.TrimSpace(req.Feedback) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "feedback is required when rejecting"})
		return
	}
	err := h.uc.SaveTaskDecision(c.Request.Context(), c.Param("id"), c.Param("task_id"), req.Approved, req.Feedback)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}

func (h *WorkspaceHandler) GetCheckpointDraft(c *gin.Context) {
	draft, err := h.uc.GetCheckpointDraft(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, draft)
}

func (h *WorkspaceHandler) SubmitCheckpointReviews(c *gin.Context) {
	if err := h.uc.SubmitCheckpointReviews(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}

func saveFile(wsID, filename string, fh *multipart.FileHeader) error {
	dir := filepath.Join(config.Dir(), "workspaces", wsID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(filepath.Join(dir, filename))
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}
