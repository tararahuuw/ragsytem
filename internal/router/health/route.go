package health

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	healthctrl "github.com/tararahuuw/ragsytem/internal/controller/health"
	healthrepo "github.com/tararahuuw/ragsytem/internal/repository/health"
	healthsvc "github.com/tararahuuw/ragsytem/internal/service/health"
)

// Register wires the health module (repository -> service -> controller) and
// mounts its routes onto the given group.
func Register(rg *gin.RouterGroup, db *gorm.DB) {
	ctrl := healthctrl.NewController(
		healthsvc.NewService(
			healthrepo.NewRepository(db),
		),
	)

	rg.GET("/healthz", ctrl.Check)
}
