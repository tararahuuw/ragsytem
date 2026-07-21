package organization

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/tararahuuw/ragsytem/internal/config"
	orgctrl "github.com/tararahuuw/ragsytem/internal/controller/organization"
	cacheinfra "github.com/tararahuuw/ragsytem/internal/infra/cache"
	"github.com/tararahuuw/ragsytem/internal/middleware"
	"github.com/tararahuuw/ragsytem/internal/rbac"
	orgrepo "github.com/tararahuuw/ragsytem/internal/repository/organization"
	orgsvc "github.com/tararahuuw/ragsytem/internal/service/organization"
)

// Register wires the organization module. Read (list/get) is for any
// authenticated user; write (create/update/delete) is admin-only. The org
// registry is cached (hot ExistsActive + rarely-changing) with write invalidation.
func Register(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB, c cacheinfra.Cache) {
	repo := orgrepo.NewCachedRepository(orgrepo.NewRepository(db), c, cfg.CacheTTL)
	ctrl := orgctrl.NewController(orgsvc.NewService(repo))

	group := rg.Group("/organizations")
	group.Use(middleware.JWTAuth(cfg))
	{
		group.GET("", ctrl.List)
		group.GET("/:code", ctrl.Get)
		group.POST("", middleware.RequireRole(rbac.RoleAdmin), ctrl.Create)
		group.PUT("/:code", middleware.RequireRole(rbac.RoleAdmin), ctrl.Update)
		group.DELETE("/:code", middleware.RequireRole(rbac.RoleAdmin), ctrl.Delete)
	}
}
