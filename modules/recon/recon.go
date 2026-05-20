package recon

import (
	"github.com/faizalv/lemongrass/config"
	"github.com/faizalv/lemongrass/modules/recon/internal/usecase"
	"github.com/faizalv/lemongrass/modules/recon/internal/usecase/lang/golang"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

type Recon struct {
	uc *usecase.ReconUsecase
}

func (r *Recon) LoadMe(_ config.Config, _ *sqlx.DB, _ *redis.Client) {
	r.uc = usecase.New(golang.New())
}

// StartHTTPRouter is a no-op until #lg.recon.* handlers are wired in Phase 3.
func (r *Recon) StartHTTPRouter(_ *gin.RouterGroup) {}

func (r *Recon) Usecase() *usecase.ReconUsecase {
	return r.uc
}
