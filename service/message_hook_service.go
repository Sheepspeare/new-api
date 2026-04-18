package service

import (
	"errors"
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"strings"
	"time"
)

// MessageHookService interface for hook management and execution
type MessageHookService interface {
	// CRUD operations
	CreateHook(hook *model.MessageHook) error
	UpdateHook(hook *model.MessageHook) error
	DeleteHook(id int) error
	GetHook(id int) (*model.MessageHook, error)
	ListHooks(page, pageSize int, enabled *bool) ([]*model.MessageHook, int64, error)

	// Execution
	GetEnabledHooks() ([]*model.MessageHook, error)
	ExecuteHooks(hooks []*model.MessageHook, input *dto.HookInput) (*dto.HookOutput, error)

	// Statistics
	UpdateHookStats(hookId int, success bool, duration time.Duration)

	// Testing
	TestHook(hook *model.MessageHook, input *dto.HookInput) (*dto.HookOutput, error)

	// Cache management
	InvalidateCache() error
}

// messageHookService implements MessageHookService
type messageHookService struct {
	luaExecutor  LuaExecutor
	httpExecutor HTTPExecutor
}

// NewMessageHookService creates a new message hook service
func NewMessageHookService() MessageHookService {
	return &messageHookService{
		luaExecutor:  NewLuaExecutor(),
		httpExecutor: NewHTTPExecutor(),
	}
}

// CreateHook creates a new message hook
func (s *messageHookService) CreateHook(hook *model.MessageHook) error {
	if hook == nil {
		return errors.New("hook is nil")
	}

	// Validate hook
	if err := s.validateHook(hook); err != nil {
		return err
	}

	// Create hook in database
	if err := model.CreateMessageHook(hook); err != nil {
		return err
	}

	// Invalidate cache
	_ = s.InvalidateCache()

	return nil
}

// UpdateHook updates an existing message hook
func (s *messageHookService) UpdateHook(hook *model.MessageHook) error {
	if hook == nil {
		return errors.New("hook is nil")
	}

	if hook.Id == 0 {
		return errors.New("hook id is required")
	}

	// Validate hook
	if err := s.validateHook(hook); err != nil {
		return err
	}

	// Update hook in database
	if err := model.UpdateMessageHook(hook); err != nil {
		return err
	}

	// Invalidate cache
	_ = s.InvalidateCache()

	return nil
}

// DeleteHook deletes a message hook
func (s *messageHookService) DeleteHook(id int) error {
	if err := model.DeleteMessageHook(id); err != nil {
		return err
	}

	// Invalidate cache
	_ = s.InvalidateCache()

	return nil
}

// GetHook retrieves a single message hook
func (s *messageHookService) GetHook(id int) (*model.MessageHook, error) {
	return model.GetMessageHook(id)
}

// ListHooks retrieves all message hooks with pagination
func (s *messageHookService) ListHooks(page, pageSize int, enabled *bool) ([]*model.MessageHook, int64, error) {
	return model.GetAllMessageHooks(page, pageSize, enabled)
}

// GetEnabledHooks retrieves all enabled hooks sorted by priority
func (s *messageHookService) GetEnabledHooks() ([]*model.MessageHook, error) {
	// Try to get from cache first
	if common.RedisEnabled {
		cachedData, err := common.RedisGet(constant.EnabledHooksCacheKey)
		if err == nil && cachedData != "" {
			var hooks []*model.MessageHook
			if err := common.Unmarshal([]byte(cachedData), &hooks); err == nil {
				return hooks, nil
			}
			// If unmarshal fails, continue to fetch from database
			common.SysError(fmt.Sprintf("Failed to unmarshal cached hooks: %v", err))
		}
	}

	// Fetch from database
	hooks, err := model.GetEnabledMessageHooks()
	if err != nil {
		return nil, err
	}

	// Cache the result
	if common.RedisEnabled && len(hooks) > 0 {
		data, err := common.Marshal(hooks)
		if err == nil {
			ttl := time.Duration(common.MessageHookCacheTTL) * time.Second
			_ = common.RedisSet(constant.EnabledHooksCacheKey, string(data), ttl)
		}
	}

	return hooks, nil
}

