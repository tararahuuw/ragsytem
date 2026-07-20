package auth

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/tararahuuw/ragsytem/internal/config"
	authctrl "github.com/tararahuuw/ragsytem/internal/controller/auth"
	"github.com/tararahuuw/ragsytem/internal/infra/email"
	"github.com/tararahuuw/ragsytem/internal/middleware"
	"github.com/tararahuuw/ragsytem/internal/ratelimit"
	"github.com/tararahuuw/ragsytem/internal/rbac"
	orgrepo "github.com/tararahuuw/ragsytem/internal/repository/organization"
	userrepo "github.com/tararahuuw/ragsytem/internal/repository/user"
	authsvc "github.com/tararahuuw/ragsytem/internal/service/auth"
)

// Register wires the auth module and mounts its public routes.
func Register(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB, rl *ratelimit.Limiter) {
	ctrl := authctrl.NewController(
		authsvc.NewService(
			userrepo.NewRepository(db),
			orgrepo.NewRepository(db),
			email.NewSender(cfg),
			authsvc.Config{
				Secret:           cfg.JWTSecret,
				AccessTTL:        cfg.JWTAccessTTL,
				RefreshTTL:       cfg.JWTRefreshTTL,
				AppBaseURL:       cfg.AppBaseURL,
				PasswordResetTTL: cfg.PasswordResetTTL,
			},
		),
	)

	group := rg.Group("/auth")
	{
		// Public — rate-limited per IP (anti brute-force).
		group.POST("/login", middleware.RateLimit(rl, "auth"), ctrl.Login)
		group.POST("/refresh", middleware.RateLimit(rl, "auth"), ctrl.Refresh)
		// Password reset (public) — rate-limited per IP (anti abuse/enumeration).
		group.POST("/forgot-password", middleware.RateLimit(rl, "auth"), ctrl.ForgotPassword)
		group.POST("/reset-password", middleware.RateLimit(rl, "auth"), ctrl.ResetPassword)
		// Authenticated self-service.
		group.POST("/change-password", middleware.JWTAuth(cfg), ctrl.ChangePassword)
		group.POST("/logout", middleware.JWTAuth(cfg), ctrl.Logout)
		// Admin-only: creating users requires a valid admin access token.
		group.POST("/register",
			middleware.JWTAuth(cfg),
			middleware.RequireRole(rbac.RoleAdmin),
			ctrl.Register)
		group.POST("/register/bulk",
			middleware.JWTAuth(cfg),
			middleware.RequireRole(rbac.RoleAdmin),
			ctrl.BulkRegister)
	}
}
