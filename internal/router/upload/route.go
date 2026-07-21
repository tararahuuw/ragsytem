package upload

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/tararahuuw/ragsytem/internal/config"
	uploadctrl "github.com/tararahuuw/ragsytem/internal/controller/upload"
	cacheinfra "github.com/tararahuuw/ragsytem/internal/infra/cache"
	minioinfra "github.com/tararahuuw/ragsytem/internal/infra/minio"
	"github.com/tararahuuw/ragsytem/internal/middleware"
	"github.com/tararahuuw/ragsytem/internal/ratelimit"
	uploadrepo "github.com/tararahuuw/ragsytem/internal/repository/upload"
	uploadsvc "github.com/tararahuuw/ragsytem/internal/service/upload"
)

// Register wires the upload module and mounts its routes (all require a JWT). The
// cache is passed so a completed upload can invalidate the document-list cache.
func Register(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB, store *minioinfra.Client, rl *ratelimit.Limiter, c cacheinfra.Cache) {
	ctrl := uploadctrl.NewController(
		uploadsvc.NewService(
			uploadrepo.NewRepository(db),
			store,
			c,
			uploadsvc.Config{
				MaxFileSize:   cfg.UploadMaxFileSize,
				PreviewExpiry: cfg.UploadPreviewExpiry,
			},
		),
		// per-request body cap = file cap + slack for multipart boundaries/fields.
		cfg.UploadMaxFileSize+(32<<20),
	)

	group := rg.Group("/uploads")
	group.Use(middleware.JWTAuth(cfg))
	{
		// generous per-user limit (a chunked file = many requests).
		group.POST("/chunk", middleware.RateLimit(rl, "upload"), ctrl.Chunk)
	}
}
