package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/QuantumNous/new-api/dto"
	lua "github.com/yuin/gopher-lua"
	"sync"
	"time"
)

// LuaExecutor interface for executing Lua scripts
type LuaExecutor interface {
	Execute(script string, input *dto.HookInput, timeout time.Duration) (*dto.HookOutput, error)
}

// luaExecutor implements LuaExecutor with state pooling
type luaExecutor struct {
	pool *sync.Pool
}

// NewLuaExecutor creates a new Lua executor with state pooling
func NewLuaExecutor() LuaExecutor {
	return &luaExecutor{
		pool: &sync.Pool{
			New: func() interface{} {
				return createSandboxedLuaState()
			},
		},
	}
}

// createSandboxedLuaState creates a new Lua state with security restrictions
func createSandboxedLuaState() *lua.LState {
	L := lua.NewState(lua.Options{
		CallStackSize:       120,
		RegistrySize:        120 * 20,
		SkipOpenLibs:        true,
		IncludeGoStackTrace: false,
	})

	// Load only safe libraries
	for _, pair := range []struct {
		n string
		f lua.LGFunction
	}{
		{lua.LoadLibName, lua.OpenPackage},
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
	} {
		if err := L.CallByParam(lua.P{
			Fn:      L.NewFunction(pair.f),
			NRet:    0,
			Protect: true,
		}, lua.LString(pair.n)); err != nil {
			panic(err)
		}
	}

	// Disable dangerous functions
	L.SetGlobal("dofile", lua.LNil)
	L.SetGlobal("loadfile", lua.LNil)
	L.SetGlobal("require", lua.LNil)
	L.SetGlobal("load", lua.LNil)
	L.SetGlobal("loadstring", lua.LNil)

	// Disable dangerous modules
	L.SetGlobal("os", lua.LNil)
	L.SetGlobal("io", lua.LNil)
	L.SetGlobal("package", lua.LNil)
	L.SetGlobal("debug", lua.LNil)

	return L
}

// Execute executes a Lua script with the given input and timeout
func (e *luaExecutor) Execute(script string, input *dto.HookInput, timeout time.Duration) (*dto.HookOutput, error) {
	if script == "" {
		return nil, errors.New("script is empty")
	}

	if input == nil {
		return nil, errors.New("input is nil")
	}

	// Validate input
	if err := dto.ValidateHookInput(input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Get Lua state from pool
	L := e.pool.Get().(*lua.LState)
	defer func() {
		// Clear globals before returning to pool
		L.SetGlobal("input", lua.LNil)
		L.SetGlobal("output", lua.LNil)
		e.pool.Put(L)
	}()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Set up timeout handler
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			// Force close Lua state on timeout
			L.Close()
		case <-done:
			// Execution completed normally
		}
	}()

	// Convert input to Lua table
	inputTable, err := e.convertInputToLuaTable(L, input)
	if err != nil {
		close(done)
		return nil, fmt.Errorf("failed to convert input to Lua table: %w", err)
	}

	// Inject input as global variable
	L.SetGlobal("input", inputTable)

	// Execute script
	if err := L.DoString(script); err != nil {
		close(done)
		return nil, fmt.Errorf("Lua execution failed: %w", err)
	}

	// Signal completion
	close(done)

	// Check if context was cancelled (timeout)
	if ctx.Err() != nil {
		return nil, fmt.Errorf("Lua execution timeout: %w", ctx.Err())
	}

	// Get output from Lua global
	outputValue := L.GetGlobal("output")
	if outputValue == lua.LNil {
		return nil, errors.New("script must set 'output' global variable")
	}

	// Convert Lua table to HookOutput
	output, err := e.convertLuaTableToOutput(L, outputValue)
	if err != nil {
		return nil, fmt.Errorf("failed to convert Lua output: %w", err)
	}

	// Validate output
	if err := dto.ValidateHookOutput(output); err != nil {
		return nil, fmt.Errorf("invalid output: %w", err)
	}

	return output, nil
}

// convertInputToLuaTable converts HookInput to Lua table
func (e *luaExecutor) convertInputToLuaTable(L *lua.LState, input *dto.HookInput) (*lua.LTable, error) {
	table := L.NewTable()

	// Set user_id
	table.RawSetString("user_id", lua.LNumber(input.UserId))

	// Set conversation_id
	if input.ConversationId != "" {
		table.RawSetString("conversation_id", lua.LString(input.ConversationId))
	}

	// Set model
	table.RawSetString("model", lua.LString(input.Model))

	// Set token_id
	if input.TokenId > 0 {
		table.RawSetString("token_id", lua.LNumber(input.TokenId))
	}

	// Convert messages to Lua table
	messagesTable := L.NewTable()
	for i, msg := range input.Messages {
		msgTable := L.NewTable()
		msgTable.RawSetString("role", lua.LString(msg.Role))

		// Handle content (can be string or array)
		content := msg.StringContent()
		if content != "" {
			msgTable.RawSetString("content", lua.LString(content))
		}

		messagesTable.RawSetInt(i+1, msgTable) // Lua arrays are 1-indexed
	}
	table.RawSetString("messages", messagesTable)

	return table, nil
}

// convertLuaTableToOutput converts Lua table to HookOutput
func (e *luaExecutor) convertLuaTableToOutput(L *lua.LState, value lua.LValue) (*dto.HookOutput, error) {
	table, ok := value.(*lua.LTable)
	if !ok {
		return nil, errors.New("output must be a table")
	}

	output := &dto.HookOutput{}

	// Get modified field
	modifiedValue := table.RawGetString("modified")
	if modifiedValue != lua.LNil {
		if modifiedBool, ok := modifiedValue.(lua.LBool); ok {
			output.Modified = bool(modifiedBool)
		}
	}

	// Get abort field
	abortValue := table.RawGetString("abort")
	if abortValue != lua.LNil {
		if abortBool, ok := abortValue.(lua.LBool); ok {
			output.Abort = bool(abortBool)
		}
	}

	// Get reason field
	reasonValue := table.RawGetString("reason")
	if reasonValue != lua.LNil {
		if reasonStr, ok := reasonValue.(lua.LString); ok {
			output.Reason = string(reasonStr)
		}
	}

	// Get messages field
	messagesValue := table.RawGetString("messages")
	if messagesValue != lua.LNil {
		messagesTable, ok := messagesValue.(*lua.LTable)
		if !ok {
			return nil, errors.New("messages must be a table")
		}

		// Convert Lua messages table to Go slice
		messages := make([]dto.Message, 0)
		messagesTable.ForEach(func(key, value lua.LValue) {
			msgTable, ok := value.(*lua.LTable)
			if !ok {
				return
			}

			msg := dto.Message{}

			// Get role
			roleValue := msgTable.RawGetString("role")
			if roleStr, ok := roleValue.(lua.LString); ok {
				msg.Role = string(roleStr)
			}

			// Get content
			contentValue := msgTable.RawGetString("content")
			if contentStr, ok := contentValue.(lua.LString); ok {
				msg.Content = string(contentStr)
			}

			messages = append(messages, msg)
		})

		output.Messages = messages
	}

	return output, nil
}
