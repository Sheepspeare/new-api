package controller

import (
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

var messageHookService = service.NewMessageHookService()

// GetMessageHooks retrieves all message hooks with pagination
func GetMessageHooks(c *gin.Context) {
	// Check admin role
	if !isAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Admin access required",
		})
		return
	}

	// Parse pagination parameters
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}

	// Parse enabled filter
	var enabled *bool
	if enabledStr := c.Query("enabled"); enabledStr != "" {
		e := enabledStr == "true"
		enabled = &e
	}

	// Get hooks
	hooks, total, err := messageHookService.ListHooks(page, pageSize, enabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Failed to get hooks: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    hooks,
		"total":   total,
		"page":    page,
		"size":    pageSize,
	})
}

// GetMessageHook retrieves a single message hook by ID
func GetMessageHook(c *gin.Context) {
	// Check admin role
	if !isAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Admin access required",
		})
		return
	}

	// Parse ID
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid hook ID",
		})
		return
	}

	// Get hook
	hook, err := messageHookService.GetHook(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": fmt.Sprintf("Hook not found: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    hook,
	})
}

// CreateMessageHook creates a new message hook
func CreateMessageHook(c *gin.Context) {
	// Check admin role
	if !isAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Admin access required",
		})
		return
	}

	// Parse request body
	var hook model.MessageHook
	if err := c.ShouldBindJSON(&hook); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	// Create hook
	if err := messageHookService.CreateHook(&hook); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Failed to create hook: %v", err),
		})
		return
	}

	// Log operation
	userId := c.GetInt("id")
	common.SysLog(fmt.Sprintf("User %d created message hook: %s (id: %d)", userId, hook.Name, hook.Id))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    hook,
		"message": "Hook created successfully",
	})
}

// UpdateMessageHook updates an existing message hook
func UpdateMessageHook(c *gin.Context) {
	// Check admin role
	if !isAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Admin access required",
		})
		return
	}

	// Parse ID
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid hook ID",
		})
		return
	}

	// Parse request body
	var hook model.MessageHook
	if err := c.ShouldBindJSON(&hook); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	// Set ID from URL
	hook.Id = id

	// Update hook
	if err := messageHookService.UpdateHook(&hook); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Failed to update hook: %v", err),
		})
		return
	}

	// Log operation
	userId := c.GetInt("id")
	common.SysLog(fmt.Sprintf("User %d updated message hook: %s (id: %d)", userId, hook.Name, hook.Id))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    hook,
		"message": "Hook updated successfully",
	})
}

// DeleteMessageHook deletes a message hook
func DeleteMessageHook(c *gin.Context) {
	// Check admin role
	if !isAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Admin access required",
		})
		return
	}

	// Parse ID
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid hook ID",
		})
		return
	}

	// Get hook name for logging
	hook, _ := messageHookService.GetHook(id)
	hookName := ""
	if hook != nil {
		hookName = hook.Name
	}

	// Delete hook
	if err := messageHookService.DeleteHook(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Failed to delete hook: %v", err),
		})
		return
	}

	// Log operation
	userId := c.GetInt("id")
	common.SysLog(fmt.Sprintf("User %d deleted message hook: %s (id: %d)", userId, hookName, id))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Hook deleted successfully",
	})
}

// GetMessageHookStats retrieves statistics for a message hook
func GetMessageHookStats(c *gin.Context) {
	// Check admin role
	if !isAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Admin access required",
		})
		return
	}

	// Parse ID
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid hook ID",
		})
		return
	}

	// Get hook
	hook, err := messageHookService.GetHook(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": fmt.Sprintf("Hook not found: %v", err),
		})
		return
	}

	// Calculate success rate
	successRate := 0.0
	if hook.CallCount > 0 {
		successRate = float64(hook.SuccessCount) / float64(hook.CallCount) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"call_count":    hook.CallCount,
			"success_count": hook.SuccessCount,
			"error_count":   hook.ErrorCount,
			"success_rate":  successRate,
			"avg_duration":  hook.AvgDuration,
		},
	})
}

// TestMessageHook tests a message hook with sample input
func TestMessageHook(c *gin.Context) {
	// Check admin role
	if !isAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Admin access required",
		})
		return
	}

	// Parse request body
	var request struct {
		Hook  model.MessageHook `json:"hook"`
		Input dto.HookInput     `json:"input"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	// Test hook
	output, err := messageHookService.TestHook(&request.Hook, &request.Input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Hook test failed: %v", err),
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    output,
		"message": "Hook test completed successfully",
	})
}

// isAdmin checks if the user has admin role
func isAdmin(c *gin.Context) bool {
	role := c.GetInt("role")
	return role >= common.RoleAdminUser
}
