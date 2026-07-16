package auth

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/tararahuuw/ragsytem/internal/config"
	authctrl "github.com/tararahuuw/ragsytem/internal/controller/auth"
	"github.com/tararahuuw/ragsytem/internal/middleware"
	"github.com/tararahuuw/ragsytem/internal/rbac"
	userrepo "github.com/tararahuuw/ragsytem/internal/repository/user"
	authsvc "github.com/tararahuuw/ragsytem/internal/service/auth"
)

// Register wires the auth module and mounts its public routes.
func Register(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB) {
	ctrl := authctrl.NewController(
		authsvc.NewService(
			userrepo.NewRepository(db),
			authsvc.Config{
				Secret:     cfg.JWTSecret,
				AccessTTL:  cfg.JWTAccessTTL,
				RefreshTTL: cfg.JWTRefreshTTL,
			},
		),
	)

	group := rg.Group("/auth")
	{
		// Public
		group.POST("/login", ctrl.Login)
		group.POST("/refresh", ctrl.Refresh)
		// Admin-only: creating users requires a valid admin access token.
		group.POST("/register",
			middleware.JWTAuth(cfg),
			middleware.RequireRole(rbac.RoleAdmin),
			ctrl.Register)
	}
}
