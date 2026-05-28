package modules

import (
	"github.com/faizalv/lemongrass/config"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type Module interface {
	LoadMe(cfg config.Config, db *sqlx.DB)
	StartHTTPRouter(rg *gin.RouterGroup)
}
