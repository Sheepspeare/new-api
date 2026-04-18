//go:build !waffo
// +build !waffo

package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const waffoPancakeFeatureDisabledMessage = "Waffo Pancake feature is not built. Rebuild with -tags waffo to enable it."

func RequestWaffoPancakeAmount(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "error",
		"data":    waffoPancakeFeatureDisabledMessage,
	})
}

func RequestWaffoPancakePay(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "error",
		"data":    waffoPancakeFeatureDisabledMessage,
	})
}

func WaffoPancakeWebhook(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
		"success": false,
		"message": waffoPancakeFeatureDisabledMessage,
	})
}
