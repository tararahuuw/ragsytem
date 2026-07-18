package chat

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/tararahuuw/ragsytem/internal/config"
	chatctrl "github.com/tararahuuw/ragsytem/internal/controller/chat"
	"github.com/tararahuuw/ragsytem/internal/infra/ai"
	"github.com/tararahuuw/ragsytem/internal/middleware"
	"github.com/tararahuuw/ragsytem/internal/ratelimit"
	chatrepo "github.com/tararahuuw/ragsytem/internal/repository/chat"
	chatsvc "github.com/tararahuuw/ragsytem/internal/service/chat"
)

// Register wires the chat module and mounts its routes (all require a JWT).
//
// The AI client is chosen by config (ai.NewClient): mock when AI_BASE_URL is
// empty, real HTTP client once the AI team's contract is finalized (§8c).
func Register(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB, rl *ratelimit.Limiter) {
	ctrl := chatctrl.NewController(
		chatsvc.NewService(
			chatrepo.NewRepository(db),
			ai.NewClient(cfg),
			cfg.AITimeout,
		),
	)

	group := rg.Group("/chat")
	group.Use(middleware.JWTAuth(cfg))
	{
		// /ask hits the (paid) AI service — rate-limited per user.
		group.POST("/ask", middleware.RateLimit(rl, "chat"), ctrl.Ask)
		group.GET("/sessions", ctrl.ListSessions)
		group.GET("/sessions/:id", ctrl.GetSession)
		group.DELETE("/sessions/:id", ctrl.DeleteSession)
	}
}
