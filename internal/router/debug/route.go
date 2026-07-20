package debug

import (
	"github.com/gin-gonic/gin"

	debugctrl "github.com/tararahuuw/ragsytem/internal/controller/debug"
)

// Register mounts the debug/Sentry-verification routes. The caller (router.New)
// is responsible for only invoking this outside production.
func Register(rg *gin.RouterGroup) {
	ctrl := debugctrl.NewController()

	group := rg.Group("/debug")
	{
		group.GET("/error", ctrl.ForceError)
		group.GET("/panic", ctrl.ForcePanic)
		group.GET("/message", ctrl.ForceMessage)
	}
}
