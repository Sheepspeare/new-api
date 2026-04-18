package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
)

// Property 8: Sandbox Isolation (Lua)
// Validates: Requirements 3.7, 9.1, 9.2, 9.3, 9.4, 9.5
// This test verifies that Lua sandbox prevents dangerous operations

func TestLuaSandboxIsolation(t *testing.T) {
	executor := NewLuaExecutor()
	input := &dto.HookInput{
		UserId: 1,
		Messages: []dto.Message{
			{Role: "user", Content: "Test"},
		},
		Model: "gpt-4",
	}

	t.Run("os module is disabled", func(t *testing.T) {
		script := `
			if os then
				error("os module should be disabled")
			end
			output = {modified = false, abort = false}
		`
		_, err := executor.Execute(script, input, 1*time.Second)
		if err != nil {
			t.Errorf("Expected script to run (os should be nil), got error: %v", err)
		}
	})

	t.Run("io module is disabled", func(t *testing.T) {
		script := `
			if io then
				error("io module should be disabled")
			end
			output = {modified = false, abort = false}
		`
		_, err := executor.Execute(script, input, 1*time.Second)
		if err != nil {
			t.Errorf("Expected script to run (io should be nil), got error: %v", err)
		}
	})

	t.Run("package module is disabled", func(t *testing.T) {
		script := `
			if package then
				error("package module should be disabled")
			end
			output = {modified = false, abort = false}
		`
		_, err := executor.Execute(script, input, 1*time.Second)
		if err != nil {
			t.Errorf("Expected script to run (package should be nil), got error: %v", err)
		}
	})

	t.Run("debug module is disabled", func(t *testing.T) {
		script := `
			if debug then
				error("debug module should be disabled")
			end
			output = {modified = false, abort = false}
		`
		_, err := executor.Execute(script, input, 1*time.Second)
		if err != nil {
			t.Errorf("Expected script to run (debug should be nil), got error: %v", err)
		}
	})

	t.Run("dofile is disabled", func(t *testing.T) {
		script := `
			if dofile then
				error("dofile should be disabled")
			end
			output = {modified = false, abort = false}
		`
		_, err := executor.Execute(script, input, 1*time.Second)
		if err != nil {
			t.Errorf("Expected script to run (dofile should be nil), got error: %v", err)
		}
	})

	t.Run("loadfile is disabled", func(t *testing.T) {
		script := `
			if loadfile then
				error("loadfile should be disabled")
			end
			output = {modified = false, abort = false}
		`
		_, err := executor.Execute(script, input, 1*time.Second)
		if err != nil {
			t.Errorf("Expected script to run (loadfile should be nil), got error: %v", err)
		}
	})

	t.Run("require is disabled", func(t *testing.T) {
		script := `
			if require then
				error("require should be disabled")
			end
			output = {modified = false, abort = false}
		`
		_, err := executor.Execute(script, input, 1*time.Second)
		if err != nil {
			t.Errorf("Expected script to run (require should be nil), got error: %v", err)
		}
	})

	t.Run("load is disabled", func(t *testing.T) {
		script := `
			if load then
				error("load should be disabled")
			end
			output = {modified = false, abort = false}
		`
		_, err := executor.Execute(script, input, 1*time.Second)
		if err != nil {
			t.Errorf("Expected script to run (load should be nil), got error: %v", err)
		}
	})

	t.Run("loadstring is disabled", func(t *testing.T) {
		script := `
			if loadstring then
				error("loadstring should be disabled")
			end
			output = {modified = false, abort = false}
		`
		_, err := executor.Execute(script, input, 1*time.Second)
		if err != nil {
			t.Errorf("Expected script to run (loadstring should be nil), got error: %v", err)
		}
	})

	t.Run("safe modules are available", func(t *testing.T) {
		script := `
			-- Test string module
			local s = string.upper("hello")
			if s ~= "HELLO" then
				error("string module not working")
			end
			
			-- Test table module
			local t = {1, 2, 3}
			table.insert(t, 4)
			if #t ~= 4 then
				error("table module not working")
			end
			
			-- Test math module
			local m = math.abs(-5)
			if m ~= 5 then
				error("math module not working")
			end
			
			output = {modified = false, abort = false}
		`
		_, err := executor.Execute(script, input, 1*time.Second)
		if err != nil {
			t.Errorf("Expected safe modules to work, got error: %v", err)
		}
	})
}

