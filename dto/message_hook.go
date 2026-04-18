package dto

import (
	"errors"
	"fmt"
)

// HookInput represents the input data passed to message hooks
type HookInput struct {
	UserId         int       `json:"user_id"`
	ConversationId string    `json:"conversation_id,omitempty"`
	Messages       []Message `json:"messages"`
	Model          string    `json:"model"`
	TokenId        int       `json:"token_id,omitempty"`
}

// HookOutput represents the output data returned by message hooks
type HookOutput struct {
	Modified bool      `json:"modified"`
	Messages []Message `json:"messages,omitempty"`
	Abort    bool      `json:"abort"`
	Reason   string    `json:"reason,omitempty"`
}

// ValidateHookInput validates the HookInput structure
func ValidateHookInput(input *HookInput) error {
	if input == nil {
		return errors.New("input is nil")
	}

	if input.UserId <= 0 {
		return errors.New("user_id must be greater than 0")
	}

	if len(input.Messages) == 0 {
		return errors.New("messages array cannot be empty")
	}

	if input.Model == "" {
		return errors.New("model cannot be empty")
	}

	// Validate each message structure
	for i, msg := range input.Messages {
		if err := validateMessage(&msg); err != nil {
			return fmt.Errorf("invalid message at index %d: %w", i, err)
		}
	}

	return nil
}

// ValidateHookOutput validates the HookOutput structure
func ValidateHookOutput(output *HookOutput) error {
	if output == nil {
		return errors.New("output is nil")
	}

	// If modified is true, messages must be provided and non-empty
	if output.Modified {
		if len(output.Messages) == 0 {
			return errors.New("modified is true but messages array is empty")
		}

		// Validate each message structure
		for i, msg := range output.Messages {
			if err := validateMessage(&msg); err != nil {
				return fmt.Errorf("invalid message at index %d: %w", i, err)
			}
		}
	}

	// If abort is true, reason should be provided (warning, not error)
	if output.Abort && output.Reason == "" {
		// This is acceptable but not ideal
		output.Reason = "Request aborted by hook"
	}

	return nil
}

// validateMessage validates a single message structure
func validateMessage(msg *Message) error {
	if msg == nil {
		return errors.New("message is nil")
	}

	// Role must be non-empty
	if msg.Role == "" {
		return errors.New("message role cannot be empty")
	}

	// Role must be one of the valid values
	validRoles := map[string]bool{
		"system":    true,
		"user":      true,
		"assistant": true,
		"tool":      true,
		"function":  true, // Legacy support
	}

	if !validRoles[msg.Role] {
		return fmt.Errorf("invalid message role: %s (must be one of: system, user, assistant, tool, function)", msg.Role)
	}

	// Content validation depends on role
	// For most roles, content should be present
	// Tool role may have tool_calls instead of content
	if msg.Role != "tool" && msg.Role != "assistant" {
		contentSlice, isSlice := msg.Content.([]interface{})
		if msg.StringContent() == "" && (!isSlice || len(contentSlice) == 0) {
			return fmt.Errorf("message content cannot be empty for role: %s", msg.Role)
		}
	}

	return nil
}
