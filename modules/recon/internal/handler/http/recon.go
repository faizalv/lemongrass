package handler

import (
	"io"
	"net/http"
	"strconv"

	"github.com/faizalv/lemongrass/config"
	"github.com/faizalv/lemongrass/infra/lgart"
	"github.com/faizalv/lemongrass/modules/recon/internal/usecase"
	transporter "github.com/faizalv/lemongrass/modules/recon/transporter/http"
	"github.com/gin-gonic/gin"
)

type ReconHandler struct {
	uc *usecase.ReconUsecase
}

func New(uc *usecase.ReconUsecase) *ReconHandler {
	return &ReconHandler{uc: uc}
}

func (h *ReconHandler) ListNodes(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	nodes, err := h.uc.ListNodes(c.Request.Context(), projectID,
		c.Query("language"),
		c.Query("kind"),
		c.Query("status"),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := make([]transporter.NodeResponse, len(nodes))
	for i, n := range nodes {
		resp[i] = transporter.NodeToResponse(n)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *ReconHandler) GetCoverage(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	coverage, err := h.uc.GetCoverage(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := make([]transporter.LangCoverageResponse, len(coverage))
	for i, cov := range coverage {
		resp[i] = transporter.CoverageToResponse(cov)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *ReconHandler) ListKnowledge(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	entries, err := h.uc.ListKnowledge(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp := make([]transporter.KnowledgeResponse, len(entries))
	for i, e := range entries {
		resp[i] = transporter.KnowledgeToResponse(e)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *ReconHandler) Activate(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	h.uc.Activate(projectID)
	c.JSON(http.StatusAccepted, gin.H{"status": "syncing"})
}

func (h *ReconHandler) SyncStatus(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	syncing, lastNano := h.uc.SyncStatus(projectID)

	interval, _ := h.uc.GetSyncInterval(c.Request.Context(), projectID)

	resp := gin.H{
		"syncing":       syncing,
		"sync_interval": interval,
	}
	if lastNano > 0 {
		resp["last_synced"] = lastNano
	}
	c.JSON(http.StatusOK, resp)
}

func (h *ReconHandler) UpdateSyncInterval(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	var req struct {
		Interval string `json:"sync_interval"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	switch req.Interval {
	case "off", "5m", "15m", "30m", "1h":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid interval"})
		return
	}
	if err := h.uc.UpdateSyncInterval(c.Request.Context(), projectID, req.Interval); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sync_interval": req.Interval})
}

func (h *ReconHandler) GitStatus(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	status, err := h.uc.GitStatus(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, transporter.GitStatusToResponse(status))
}

func (h *ReconHandler) GitInit(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	if err := h.uc.GitInit(c.Request.Context(), projectID); err != nil {
		if err.Error() == "already a git repository" {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ReconHandler) EmbedStatus(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	pending, total, current, recent, err := h.uc.EmbedStatus(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"pending": pending,
		"total":   total,
		"current": current,
		"recent":  recent,
	})
}

func (h *ReconHandler) Export(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	f, err := h.uc.ExportArtifacts(
		c.Request.Context(), projectID, config.ReadOriginID(),
		c.Query("project_label"), c.Query("git_origin"), c.Query("git_user"),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	data, err := lgart.Encode(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/octet-stream", data)
}

func (h *ReconHandler) Import(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}
	f, err := lgart.Decode(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lgart file: " + err.Error()})
		return
	}
	force := c.Query("force") == "true"
	result, err := h.uc.ImportArtifacts(c.Request.Context(), projectID, f, force)
	if err != nil {
		if err.Error() == "semantic map not ready" {
			c.JSON(http.StatusConflict, gin.H{"error": "semantic map not ready -- wait for initial sync to complete"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *ReconHandler) ImportOverlap(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	var req struct {
		Keys []string `json:"keys"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	matched, total, ready, err := h.uc.NodeOverlap(c.Request.Context(), projectID, req.Keys)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"matched": matched, "total": total, "ready": ready})
}

func (h *ReconHandler) GetLgIgnore(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	patterns, err := h.uc.GetLgIgnorePatterns(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"patterns": patterns})
}

func (h *ReconHandler) Prune(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	orphanDays := 30
	if s := c.Query("orphan_days"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			orphanDays = n
		}
	}
	superseded, orphans, err := h.uc.Prune(c.Request.Context(), projectID, orphanDays)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"superseded": superseded, "orphans": orphans})
}
