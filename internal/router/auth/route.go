package auth

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	authctrl "github.com/tararahuuw/ragsytem/internal/controller/auth"
	authrepo "github.com/tararahuuw/ragsytem/internal/repository/auth"
	authsvc "github.com/tararahuuw/ragsytem/internal/service/auth"
)

// Register wires the auth module (repository -> service -> controller) and
// mounts its routes onto the given group.
//
// db is unused for now: the dummy repository is in-memory. Keep the parameter
// so switching to a GORM-backed repository later is a one-line change here.
func Register(rg *gin.RouterGroup, db *gorm.DB) {
	ctrl := authctrl.NewController(
		authsvc.NewService(
			authrepo.NewRepository(),
		),
	)

	group := rg.Group("/auth")
	{
		group.POST("/register", ctrl.Register)
		group.POST("/login", ctrl.Login)
	}
}
