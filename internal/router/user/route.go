package user

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/tararahuuw/ragsytem/internal/config"
	userctrl "github.com/tararahuuw/ragsytem/internal/controller/user"
	"github.com/tararahuuw/ragsytem/internal/middleware"
	userrepo "github.com/tararahuuw/ragsytem/internal/repository/user"
	usersvc "github.com/tararahuuw/ragsytem/internal/service/user"
)

// Register wires the user module and mounts its routes, all protected by JWT.
func Register(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB) {
	ctrl := userctrl.NewController(
		usersvc.NewService(
			userrepo.NewRepository(db),
		),
	)

	group := rg.Group("/users")
	group.Use(middleware.JWTAuth(cfg))
	{
		group.GET("/me", ctrl.Me)
		group.GET("/:id", ctrl.GetByID)
		group.PUT("/:id", ctrl.Update)
		group.DELETE("/:id", ctrl.Delete)
	}
}
