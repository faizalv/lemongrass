package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/faizalv/lemongrass/config"
	lgentity "github.com/faizalv/lemongrass/modules/lg/entity"
	"github.com/faizalv/lemongrass/modules/workspace/entity"
	transporter "github.com/faizalv/lemongrass/modules/workspace/transporter/http"
	"github.com/gin-gonic/gin"
)

type workspaceUC interface {
	Create(ctx context.Context, ws entity.Workspace) (entity.Workspace, error)
	Get(ctx context.Context, id string) (entity.Workspace, error)
	ListByProject(ctx context.Context, projectID int64, includeDeleted bool) ([]entity.Workspace, error)
	DeleteWorkspace(ctx context.Context, id string) error
	IsExecutionLocked(ctx context.Context, projectID int64) (bool, error)
}

type groomingUC interface {
	StartGrooming(ctx context.Context, workspaceID string) error
}

type checkpointUC interface {
	GetTasks(ctx context.Context, workspaceID string) ([]entity.Task, error)
	ApproveCheckpoint(ctx context.Context, workspaceID string) error
	SaveTaskDecision(ctx context.Context, workspaceID, taskID string, approved bool, feedback string) error
	GetCheckpointDraft(ctx context.Context, workspaceID string) (map[string]entity.TaskDecision, error)
	SubmitCheckpointReviews(ctx context.Context, workspaceID string) error
}

type requirementUC interface {
	ListRequirements(ctx context.Context, workspaceID string) ([]entity.WorkspaceRequirement, error)
	AddTextRequirement(ctx context.Context, workspaceID, text string) (entity.WorkspaceRequirement, error)
	AddFileRequirement(ctx context.Context, workspaceID, reqType, filePath, fileName string) (entity.WorkspaceRequirement, error)
	DeleteRequirement(ctx context.Context, workspaceID, reqID string) error
}

type sessionUC interface {
	GetSessionActivity(ctx context.Context, workspaceID string) (time.Time, int, []lgentity.EchoMessage, bool)
	ResetSession(ctx context.Context, workspaceID string) error
}

type executionUC interface {
	StartExecution(ctx context.Context, workspaceID string) error
	ForceStopExecution(ctx context.Context, workspaceID string) error
}

type WorkspaceHandler struct {
	workspace   workspaceUC
	grooming    groomingUC
	checkpoint  checkpointUC
	requirement requirementUC
	session     sessionUC
	execution   executionUC
}

func New(ws workspaceUC, gr groomingUC, cp checkpointUC, rq requirementUC, sess sessionUC, exec executionUC) *WorkspaceHandler {
	return &WorkspaceHandler{
		workspace:   ws,
		grooming:    gr,
		checkpoint:  cp,
		requirement: rq,
		session:     sess,
		execution:   exec,
	}
}

var allowedFileExts = map[string]struct {
	reqType string
	maxSize int64
}{
	".pdf":  {"pdf", 50 * 1024 * 1024},
	".png":  {"image", 20 * 1024 * 1024},
	".jpg":  {"image", 20 * 1024 * 1024},
	".jpeg": {"image", 20 * 1024 * 1024},
	".webp": {"image", 20 * 1024 * 1024},
	".gif":  {"image", 20 * 1024 * 1024},
}

func (h *WorkspaceHandler) Create(c *gin.Context) {
	var req transporter.CreateJSONRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	ws, err := h.workspace.Create(c.Request.Context(), entity.Workspace{
		ProjectID: req.ProjectID,
		Name:      strings.TrimSpace(req.Name),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, transporter.ToResponse(ws))
}

func (h *WorkspaceHandler) Get(c *gin.Context) {
	ws, err := h.workspace.Get(c.Request.Context(), c.Param("id"))
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
	includeDeleted := c.Query("include_deleted") == "true"
	list, err := h.workspace.ListByProject(c.Request.Context(), projectID, includeDeleted)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if includeDeleted {
		out := make([]transporter.WorkspaceWithRequirementsResponse, 0, len(list))
		for _, ws := range list {
			reqs, _ := h.requirement.ListRequirements(c.Request.Context(), ws.ID)
			reqOut := make([]transporter.WorkspaceRequirementResponse, len(reqs))
			for j, r := range reqs {
				reqOut[j] = transporter.ToRequirementResponse(r)
			}
			out = append(out, transporter.WorkspaceWithRequirementsResponse{
				WorkspaceResponse: transporter.ToResponse(ws),
				Requirements:      reqOut,
			})
		}
		c.JSON(http.StatusOK, out)
		return
	}
	out := make([]transporter.WorkspaceResponse, len(list))
	for i, ws := range list {
		out[i] = transporter.ToResponse(ws)
	}
	c.JSON(http.StatusOK, out)
}

func (h *WorkspaceHandler) Delete(c *gin.Context) {
	if err := h.workspace.DeleteWorkspace(c.Request.Context(), c.Param("id")); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "must be idle") {
			status = http.StatusConflict
		} else if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *WorkspaceHandler) StartGrooming(c *gin.Context) {
	if err := h.grooming.StartGrooming(c.Request.Context(), c.Param("id")); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "must be idle") ||
			strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "no requirements") {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}

