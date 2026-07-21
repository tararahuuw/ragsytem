package document

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/tararahuuw/ragsytem/internal/config"
	documentctrl "github.com/tararahuuw/ragsytem/internal/controller/document"
	cacheinfra "github.com/tararahuuw/ragsytem/internal/infra/cache"
	minioinfra "github.com/tararahuuw/ragsytem/internal/infra/minio"
	"github.com/tararahuuw/ragsytem/internal/middleware"
	documentrepo "github.com/tararahuuw/ragsytem/internal/repository/document"
	documentsvc "github.com/tararahuuw/ragsytem/internal/service/document"
)

// Register wires the document module and mounts its routes (all require a JWT).
// Document metadata is cached (immutable, read-heavy); presigned URLs stay fresh
// (generated in the service). The list cache is invalidated by the upload module.
func Register(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB, store *minioinfra.Client, c cacheinfra.Cache) {
	repo := documentrepo.NewCachedRepository(documentrepo.NewRepository(db), c, cfg.CacheTTL)
	ctrl := documentctrl.NewController(
		documentsvc.NewService(
			repo,
			store,
			documentsvc.Config{PreviewExpiry: cfg.UploadPreviewExpiry},
		),
	)

	group := rg.Group("/documents")
	group.Use(middleware.JWTAuth(cfg))
	{
		group.GET("", ctrl.List)
		group.GET("/:id", ctrl.GetByID)
	}
}
