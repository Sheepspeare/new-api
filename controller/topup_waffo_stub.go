//go:build !waffo
// +build !waffo

package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const waffoFeatureDisabledMessage = "Waffo feature is not built. Rebuild with -tags waffo to enable it."

func RequestWaffoAmount(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "error",
		"data":    waffoFeatureDisabledMessage,
	})
}

func RequestWaffoPay(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "error",
		"data":    waffoFeatureDisabledMessage,
	})
}

func WaffoWebhook(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
		"success": false,
		"message": waffoFeatureDisabledMessage,
	})
}
