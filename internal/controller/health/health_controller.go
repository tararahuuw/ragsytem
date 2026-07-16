package health

import (
	"net/http"

	"github.com/gin-gonic/gin"

	healthdto "github.com/tararahuuw/ragsytem/internal/dto/health"
	healthsvc "github.com/tararahuuw/ragsytem/internal/service/health"
)

// Controller exposes health-check endpoints.
type Controller struct {
	svc healthsvc.Service
}

// NewController wires a health Controller over the given service.
func NewController(svc healthsvc.Service) *Controller {
	return &Controller{svc: svc}
}

// Check godoc
//
//	@Summary		Health check
//	@Description	Returns service and database health status
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	healthdto.HealthResponse
//	@Failure		503	{object}	healthdto.HealthResponse
//	@Router			/healthz [get]
func (c *Controller) Check(ctx *gin.Context) {
	// Health returns the probe struct directly (not the BaseResponse envelope)
	// so external uptime probes get a stable, minimal shape.
	var res healthdto.HealthResponse = c.svc.Check(ctx.Request.Context())

	status := http.StatusOK
	if res.Status != "ok" {
		status = http.StatusServiceUnavailable
	}
	ctx.JSON(status, res)
}
