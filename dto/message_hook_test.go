package dto

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

// Property 11: Input Validation
// Validates: Requirements 19.1, 19.2, 19.3
// This test verifies that HookInput validation works correctly

func TestValidateHookInput(t *testing.T) {
	t.Run("Valid input", func(t *testing.T) {
		input := &HookInput{
			UserId: 1,
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
			Model: "gpt-4",
		}

		err := ValidateHookInput(input)
		if err != nil {
			t.Errorf("Expected valid input, got error: %v", err)
		}
	})

	t.Run("Nil input", func(t *testing.T) {
		err := ValidateHookInput(nil)
		if err == nil {
			t.Error("Expected error for nil input")
		}
	})

	t.Run("Invalid user_id (zero)", func(t *testing.T) {
		input := &HookInput{
			UserId: 0,
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
			Model: "gpt-4",
		}

		err := ValidateHookInput(input)
		if err == nil {
			t.Error("Expected error for user_id = 0")
		}
	})

	t.Run("Invalid user_id (negative)", func(t *testing.T) {
		input := &HookInput{
			UserId: -1,
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
			Model: "gpt-4",
		}

		err := ValidateHookInput(input)
		if err == nil {
			t.Error("Expected error for negative user_id")
		}
	})

	t.Run("Empty messages array", func(t *testing.T) {
		input := &HookInput{
			UserId:   1,
			Messages: []Message{},
			Model:    "gpt-4",
		}

		err := ValidateHookInput(input)
		if err == nil {
			t.Error("Expected error for empty messages array")
		}
	})

	t.Run("Empty model", func(t *testing.T) {
		input := &HookInput{
			UserId: 1,
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
			Model: "",
		}

		err := ValidateHookInput(input)
		if err == nil {
			t.Error("Expected error for empty model")
		}
	})

	t.Run("Invalid message role", func(t *testing.T) {
		input := &HookInput{
			UserId: 1,
			Messages: []Message{
				{Role: "invalid_role", Content: "Hello"},
			},
			Model: "gpt-4",
		}

		err := ValidateHookInput(input)
		if err == nil {
			t.Error("Expected error for invalid message role")
		}
	})

	t.Run("Valid roles", func(t *testing.T) {
		validRoles := []string{"system", "user", "assistant", "tool", "function"}
		for _, role := range validRoles {
			input := &HookInput{
				UserId: 1,
				Messages: []Message{
					{Role: role, Content: "Test content"},
				},
				Model: "gpt-4",
			}

			err := ValidateHookInput(input)
			if err != nil {
				t.Errorf("Expected role %s to be valid, got error: %v", role, err)
			}
		}
	})

	t.Run("Multiple messages", func(t *testing.T) {
		input := &HookInput{
			UserId: 1,
			Messages: []Message{
				{Role: "system", Content: "You are a helpful assistant"},
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
			},
			Model: "gpt-4",
		}

		err := ValidateHookInput(input)
		if err != nil {
			t.Errorf("Expected valid input with multiple messages, got error: %v", err)
		}
	})

	t.Run("Optional fields", func(t *testing.T) {
		input := &HookInput{
			UserId:         1,
			ConversationId: "conv-123",
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
			Model:   "gpt-4",
			TokenId: 100,
		}

		err := ValidateHookInput(input)
		if err != nil {
			t.Errorf("Expected valid input with optional fields, got error: %v", err)
		}
	})
}

// Property 12: Output Validation
// Validates: Requirements 6.1, 6.2, 19.5
// This test verifies that HookOutput validation works correctly

func TestValidateHookOutput(t *testing.T) {
	t.Run("Valid output - not modified", func(t *testing.T) {
		output := &HookOutput{
			Modified: false,
			Abort:    false,
		}

		err := ValidateHookOutput(output)
		if err != nil {
			t.Errorf("Expected valid output, got error: %v", err)
		}
	})

	t.Run("Valid output - modified with messages", func(t *testing.T) {
		output := &HookOutput{
			Modified: true,
			Messages: []Message{
				{Role: "user", Content: "Modified content"},
			},
			Abort: false,
		}

		err := ValidateHookOutput(output)
		if err != nil {
			t.Errorf("Expected valid output, got error: %v", err)
		}
	})

	t.Run("Valid output - abort with reason", func(t *testing.T) {
		output := &HookOutput{
			Modified: false,
			Abort:    true,
			Reason:   "Content policy violation",
		}

		err := ValidateHookOutput(output)
		if err != nil {
			t.Errorf("Expected valid output, got error: %v", err)
		}
	})

	t.Run("Nil output", func(t *testing.T) {
		err := ValidateHookOutput(nil)
		if err == nil {
			t.Error("Expected error for nil output")
		}
	})

	t.Run("Modified true but empty messages", func(t *testing.T) {
		output := &HookOutput{
			Modified: true,
			Messages: []Message{},
			Abort:    false,
		}

		err := ValidateHookOutput(output)
		if err == nil {
			t.Error("Expected error for modified=true with empty messages")
		}
	})

	t.Run("Modified true but nil messages", func(t *testing.T) {
		output := &HookOutput{
			Modified: true,
			Messages: nil,
			Abort:    false,
		}

		err := ValidateHookOutput(output)
		if err == nil {
			t.Error("Expected error for modified=true with nil messages")
		}
	})

	t.Run("Abort true without reason (auto-filled)", func(t *testing.T) {
		output := &HookOutput{
			Modified: false,
			Abort:    true,
			Reason:   "",
		}

		err := ValidateHookOutput(output)
		if err != nil {
			t.Errorf("Expected valid output (reason auto-filled), got error: %v", err)
		}

		// Verify reason was auto-filled
		if output.Reason == "" {
			t.Error("Expected reason to be auto-filled")
		}
	})

	t.Run("Invalid message in output", func(t *testing.T) {
		output := &HookOutput{
			Modified: true,
			Messages: []Message{
				{Role: "invalid_role", Content: "Test"},
			},
			Abort: false,
		}

		err := ValidateHookOutput(output)
		if err == nil {
			t.Error("Expected error for invalid message role")
		}
	})

	t.Run("Multiple valid messages", func(t *testing.T) {
		output := &HookOutput{
			Modified: true,
			Messages: []Message{
				{Role: "system", Content: "System message"},
				{Role: "user", Content: "User message"},
				{Role: "assistant", Content: "Assistant message"},
			},
			Abort: false,
		}

		err := ValidateHookOutput(output)
		if err != nil {
			t.Errorf("Expected valid output with multiple messages, got error: %v", err)
		}
	})
}

