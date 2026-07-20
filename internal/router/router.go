package router

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"

	"github.com/tararahuuw/ragsytem/internal/config"
	minioinfra "github.com/tararahuuw/ragsytem/internal/infra/minio"
	"github.com/tararahuuw/ragsytem/internal/middleware"
	"github.com/tararahuuw/ragsytem/internal/ratelimit"

	authroute "github.com/tararahuuw/ragsytem/internal/router/auth"
	chatroute "github.com/tararahuuw/ragsytem/internal/router/chat"
	documentroute "github.com/tararahuuw/ragsytem/internal/router/document"
	healthroute "github.com/tararahuuw/ragsytem/internal/router/health"
	orgroute "github.com/tararahuuw/ragsytem/internal/router/organization"
	uploadroute "github.com/tararahuuw/ragsytem/internal/router/upload"
	userroute "github.com/tararahuuw/ragsytem/internal/router/user"
)

// New builds the Gin engine: global middleware, swagger UI, and versioned routes.
// Each module registers itself (and wires its own dependencies) via its Register
// function, keeping modules self-contained.
func New(cfg *config.Config, db *gorm.DB, store *minioinfra.Client) *gin.Engine {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	// Buffer only small multipart parts in memory; larger chunks spill to a temp
	// file instead of RAM (the per-request cap lives in the upload controller).
	r.MaxMultipartMemory = 16 << 20
	r.Use(
		middleware.RequestID(),  // set id first so Recovery/AccessLog can log it
		middleware.Recovery(),   // panic safety net -> standardized 500 + stack log
		middleware.AccessLog(),  // structured request log (slog) for tracing
		middleware.CORS(),
	)

	// swagger UI
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Per-category rate limiter (in-memory token buckets; see internal/ratelimit).
	rl := ratelimit.New(ratelimit.Config{
		Enabled: cfg.RateLimitEnabled,
		PerMinute: map[string]int{
			"auth":   cfg.RateLimitAuthPerMin,
			"chat":   cfg.RateLimitChatPerMin,
			"upload": cfg.RateLimitUploadPerMin,
		},
		DefaultPerMinute: 0, // un-categorized routes are not limited
	})

	// API v1 — register modules here
	v1 := r.Group("/api/v1")
	healthroute.Register(v1, db)
	authroute.Register(v1, cfg, db, rl)
	userroute.Register(v1, cfg, db)
	orgroute.Register(v1, cfg, db)
	uploadroute.Register(v1, cfg, db, store, rl)
	documentroute.Register(v1, cfg, db, store)
	chatroute.Register(v1, cfg, db, rl)

	return r
}
