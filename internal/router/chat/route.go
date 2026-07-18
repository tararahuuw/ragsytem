package chat

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/tararahuuw/ragsytem/internal/config"
	chatctrl "github.com/tararahuuw/ragsytem/internal/controller/chat"
	"github.com/tararahuuw/ragsytem/internal/infra/ai"
	"github.com/tararahuuw/ragsytem/internal/middleware"
	chatrepo "github.com/tararahuuw/ragsytem/internal/repository/chat"
	chatsvc "github.com/tararahuuw/ragsytem/internal/service/chat"
)

// Register wires the chat module and mounts its routes (all require a JWT).
//
// The AI client is a mock for now (see internal/infra/ai); swap NewMockClient()
// for the real HTTP client once the AI team's contract is finalized.
func Register(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB) {
	ctrl := chatctrl.NewController(
		chatsvc.NewService(
			chatrepo.NewRepository(db),
			ai.NewMockClient(),
		),
	)

	group := rg.Group("/chat")
	group.Use(middleware.JWTAuth(cfg))
	{
		group.POST("/ask", ctrl.Ask)
		group.GET("/sessions", ctrl.ListSessions)
		group.GET("/sessions/:id", ctrl.GetSession)
		group.DELETE("/sessions/:id", ctrl.DeleteSession)
	}
}
