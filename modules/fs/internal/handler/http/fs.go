package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/faizalv/lemongrass/modules/fs/internal/usecase"
	transporter "github.com/faizalv/lemongrass/modules/fs/transporter/http"
	"github.com/gin-gonic/gin"
)

type FsHandler struct {
	uc *usecase.FsUsecase
}

func New(uc *usecase.FsUsecase) *FsHandler {
	return &FsHandler{uc: uc}
}

func (h *FsHandler) Browse(c *gin.Context) {
	force := c.Query("refresh") == "true"
	nodes, err := h.uc.Browse(force)
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

func (h *FsHandler) AddProject(c *gin.Context) {
	var req transporter.AddProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}

	if err := h.uc.AddProject(req.ToPayload()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusAccepted)
}

func (h *FsHandler) DeleteProject(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.uc.RemoveProject(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *FsHandler) ListArtifacts(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	artifacts, err := h.uc.ListArtifacts(c.Request.Context(), projectID, c.Query("type"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]transporter.ArtifactResponse, len(artifacts))
	for i, a := range artifacts {
		out[i] = transporter.ArtifactToResponse(a)
	}
	c.JSON(http.StatusOK, out)
}

func (h *FsHandler) CreateArtifact(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	var req transporter.CreateArtifactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Type) == "" || strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type and name are required"})
		return
	}
	artifact, err := h.uc.CreateArtifact(c.Request.Context(), projectID, req.Type, req.Name, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, transporter.ArtifactToResponse(artifact))
}

func (h *FsHandler) DeleteArtifact(c *gin.Context) {
	if err := h.uc.DeleteArtifact(c.Request.Context(), c.Param("artifact_id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *FsHandler) ListProjects(c *gin.Context) {
	projects, err := h.uc.ListProjects()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := make([]transporter.ProjectResponse, len(projects))
	for i, p := range projects {
		resp[i] = transporter.ProjectToResponse(p)
	}
	c.JSON(http.StatusOK, resp)
}
