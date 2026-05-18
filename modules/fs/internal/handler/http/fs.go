package handler

import (
	"net/http"
	"strconv"

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

func (h *FsHandler) Attach(c *gin.Context) {
	var req transporter.AttachRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}

	if err := h.uc.Attach(req.ToPayload()); err != nil {
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
