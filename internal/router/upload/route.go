package upload

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/tararahuuw/ragsytem/internal/config"
	uploadctrl "github.com/tararahuuw/ragsytem/internal/controller/upload"
	minioinfra "github.com/tararahuuw/ragsytem/internal/infra/minio"
	"github.com/tararahuuw/ragsytem/internal/middleware"
	uploadrepo "github.com/tararahuuw/ragsytem/internal/repository/upload"
	uploadsvc "github.com/tararahuuw/ragsytem/internal/service/upload"
)

// Register wires the upload module and mounts its routes (all require a JWT).
func Register(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB, store *minioinfra.Client) {
	ctrl := uploadctrl.NewController(
		uploadsvc.NewService(
			uploadrepo.NewRepository(db),
			store,
			uploadsvc.Config{
				MaxFileSize:   cfg.UploadMaxFileSize,
				PreviewExpiry: cfg.UploadPreviewExpiry,
			},
		),
	)

	group := rg.Group("/uploads")
	group.Use(middleware.JWTAuth(cfg))
	{
		group.POST("/chunk", ctrl.Chunk)
	}
}