func (h *WorkspaceHandler) GetTasks(c *gin.Context) {
	tasks, err := h.checkpoint.GetTasks(c.Request.Context(), c.Param("id"))
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
	if err := h.checkpoint.ApproveCheckpoint(c.Request.Context(), c.Param("id")); err != nil {
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
	if err := h.checkpoint.SaveTaskDecision(c.Request.Context(), c.Param("id"), c.Param("task_id"), req.Approved, req.Feedback); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}

func (h *WorkspaceHandler) GetCheckpointDraft(c *gin.Context) {
	draft, err := h.checkpoint.GetCheckpointDraft(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, draft)
}

func (h *WorkspaceHandler) SubmitCheckpointReviews(c *gin.Context) {
	if err := h.checkpoint.SubmitCheckpointReviews(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}

func (h *WorkspaceHandler) ListRequirements(c *gin.Context) {
	reqs, err := h.requirement.ListRequirements(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]transporter.WorkspaceRequirementResponse, len(reqs))
	for i, r := range reqs {
		out[i] = transporter.ToRequirementResponse(r)
	}
	c.JSON(http.StatusOK, out)
}

func (h *WorkspaceHandler) AddRequirement(c *gin.Context) {
	id := c.Param("id")
	if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
		h.addFileRequirement(c, id)
	} else {
		h.addTextRequirement(c, id)
	}
}

func (h *WorkspaceHandler) addTextRequirement(c *gin.Context, wsID string) {
	var req transporter.AddTextRequirementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	text := strings.TrimSpace(req.TextContent)
	if len(text) < 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text must be at least 10 characters"})
		return
	}
	if int64(len(text)) > 500*1024 {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "text exceeds 500 KB limit"})
		return
	}
	result, err := h.requirement.AddTextRequirement(c.Request.Context(), wsID, text)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "locked") {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, transporter.ToRequirementResponse(result))
}

func (h *WorkspaceHandler) addFileRequirement(c *gin.Context, wsID string) {
	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	meta, ok := allowedFileExts[ext]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("unsupported file type: %s", ext)})
		return
	}
	if fh.Size > meta.maxSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file exceeds size limit"})
		return
	}
	b := make([]byte, 8)
	rand.Read(b)
	filename := hex.EncodeToString(b) + ext
	if err := saveFile(wsID, filename, fh); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}
	result, err := h.requirement.AddFileRequirement(c.Request.Context(), wsID, meta.reqType, filename, fh.Filename)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "locked") {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, transporter.ToRequirementResponse(result))
}

func (h *WorkspaceHandler) DeleteRequirement(c *gin.Context) {
	if err := h.requirement.DeleteRequirement(c.Request.Context(), c.Param("id"), c.Param("req_id")); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "locked") {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *WorkspaceHandler) SessionActivity(c *gin.Context) {
	lastAt, idleSec, echoes, active := h.session.GetSessionActivity(c.Request.Context(), c.Param("id"))
	msgs := make([]transporter.EchoMessageResponse, len(echoes))
	for i, e := range echoes {
		msgs[i] = transporter.EchoMessageResponse{
			Ts:   e.Timestamp.UTC().Format("2006-01-02T15:04:05Z"),
			Text: e.Text,
		}
	}
	resp := transporter.SessionActivityResponse{
		IdleSeconds: idleSec,
		Messages:    msgs,
	}
	if active {
		s := lastAt.UTC().Format("2006-01-02T15:04:05Z")
		resp.LastActivityAt = &s
	}
	c.JSON(http.StatusOK, resp)
}

func (h *WorkspaceHandler) SessionReset(c *gin.Context) {
	if err := h.session.ResetSession(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *WorkspaceHandler) StartExecution(c *gin.Context) {
	if err := h.execution.StartExecution(c.Request.Context(), c.Param("id")); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "already executing") {
			status = http.StatusConflict
		} else if strings.Contains(err.Error(), "git checkout") {
			status = http.StatusUnprocessableEntity
		} else if strings.Contains(err.Error(), "must be awaiting_execution") {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *WorkspaceHandler) ForceStopExecution(c *gin.Context) {
	if err := h.execution.ForceStopExecution(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
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
