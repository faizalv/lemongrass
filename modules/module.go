package modules

import (
	"github.com/faizalv/lemongrass/config"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

type Module interface {
	LoadMe(cfg config.Config, db *sqlx.DB, rds *redis.Client)
	StartHTTPRouter(rg *gin.RouterGroup)
}
