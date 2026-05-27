package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthHandler retorna 200 OK para liveness/readiness probes do Kubernetes.
func HealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
