package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

var (
	messageHookService service.MessageHookService
	hookServiceInit    bool
)

// initMessageHookService initializes the message hook service (lazy initialization)
func initMessageHookService() {
	if !hookServiceInit {
		messageHookService = service.NewMessageHookService()
		hookServiceInit = true
	}
}

// MessageHook middleware processes messages through configured hooks before reaching LLM
func MessageHook() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Initialize service if needed
		initMessageHookService()

		// Log: Request received
		common.SysLog(fmt.Sprintf("[MESSAGE_HOOK] Request received: %s %s", c.Request.Method, c.Request.URL.Path))

		// Only process chat completion requests
		if !isChatCompletionRequest(c) {
			common.SysLog(fmt.Sprintf("[MESSAGE_HOOK] Skipping non-chat request: %s", c.Request.URL.Path))
			c.Next()
			return
		}

		common.SysLog("[MESSAGE_HOOK] Chat completion request detected")

		// Extract context data
		userId := c.GetInt("id")
		if userId == 0 {
			// User not authenticated, skip hooks
			common.SysLog("[MESSAGE_HOOK] User not authenticated (userId=0), skipping hooks")
			c.Next()
			return
		}

		common.SysLog(fmt.Sprintf("[MESSAGE_HOOK] User authenticated: userId=%d", userId))

		// Parse request body first to get model
		var request dto.GeneralOpenAIRequest
		err := common.UnmarshalBodyReusable(c, &request)
		if err != nil || len(request.Messages) == 0 {
			// Invalid request or no messages, skip hooks
			common.SysLog(fmt.Sprintf("[MESSAGE_HOOK] Failed to parse request or no messages: err=%v, messageCount=%d", err, len(request.Messages)))
			c.Next()
			return
		}

		common.SysLog(fmt.Sprintf("[MESSAGE_HOOK] Request parsed successfully: %d messages", len(request.Messages)))

		// Get model from request body (not from context, as Distribute hasn't run yet)
		tokenId := c.GetInt(string(constant.ContextKeyTokenId))
		modelName := request.Model
		if modelName == "" {
			common.SysLog("[MESSAGE_HOOK] ⚠️ Model name is empty in request, skipping hooks")
			c.Next()
			return
		}

		common.SysLog(fmt.Sprintf("[MESSAGE_HOOK] Request info: userId=%d, tokenId=%d, model=%s", userId, tokenId, modelName))

		// Extract conversation ID if available
		conversationId := c.GetString(constant.ContextKeyConversationId)

		// Build hook input
		input := &dto.HookInput{
			UserId:         userId,
			ConversationId: conversationId,
			Messages:       request.Messages,
			Model:          modelName,
			TokenId:        tokenId,
		}

		// Get enabled hooks
		common.SysLog("[MESSAGE_HOOK] Querying enabled hooks from database...")
		hooks, err := messageHookService.GetEnabledHooks()
		if err != nil {
			common.SysLog(fmt.Sprintf("[MESSAGE_HOOK] ❌ Failed to get enabled hooks: %v", err))
			c.Next()
			return
		}

		if len(hooks) == 0 {
			common.SysLog("[MESSAGE_HOOK] ⚠️ No enabled hooks found in database")
			// No hooks configured
			c.Next()
			return
		}

		common.SysLog(fmt.Sprintf("[MESSAGE_HOOK] ✅ Found %d enabled hooks", len(hooks)))
		for i, hook := range hooks {
			common.SysLog(fmt.Sprintf("[MESSAGE_HOOK]   Hook #%d: id=%d, name=%s, type=%d, priority=%d, enabled=%v", 
				i+1, hook.Id, hook.Name, hook.Type, hook.Priority, hook.Enabled))
		}

		// Execute hooks
		common.SysLog(fmt.Sprintf("[MESSAGE_HOOK] 🚀 Executing %d hooks for userId=%d...", len(hooks), userId))
		output, err := messageHookService.ExecuteHooks(hooks, input)
		if err != nil {
			common.SysLog(fmt.Sprintf("[MESSAGE_HOOK] ❌ Hook execution failed: %v", err))
			// Continue with original request (graceful degradation)
			c.Next()
			return
		}

		common.SysLog(fmt.Sprintf("[MESSAGE_HOOK] ✅ Hooks executed: modified=%v, abort=%v, messageCount=%d", 
			output.Modified, output.Abort, len(output.Messages)))

		// Handle abort
		if output.Abort {
			reason := output.Reason
			if reason == "" {
				reason = "Request aborted by message hook"
			}
			common.SysLog(fmt.Sprintf("[MESSAGE_HOOK] 🛑 Request aborted by hook: reason=%s", reason))
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": reason,
					"type":    "hook_abort",
					"code":    "hook_abort",
				},
			})
			c.Abort()
			return
		}

		// Handle modification
		if output.Modified && len(output.Messages) > 0 {
			common.SysLog(fmt.Sprintf("[MESSAGE_HOOK] 📝 Messages modified by hook: original=%d, modified=%d", 
				len(request.Messages), len(output.Messages)))
			
			// Update request with modified messages
			request.Messages = output.Messages

			// Re-marshal the modified request
			modifiedBody, err := common.Marshal(request)
			if err != nil {
				common.SysLog(fmt.Sprintf("[MESSAGE_HOOK] ❌ Failed to marshal modified request: %v", err))
				c.Next()
				return
			}

			common.SysLog(fmt.Sprintf("[MESSAGE_HOOK] Modified body size: %d bytes", len(modifiedBody)))

			// Replace the BodyStorage so downstream relay reads modified body
			newStorage, storageErr := common.CreateBodyStorage(modifiedBody)
			if storageErr != nil {
				common.SysLog(fmt.Sprintf("[MESSAGE_HOOK] ❌ Failed to create body storage: %v", storageErr))
				c.Next()
				return
			}
			c.Set(common.KeyBodyStorage, newStorage)
			c.Request.ContentLength = int64(len(modifiedBody))
			c.Request.Body = io.NopCloser(bytes.NewReader(modifiedBody))
			
			common.SysLog(fmt.Sprintf("[MESSAGE_HOOK] ✅ Request body replaced successfully for userId=%d", userId))
		} else {
			common.SysLog("[MESSAGE_HOOK] No modifications made to request")
		}

		common.SysLog(fmt.Sprintf("[MESSAGE_HOOK] ✅ Middleware completed for userId=%d, continuing to relay...", userId))
		c.Next()
	}
}

// isChatCompletionRequest checks if the request is a chat completion request
func isChatCompletionRequest(c *gin.Context) bool {
	path := c.Request.URL.Path
	// Check for chat completion endpoints
	return path == "/v1/chat/completions" ||
		path == "/v1beta/chat/completions" ||
		path == "/pg/chat/completions"
}
