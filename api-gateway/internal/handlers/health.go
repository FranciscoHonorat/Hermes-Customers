package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthHandler godoc
// @Summary      Health Check
// @Description  Verifica se o API Gateway está funcionando e retorna informações de saúde do serviço
// @Tags         System
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]string  "Serviço está funcionando corretamente"
// @Router       /health [get]
func HealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Serviço está funcionando corretamente",
	})
}