// ExecuteHooks executes all hooks in priority order
func (s *messageHookService) ExecuteHooks(hooks []*model.MessageHook, input *dto.HookInput) (*dto.HookOutput, error) {
	if len(hooks) == 0 {
		common.SysLog("[MESSAGE_HOOK_SERVICE] No hooks to execute")
		return &dto.HookOutput{
			Modified: false,
			Messages: input.Messages,
			Abort:    false,
		}, nil
	}

	common.SysLog(fmt.Sprintf("[MESSAGE_HOOK_SERVICE] Starting execution of %d hooks for userId=%d", len(hooks), input.UserId))

	// Validate input
	if err := dto.ValidateHookInput(input); err != nil {
		common.SysLog(fmt.Sprintf("[MESSAGE_HOOK_SERVICE] ❌ Input validation failed: %v", err))
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Current messages (will be updated by each hook)
	currentMessages := input.Messages
	anyModified := false

	// Execute hooks sequentially
	for i, hook := range hooks {
		common.SysLog(fmt.Sprintf("[MESSAGE_HOOK_SERVICE] Processing hook %d/%d: id=%d, name=%s, type=%d", 
			i+1, len(hooks), hook.Id, hook.Name, hook.Type))

		// Check if hook matches filters
		if !s.matchesFilters(hook, input) {
			common.SysLog(fmt.Sprintf("[MESSAGE_HOOK_SERVICE] ⏭️ Hook %d skipped (filters not matched)", hook.Id))
			continue
		}

		common.SysLog(fmt.Sprintf("[MESSAGE_HOOK_SERVICE] ✅ Hook %d matches filters, executing...", hook.Id))

		// Update input with current messages
		input.Messages = currentMessages

		// Execute hook with timeout
		startTime := time.Now()
		common.SysLog(fmt.Sprintf("[MESSAGE_HOOK_SERVICE] ⏱️ Executing hook %d (%s) at %s", 
			hook.Id, hook.Name, startTime.Format("2006-01-02 15:04:05")))
		
		output, err := s.executeHook(hook, input)
		duration := time.Since(startTime)

		// Update statistics asynchronously
		go s.UpdateHookStats(hook.Id, err == nil && !output.Abort, duration)

		// Handle execution error
		if err != nil {
			common.SysError(fmt.Sprintf("[MESSAGE_HOOK_SERVICE] ❌ Hook %d (%s) execution failed after %v: %v", 
				hook.Id, hook.Name, duration, err))
			// Continue to next hook (graceful degradation)
			continue
		}

		common.SysLog(fmt.Sprintf("[MESSAGE_HOOK_SERVICE] ✅ Hook %d (%s) executed successfully in %v", 
			hook.Id, hook.Name, duration))

		// Handle abort
		if output.Abort {
			common.SysLog(fmt.Sprintf("[MESSAGE_HOOK_SERVICE] 🛑 Hook %d (%s) aborted request: %s", 
				hook.Id, hook.Name, output.Reason))
			return output, nil
		}

		// Handle modification
		if output.Modified && len(output.Messages) > 0 {
			anyModified = true
			currentMessages = output.Messages
			common.SysLog(fmt.Sprintf("[MESSAGE_HOOK_SERVICE] 📝 Hook %d (%s) modified messages: %d → %d", 
				hook.Id, hook.Name, len(input.Messages), len(output.Messages)))
		} else {
			common.SysLog(fmt.Sprintf("[MESSAGE_HOOK_SERVICE] ⏭️ Hook %d (%s) made no modifications", 
				hook.Id, hook.Name))
		}
	}

	common.SysLog(fmt.Sprintf("[MESSAGE_HOOK_SERVICE] ✅ All hooks completed. anyModified=%v, finalMessageCount=%d", 
		anyModified, len(currentMessages)))

	// Return final result
	finalModified := anyModified || len(currentMessages) != len(input.Messages) || !messagesEqual(currentMessages, input.Messages)
	common.SysLog(fmt.Sprintf("[MESSAGE_HOOK_SERVICE] 📊 Final result: modified=%v, messageCount=%d", 
		finalModified, len(currentMessages)))
	
	return &dto.HookOutput{
		Modified: finalModified,
		Messages: currentMessages,
		Abort:    false,
	}, nil
}

// executeHook executes a single hook
func (s *messageHookService) executeHook(hook *model.MessageHook, input *dto.HookInput) (*dto.HookOutput, error) {
	timeout := time.Duration(hook.Timeout) * time.Millisecond

	switch hook.Type {
	case constant.MessageHookTypeLua:
		return s.luaExecutor.Execute(hook.Content, input, timeout)
	case constant.MessageHookTypeHTTP:
		return s.httpExecutor.Execute(hook.Content, input, timeout)
	default:
		return nil, fmt.Errorf("unknown hook type: %d", hook.Type)
	}
}

// TestHook tests a hook without updating statistics
func (s *messageHookService) TestHook(hook *model.MessageHook, input *dto.HookInput) (*dto.HookOutput, error) {
	if hook == nil {
		return nil, errors.New("hook is nil")
	}

	// Validate hook
	if err := s.validateHook(hook); err != nil {
		return nil, err
	}

	// Validate input
	if err := dto.ValidateHookInput(input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Execute hook
	return s.executeHook(hook, input)
}

// UpdateHookStats updates hook statistics asynchronously
func (s *messageHookService) UpdateHookStats(hookId int, success bool, duration time.Duration) {
	if err := model.UpdateMessageHookStats(hookId, success, duration); err != nil {
		common.SysError(fmt.Sprintf("Failed to update hook stats: %v", err))
	}
}

// validateHook validates a hook configuration
func (s *messageHookService) validateHook(hook *model.MessageHook) error {
	// Validate name
	if strings.TrimSpace(hook.Name) == "" {
		return errors.New("hook name cannot be empty")
	}

	// Validate type
	if hook.Type != constant.MessageHookTypeLua && hook.Type != constant.MessageHookTypeHTTP {
		return fmt.Errorf("invalid hook type: %d (must be 1=Lua or 2=HTTP)", hook.Type)
	}

	// Validate content
	if strings.TrimSpace(hook.Content) == "" {
		return errors.New("hook content cannot be empty")
	}

	// Validate Lua script size
	if hook.Type == constant.MessageHookTypeLua {
		if len(hook.Content) > 1024*1024 { // 1MB
			return errors.New("Lua script size cannot exceed 1MB")
		}
	}

	// Validate HTTP URL
	if hook.Type == constant.MessageHookTypeHTTP {
		if err := ValidateHTTPHookURL(hook.Content); err != nil {
			return fmt.Errorf("invalid HTTP URL: %w", err)
		}
	}

	// Validate timeout
	if hook.Timeout < 100 || hook.Timeout > 30000 {
		return fmt.Errorf("timeout must be between 100ms and 30000ms, got %dms", hook.Timeout)
	}

	// Validate filter JSON format
	if hook.FilterUsers != "" {
		var users []int
		if err := common.Unmarshal([]byte(hook.FilterUsers), &users); err != nil {
			return fmt.Errorf("invalid filter_users JSON: %w", err)
		}
	}

	if hook.FilterModels != "" {
		var models []string
		if err := common.Unmarshal([]byte(hook.FilterModels), &models); err != nil {
			return fmt.Errorf("invalid filter_models JSON: %w", err)
		}
	}

	if hook.FilterTokens != "" {
		var tokens []int
		if err := common.Unmarshal([]byte(hook.FilterTokens), &tokens); err != nil {
			return fmt.Errorf("invalid filter_tokens JSON: %w", err)
		}
	}

	return nil
}

// matchesFilters checks if a hook matches the input filters
func (s *messageHookService) matchesFilters(hook *model.MessageHook, input *dto.HookInput) bool {
	// Check user filter
	if hook.FilterUsers != "" {
		var users []int
		if err := common.Unmarshal([]byte(hook.FilterUsers), &users); err != nil {
			common.SysError(fmt.Sprintf("Failed to parse filter_users for hook %s: %v", hook.Name, err))
			return false
		}
		if !contains(users, input.UserId) {
			return false
		}
	}

	// Check model filter
	if hook.FilterModels != "" {
		var models []string
		if err := common.Unmarshal([]byte(hook.FilterModels), &models); err != nil {
			common.SysError(fmt.Sprintf("Failed to parse filter_models for hook %s: %v", hook.Name, err))
			return false
		}
		if !containsString(models, input.Model) {
			return false
		}
	}

	// Check token filter
	if hook.FilterTokens != "" {
		var tokens []int
		if err := common.Unmarshal([]byte(hook.FilterTokens), &tokens); err != nil {
			common.SysError(fmt.Sprintf("Failed to parse filter_tokens for hook %s: %v", hook.Name, err))
			return false
		}
		if !contains(tokens, input.TokenId) {
			return false
		}
	}

	return true
}

// Helper functions

func contains(slice []int, item int) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

func containsString(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

func messagesEqual(a, b []dto.Message) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Role != b[i].Role || a[i].StringContent() != b[i].StringContent() {
			return false
		}
	}
	return true
}

// InvalidateCache invalidates the enabled hooks cache
func (s *messageHookService) InvalidateCache() error {
	// Delete from Redis if available
	if common.RedisEnabled {
		if err := common.RedisDel(constant.EnabledHooksCacheKey); err != nil {
			common.SysError(fmt.Sprintf("Failed to invalidate Redis cache: %v", err))
			// Don't return error, continue to clear in-memory cache
		}
	}

	// Note: In-memory cache invalidation would be handled by the caching layer
	// For now, we just clear Redis cache and rely on TTL for in-memory cache

	return nil
}
