# Message Hook Context Information

This document describes what information is available to message hooks during execution and when it becomes available in the request lifecycle.

## Request Lifecycle

```
Client Request
    ↓
TokenAuth Middleware (sets user/token context)
    ↓
MessageHook Middleware (reads context + request body)
    ↓
Distribute Middleware (selects channel)
    ↓
Relay Handler (forwards to upstream)
```

## Available Information in Hook Input

When a message hook is executed, it receives a `HookInput` object with the following fields:

### 1. User Information

| Field | Type | Description | Source |
|-------|------|-------------|--------|
| `user_id` | int | User ID from the authenticated token | Context: `c.GetInt("id")` set by TokenAuth |
| `token_id` | int | Token ID used for this request | Context: `c.GetInt("token_id")` set by TokenAuth |

### 2. Conversation Information

| Field | Type | Description | Source |
|-------|------|-------------|--------|
| `conversation_id` | string | Conversation/session identifier (if available) | Context: `c.GetString(constant.ContextKeyConversationId)` |

### 3. Request Information

| Field | Type | Description | Source |
|-------|------|-------------|--------|
| `messages` | []Message | Array of chat messages (user, assistant, system) | Request body: parsed from JSON |
| `model` | string | Model name requested (e.g., "gpt-4", "claude-3") | Request body: `request.Model` |

### 4. Message Structure

Each message in the `messages` array contains:

```json
{
  "role": "user|assistant|system",
  "content": "message text or structured content"
}
```

## What is NOT Available

The following information is **NOT** available to hooks because it hasn't been determined yet:

- ❌ **Channel ID**: Not selected until Distribute middleware runs (after hooks)
- ❌ **Channel Name**: Not available until channel selection
- ❌ **Upstream Provider**: Not determined until Distribute middleware
- ❌ **Request IP**: Not included in hook input (but could be added if needed)
- ❌ **User Group**: Available in context but not passed to hooks
- ❌ **Token Name**: Available in context but not passed to hooks

## Additional Context Available (Not Currently Passed to Hooks)

The following information is available in the Gin context but not currently passed to hook input:

```go
// User information
c.GetString("username")           // Username
c.GetInt("role")                  // User role (0=guest, 1=user, 10=admin, 100=root)
c.GetString("user_group")         // User's group

// Token information
c.GetString("token_key")          // Token key
c.GetString("token_name")         // Token name
c.GetBool("token_unlimited_quota") // Whether token has unlimited quota
c.GetInt("token_quota")           // Token remaining quota

// Request routing
c.GetString(constant.ContextKeyUsingGroup) // Group being used for this request
```

## Example Hook Input

```json
{
  "user_id": 1,
  "token_id": 10,
  "conversation_id": "conv-abc123",
  "model": "gpt-4",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "Hello, how are you?"
    }
  ]
}
```

## Filtering Hooks

Hooks can be filtered based on:

1. **User IDs** (`filter_users`): Only execute for specific users
2. **Model Names** (`filter_models`): Only execute for specific models
3. **Token IDs** (`filter_tokens`): Only execute for specific tokens

If a filter field is empty, the hook applies to all requests.

## Extending Hook Context

If you need additional information in hooks, you can modify:

1. **`dto/message_hook.go`**: Add fields to `HookInput` struct
2. **`middleware/message_hook.go`**: Extract additional data from context and pass to hooks
3. **`service/message_hook_service.go`**: Update validation if needed

Example: Adding user group to hook input:

```go
// In middleware/message_hook.go
input := &dto.HookInput{
    UserId:         userId,
    ConversationId: conversationId,
    Messages:       request.Messages,
    Model:          modelName,
    TokenId:        tokenId,
    UserGroup:      c.GetString("user_group"), // Add this
}
```

## Debugging Hook Execution

The system logs detailed information about hook execution:

```
[MESSAGE_HOOK] Request received: POST /v1/chat/completions
[MESSAGE_HOOK] User authenticated: userId=1
[MESSAGE_HOOK] Request info: userId=1, tokenId=10, model=gpt-4
[MESSAGE_HOOK] Found 2 enabled hooks
[MESSAGE_HOOK] Hook #1: id=2, name=apikey隐私替换, type=1, priority=100
[MESSAGE_HOOK] Executing 2 hooks for userId=1...
[MESSAGE_HOOK_SERVICE] Hook 2 matches filters, executing...
[MESSAGE_HOOK_SERVICE] Hook 2 executed successfully in 15ms
[MESSAGE_HOOK_SERVICE] Hook 2 modified messages: 1 → 1
```

Check your application logs to see if hooks are being triggered and what information they receive.