// Property 16: Lua Input Injection
// Property 17: Lua Output Extraction
// Validates: Requirements 3.2, 3.4, 3.5

func TestLuaInputOutputConversion(t *testing.T) {
	executor := NewLuaExecutor()

	t.Run("Input is correctly injected", func(t *testing.T) {
		input := &dto.HookInput{
			UserId:         123,
			ConversationId: "conv-456",
			Messages: []dto.Message{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there"},
			},
			Model:   "gpt-4",
			TokenId: 789,
		}

		script := `
			-- Verify input fields
			if input.user_id ~= 123 then
				error("user_id not injected correctly")
			end
			if input.conversation_id ~= "conv-456" then
				error("conversation_id not injected correctly")
			end
			if input.model ~= "gpt-4" then
				error("model not injected correctly")
			end
			if input.token_id ~= 789 then
				error("token_id not injected correctly")
			end
			if #input.messages ~= 2 then
				error("messages not injected correctly")
			end
			if input.messages[1].role ~= "user" then
				error("message role not injected correctly")
			end
			if input.messages[1].content ~= "Hello" then
				error("message content not injected correctly")
			end
			
			output = {modified = false, abort = false}
		`

		_, err := executor.Execute(script, input, 1*time.Second)
		if err != nil {
			t.Errorf("Input injection failed: %v", err)
		}
	})

	t.Run("Output is correctly extracted", func(t *testing.T) {
		input := &dto.HookInput{
			UserId: 1,
			Messages: []dto.Message{
				{Role: "user", Content: "Test"},
			},
			Model: "gpt-4",
		}

		script := `
			output = {
				modified = true,
				messages = {
					{role = "user", content = "Modified content"}
				},
				abort = false
			}
		`

		result, err := executor.Execute(script, input, 1*time.Second)
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		if !result.Modified {
			t.Error("Expected modified to be true")
		}
		if len(result.Messages) != 1 {
			t.Errorf("Expected 1 message, got %d", len(result.Messages))
		}
		if result.Messages[0].Content != "Modified content" {
			t.Errorf("Expected 'Modified content', got '%s'", result.Messages[0].Content)
		}
		if result.Abort {
			t.Error("Expected abort to be false")
		}
	})

	t.Run("Abort output is correctly extracted", func(t *testing.T) {
		input := &dto.HookInput{
			UserId: 1,
			Messages: []dto.Message{
				{Role: "user", Content: "Test"},
			},
			Model: "gpt-4",
		}

		script := `
			output = {
				modified = false,
				abort = true,
				reason = "Content policy violation"
			}
		`

		result, err := executor.Execute(script, input, 1*time.Second)
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		if !result.Abort {
			t.Error("Expected abort to be true")
		}
		if result.Reason != "Content policy violation" {
			t.Errorf("Expected reason 'Content policy violation', got '%s'", result.Reason)
		}
	})

	t.Run("Missing output variable returns error", func(t *testing.T) {
		input := &dto.HookInput{
			UserId: 1,
			Messages: []dto.Message{
				{Role: "user", Content: "Test"},
			},
			Model: "gpt-4",
		}

		script := `
			-- Script doesn't set output
			local x = 1 + 1
		`

		_, err := executor.Execute(script, input, 1*time.Second)
		if err == nil {
			t.Error("Expected error for missing output variable")
		}
		if !strings.Contains(err.Error(), "output") {
			t.Errorf("Expected error about output variable, got: %v", err)
		}
	})
}

// Property 3: Timeout Enforcement
// Validates: Requirements 3.3, 3.6