func TestValidateMessage(t *testing.T) {
	t.Run("Valid message", func(t *testing.T) {
		msg := &Message{
			Role:    "user",
			Content: "Hello",
		}

		err := validateMessage(msg)
		if err != nil {
			t.Errorf("Expected valid message, got error: %v", err)
		}
	})

	t.Run("Nil message", func(t *testing.T) {
		err := validateMessage(nil)
		if err == nil {
			t.Error("Expected error for nil message")
		}
	})

	t.Run("Empty role", func(t *testing.T) {
		msg := &Message{
			Role:    "",
			Content: "Hello",
		}

		err := validateMessage(msg)
		if err == nil {
			t.Error("Expected error for empty role")
		}
	})

	t.Run("Invalid role", func(t *testing.T) {
		msg := &Message{
			Role:    "invalid",
			Content: "Hello",
		}

		err := validateMessage(msg)
		if err == nil {
			t.Error("Expected error for invalid role")
		}
	})

	t.Run("All valid roles", func(t *testing.T) {
		validRoles := []string{"system", "user", "assistant", "tool", "function"}
		for _, role := range validRoles {
			msg := &Message{
				Role:    role,
				Content: "Test content",
			}

			err := validateMessage(msg)
			if err != nil {
				t.Errorf("Expected role %s to be valid, got error: %v", role, err)
			}
		}
	})
}

// Property-based testing: Generate random valid inputs
func TestPropertyBasedValidation(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	t.Run("Random valid inputs should pass validation", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			input := generateRandomValidHookInput()
			err := ValidateHookInput(input)
			if err != nil {
				t.Errorf("Iteration %d: Expected valid input, got error: %v", i, err)
			}
		}
	})

	t.Run("Random valid outputs should pass validation", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			output := generateRandomValidHookOutput()
			err := ValidateHookOutput(output)
			if err != nil {
				t.Errorf("Iteration %d: Expected valid output, got error: %v", i, err)
			}
		}
	})

	t.Run("Invalid user_id should always fail", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			input := generateRandomValidHookInput()
			input.UserId = rand.Intn(2) - 1 // 0 or -1
			err := ValidateHookInput(input)
			if err == nil {
				t.Errorf("Iteration %d: Expected error for invalid user_id %d", i, input.UserId)
			}
		}
	})

	t.Run("Empty messages should always fail", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			input := generateRandomValidHookInput()
			input.Messages = []Message{}
			err := ValidateHookInput(input)
			if err == nil {
				t.Error("Expected error for empty messages")
			}
		}
	})

	t.Run("Modified=true with empty messages should always fail", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			output := &HookOutput{
				Modified: true,
				Messages: []Message{},
				Abort:    false,
			}
			err := ValidateHookOutput(output)
			if err == nil {
				t.Error("Expected error for modified=true with empty messages")
			}
		}
	})
}

// Helper functions for property-based testing

func generateRandomValidHookInput() *HookInput {
	roles := []string{"system", "user", "assistant", "tool", "function"}
	numMessages := rand.Intn(5) + 1 // 1-5 messages

	messages := make([]Message, numMessages)
	for i := 0; i < numMessages; i++ {
		messages[i] = Message{
			Role:    roles[rand.Intn(len(roles))],
			Content: fmt.Sprintf("Message content %d", rand.Int()),
		}
	}

	return &HookInput{
		UserId:         rand.Intn(1000) + 1, // 1-1000
		ConversationId: fmt.Sprintf("conv-%d", rand.Int()),
		Messages:       messages,
		Model:          fmt.Sprintf("model-%d", rand.Intn(10)),
		TokenId:        rand.Intn(1000),
	}
}

func generateRandomValidHookOutput() *HookOutput {
	modified := rand.Intn(2) == 1
	abort := rand.Intn(2) == 1

	output := &HookOutput{
		Modified: modified,
		Abort:    abort,
	}

	if modified {
		roles := []string{"system", "user", "assistant", "tool", "function"}
		numMessages := rand.Intn(5) + 1

		messages := make([]Message, numMessages)
		for i := 0; i < numMessages; i++ {
			messages[i] = Message{
				Role:    roles[rand.Intn(len(roles))],
				Content: fmt.Sprintf("Modified content %d", rand.Int()),
			}
		}
		output.Messages = messages
	}

	if abort {
		output.Reason = fmt.Sprintf("Abort reason %d", rand.Int())
	}

	return output
}
