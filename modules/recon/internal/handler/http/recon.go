package handler

import (
	"net/http"
	"strconv"

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