func TestLuaTimeoutEnforcement(t *testing.T) {
	executor := NewLuaExecutor()
	input := &dto.HookInput{
		UserId: 1,
		Messages: []dto.Message{
			{Role: "user", Content: "Test"},
		},
		Model: "gpt-4",
	}

	t.Run("Script completes within timeout", func(t *testing.T) {
		script := `
			output = {modified = false, abort = false}
		`

		start := time.Now()
		_, err := executor.Execute(script, input, 1*time.Second)
		duration := time.Since(start)

		if err != nil {
			t.Errorf("Expected script to complete, got error: %v", err)
		}
		if duration > 1*time.Second {
			t.Errorf("Script took too long: %v", duration)
		}
	})

	t.Run("Long-running script is terminated", func(t *testing.T) {
		script := `
			-- Infinite loop
			while true do
				local x = 1 + 1
			end
			output = {modified = false, abort = false}
		`

		start := time.Now()
		_, err := executor.Execute(script, input, 100*time.Millisecond)
		duration := time.Since(start)

		if err == nil {
			t.Error("Expected timeout error for infinite loop")
		}

		// Should timeout around 100ms, not run forever
		if duration > 500*time.Millisecond {
			t.Errorf("Timeout took too long: %v", duration)
		}
	})

	t.Run("Timeout error message is clear", func(t *testing.T) {
		script := `
			while true do end
		`

		_, err := executor.Execute(script, input, 50*time.Millisecond)
		if err == nil {
			t.Error("Expected timeout error")
		}

		errMsg := err.Error()
		if !strings.Contains(strings.ToLower(errMsg), "timeout") {
			t.Errorf("Expected timeout in error message, got: %v", errMsg)
		}
	})
}

// Unit tests for Lua executor

func TestLuaExecutorBasicFunctionality(t *testing.T) {
	executor := NewLuaExecutor()

	t.Run("Empty script returns error", func(t *testing.T) {
		input := &dto.HookInput{
			UserId: 1,
			Messages: []dto.Message{
				{Role: "user", Content: "Test"},
			},
			Model: "gpt-4",
		}

		_, err := executor.Execute("", input, 1*time.Second)
		if err == nil {
			t.Error("Expected error for empty script")
		}
	})

	t.Run("Nil input returns error", func(t *testing.T) {
		script := `output = {modified = false, abort = false}`
		_, err := executor.Execute(script, nil, 1*time.Second)
		if err == nil {
			t.Error("Expected error for nil input")
		}
	})

	t.Run("Invalid input returns error", func(t *testing.T) {
		script := `output = {modified = false, abort = false}`
		input := &dto.HookInput{
			UserId:   0, // Invalid
			Messages: []dto.Message{{Role: "user", Content: "Test"}},
			Model:    "gpt-4",
		}

		_, err := executor.Execute(script, input, 1*time.Second)
		if err == nil {
			t.Error("Expected error for invalid input")
		}
	})

	t.Run("Syntax error in script", func(t *testing.T) {
		input := &dto.HookInput{
			UserId: 1,
			Messages: []dto.Message{
				{Role: "user", Content: "Test"},
			},
			Model: "gpt-4",
		}

		script := `
			this is not valid lua syntax !!!
		`

		_, err := executor.Execute(script, input, 1*time.Second)
		if err == nil {
			t.Error("Expected error for syntax error")
		}
	})

	t.Run("Runtime error in script", func(t *testing.T) {
		input := &dto.HookInput{
			UserId: 1,
			Messages: []dto.Message{
				{Role: "user", Content: "Test"},
			},
			Model: "gpt-4",
		}

		script := `
			error("Intentional runtime error")
		`

		_, err := executor.Execute(script, input, 1*time.Second)
		if err == nil {
			t.Error("Expected error for runtime error")
		}
	})

	t.Run("Invalid output structure", func(t *testing.T) {
		input := &dto.HookInput{
			UserId: 1,
			Messages: []dto.Message{
				{Role: "user", Content: "Test"},
			},
			Model: "gpt-4",
		}

		script := `
			-- Set output to invalid structure (modified=true but no messages)
			output = {
				modified = true,
				abort = false
			}
		`

		_, err := executor.Execute(script, input, 1*time.Second)
		if err == nil {
			t.Error("Expected error for invalid output structure")
		}
	})
}

func TestLuaExecutorMessageModification(t *testing.T) {
	executor := NewLuaExecutor()

	t.Run("Modify message content", func(t *testing.T) {
		input := &dto.HookInput{
			UserId: 1,
			Messages: []dto.Message{
				{Role: "user", Content: "Original content"},
			},
			Model: "gpt-4",
		}

		script := `
			local messages = {}
			for i, msg in ipairs(input.messages) do
				table.insert(messages, {
					role = msg.role,
					content = "Modified: " .. msg.content
				})
			end
			
			output = {
				modified = true,
				messages = messages,
				abort = false
			}
		`

		result, err := executor.Execute(script, input, 1*time.Second)
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		if !result.Modified {
			t.Error("Expected modified to be true")
		}
		if len(result.Messages) != 1 {
			t.Fatalf("Expected 1 message, got %d", len(result.Messages))
		}
		if result.Messages[0].Content != "Modified: Original content" {
			t.Errorf("Expected 'Modified: Original content', got '%s'", result.Messages[0].Content)
		}
	})

	t.Run("Add new message", func(t *testing.T) {
		input := &dto.HookInput{
			UserId: 1,
			Messages: []dto.Message{
				{Role: "user", Content: "User message"},
			},
			Model: "gpt-4",
		}

		script := `
			local messages = {}
			
			-- Add system message
			table.insert(messages, {
				role = "system",
				content = "You are a helpful assistant"
			})
			
			-- Add original messages
			for i, msg in ipairs(input.messages) do
				table.insert(messages, msg)
			end
			
			output = {
				modified = true,
				messages = messages,
				abort = false
			}
		`

		result, err := executor.Execute(script, input, 1*time.Second)
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		if len(result.Messages) != 2 {
			t.Fatalf("Expected 2 messages, got %d", len(result.Messages))
		}
		if result.Messages[0].Role != "system" {
			t.Errorf("Expected first message role 'system', got '%s'", result.Messages[0].Role)
		}
		if result.Messages[1].Role != "user" {
			t.Errorf("Expected second message role 'user', got '%s'", result.Messages[1].Role)
		}
	})

	t.Run("Filter messages", func(t *testing.T) {
		input := &dto.HookInput{
			UserId: 1,
			Messages: []dto.Message{
				{Role: "system", Content: "System message"},
				{Role: "user", Content: "User message"},
				{Role: "assistant", Content: "Assistant message"},
			},
			Model: "gpt-4",
		}

		script := `
			local messages = {}
			
			-- Keep only user and assistant messages
			for i, msg in ipairs(input.messages) do
				if msg.role == "user" or msg.role == "assistant" then
					table.insert(messages, msg)
				end
			end
			
			output = {
				modified = true,
				messages = messages,
				abort = false
			}
		`

		result, err := executor.Execute(script, input, 1*time.Second)
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		if len(result.Messages) != 2 {
			t.Fatalf("Expected 2 messages, got %d", len(result.Messages))
		}
		for _, msg := range result.Messages {
			if msg.Role == "system" {
				t.Error("System message should have been filtered out")
			}
		}
	})
}

func TestLuaExecutorStatePooling(t *testing.T) {
	executor := NewLuaExecutor()
	input := &dto.HookInput{
		UserId: 1,
		Messages: []dto.Message{
			{Role: "user", Content: "Test"},
		},
		Model: "gpt-4",
	}

	script := `
		output = {modified = false, abort = false}
	`

	// Execute multiple times to test state pooling
	for i := 0; i < 10; i++ {
		_, err := executor.Execute(script, input, 1*time.Second)
		if err != nil {
			t.Errorf("Iteration %d failed: %v", i, err)
		}
	}

	// Verify that state is properly cleaned between executions
	t.Run("State is cleaned between executions", func(t *testing.T) {
		// First execution sets a global variable
		script1 := `
			test_var = "should be cleaned"
			output = {modified = false, abort = false}
		`
		_, err := executor.Execute(script1, input, 1*time.Second)
		if err != nil {
			t.Fatalf("First execution failed: %v", err)
		}

		// Second execution should not see the global variable
		script2 := `
			if test_var then
				error("Global variable was not cleaned")
			end
			output = {modified = false, abort = false}
		`
		_, err = executor.Execute(script2, input, 1*time.Second)
		if err != nil {
			t.Errorf("State was not properly cleaned: %v", err)
		}
	})
}

// Benchmark tests

func BenchmarkLuaExecutor(b *testing.B) {
	executor := NewLuaExecutor()
	input := &dto.HookInput{
		UserId: 1,
		Messages: []dto.Message{
			{Role: "user", Content: "Test message"},
		},
		Model: "gpt-4",
	}

	script := `
		local messages = {}
		for i, msg in ipairs(input.messages) do
			table.insert(messages, {
				role = msg.role,
				content = string.upper(msg.content)
			})
		end
		output = {
			modified = true,
			messages = messages,
			abort = false
		}
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := executor.Execute(script, input, 1*time.Second)
		if err != nil {
			b.Fatalf("Execution failed: %v", err)
		}
	}
}
