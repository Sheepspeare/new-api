# Message Hooks API Documentation

## Overview

The Message Hooks API allows administrators to create, manage, and test message processing hooks that intercept and modify user messages before they reach the LLM.

**Base URL:** `/api/message-hooks`

**Authentication:** All endpoints require administrator authentication.

## Endpoints

### List Message Hooks

Retrieve a paginated list of message hooks.

**Endpoint:** `GET /api/message-hooks`

**Query Parameters:**
- `page` (integer, optional): Page number (default: 1)
- `page_size` (integer, optional): Items per page (default: 10)
- `enabled` (boolean, optional): Filter by enabled status

**Response:**
```json
{
  "success": true,
  "message": "",
  "data": [
    {
      "id": 1,
      "name": "Privacy Filter",
      "description": "Redacts sensitive information",
      "type": 1,
      "content": "-- Lua script content",
      "enabled": true,
      "priority": 10,
      "timeout": 5000,
      "filter_users": "[1, 2, 3]",
      "filter_models": "[\"gpt-4\"]",
      "filter_tokens": "",
      "call_count": 1000,
      "success_count": 995,
      "error_count": 5,
      "avg_duration": 8.5,
      "created_time": 1678901234,
      "updated_time": 1678901234
    }
  ],
  "total": 1
}
```

### Get Message Hook

Retrieve a single message hook by ID.

**Endpoint:** `GET /api/message-hooks/:id`

**Path Parameters:**
- `id` (integer, required): Hook ID

**Response:**
```json
{
  "success": true,
  "message": "",
  "data": {
    "id": 1,
    "name": "Privacy Filter",
    "description": "Redacts sensitive information",
    "type": 1,
    "content": "-- Lua script content",
    "enabled": true,
    "priority": 10,
    "timeout": 5000,
    "filter_users": "[1, 2, 3]",
    "filter_models": "[\"gpt-4\"]",
    "filter_tokens": "",
    "call_count": 1000,
    "success_count": 995,
    "error_count": 5,
    "avg_duration": 8.5,
    "created_time": 1678901234,
    "updated_time": 1678901234
  }
}
```

### Create Message Hook

Create a new message hook.

**Endpoint:** `POST /api/message-hooks`

**Request Body:**
```json
{
  "name": "Privacy Filter",
  "description": "Redacts sensitive information from messages",
  "type": 1,
  "content": "-- Lua script or HTTPS URL",
  "enabled": true,
  "priority": 10,
  "timeout": 5000,
  "filter_users": "[1, 2, 3]",
  "filter_models": "[\"gpt-4\", \"gpt-3.5-turbo\"]",
  "filter_tokens": "[100, 200]"
}
```

**Field Descriptions:**
- `name` (string, required): Unique hook name (max 128 characters)
- `description` (string, optional): Hook description
- `type` (integer, required): Hook type (1=Lua, 2=HTTP)
- `content` (string, required): Lua script (max 1MB) or HTTPS URL
- `enabled` (boolean, optional): Enable/disable hook (default: false)
- `priority` (integer, optional): Execution priority (default: 0, lower executes first)
- `timeout` (integer, optional): Timeout in milliseconds (100-30000, default: 5000)
- `filter_users` (string, optional): JSON array of user IDs (empty = match all)
- `filter_models` (string, optional): JSON array of model names (empty = match all)
- `filter_tokens` (string, optional): JSON array of token IDs (empty = match all)

**Response:**
```json
{
  "success": true,
  "message": "Hook created successfully",
  "data": {
    "id": 1,
    "name": "Privacy Filter",
    ...
  }
}
```

**Error Responses:**
- `400 Bad Request`: Invalid input (validation error)
- `403 Forbidden`: Not an administrator
- `409 Conflict`: Hook name already exists

### Update Message Hook

Update an existing message hook.

**Endpoint:** `PUT /api/message-hooks/:id`

**Path Parameters:**
- `id` (integer, required): Hook ID

**Request Body:** Same as Create Message Hook

**Response:**
```json
{
  "success": true,
  "message": "Hook updated successfully",
  "data": {
    "id": 1,
    "name": "Privacy Filter",
    ...
  }
}
```

### Delete Message Hook

Delete a message hook.

**Endpoint:** `DELETE /api/message-hooks/:id`

**Path Parameters:**
- `id` (integer, required): Hook ID

**Response:**
```json
{
  "success": true,
  "message": "Hook deleted successfully"
}
```

### Get Hook Statistics

Retrieve execution statistics for a hook.

**Endpoint:** `GET /api/message-hooks/:id/stats`

**Path Parameters:**
- `id` (integer, required): Hook ID

**Response:**
```json
{
  "success": true,
  "data": {
    "call_count": 1000,
    "success_count": 995,
    "error_count": 5,
    "success_rate": 99.5,
    "avg_duration": 8.5
  }
}
```

### Test Message Hook

Test a hook with sample input without affecting production.

**Endpoint:** `POST /api/message-hooks/test`

**Request Body:**
```json
{
  "hook": {
    "type": 1,
    "content": "-- Lua script or HTTPS URL",
    "timeout": 5000
  },
  "input": {
    "user_id": 1,
    "conversation_id": "conv-123",
    "model": "gpt-4",
    "token_id": 100,
    "messages": [
      {
        "role": "user",
        "content": "Test message"
      }
    ]
  }
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "output": {
      "modified": true,
      "messages": [
        {
          "role": "user",
          "content": "Modified test message"
        }
      ],
      "abort": false,
      "reason": ""
    },
    "duration": 8.5,
    "error": null
  }
}
```

## Data Structures

### HookInput

Input data passed to hooks:

```json
{
  "user_id": 123,
  "conversation_id": "conv-456",
  "model": "gpt-4",
  "token_id": 789,
  "messages": [
    {
      "role": "user",
      "content": "Hello"
    }
  ]
}
```

**Fields:**
- `user_id` (integer, required): User ID (must be > 0)
- `conversation_id` (string, optional): Conversation identifier
- `model` (string, required): Model name
- `token_id` (integer, optional): Token ID
- `messages` (array, required): Array of message objects (must be non-empty)

**Message Object:**
- `role` (string, required): Message role (system, user, assistant, tool, function)
- `content` (string, required): Message content

### HookOutput

Output data returned by hooks:

```json
{
  "modified": true,
  "messages": [
    {
      "role": "user",
      "content": "Modified content"
    }
  ],
  "abort": false,
  "reason": ""
}
```

**Fields:**
- `modified` (boolean, required): Whether messages were modified
- `messages` (array, required if modified=true): Modified messages array
- `abort` (boolean, required): Whether to abort the request
- `reason` (string, optional): Reason for abort (recommended if abort=true)

## Hook Types

### Type 1: Lua Script

Executes embedded Lua script in a sandboxed environment.

**Characteristics:**
- Fast execution (target <10ms)
- Sandboxed (no file/network access)
- Memory limited to 10MB
- Dangerous modules disabled (os, io, package, debug)

**Example:**
```lua
local input = input
local output = {
    modified = true,
    messages = {},
    abort = false
}

for i, msg in ipairs(input.messages) do
    table.insert(output.messages, {
        role = msg.role,
        content = string.upper(msg.content)
    })
end

_G.output = output
```

### Type 2: HTTP Service

Calls external HTTPS service for processing.

**Characteristics:**
- Supports external integrations (vector DBs, APIs)
- Configurable timeout
- HTTPS required (HTTP blocked)
- SSRF protection (private IPs blocked)

**Request Format:**
```
POST <configured_url>
Content-Type: application/json
User-Agent: new-api-message-hook/1.0

{
  "user_id": 123,
  "model": "gpt-4",
  "messages": [...]
}
```

**Expected Response:**
```
HTTP/1.1 200 OK
Content-Type: application/json

{
  "modified": true,
  "messages": [...],
  "abort": false
}
```

## Filters

Filters determine which requests trigger a hook.

### User Filter

Match specific user IDs:
```json
{
  "filter_users": "[1, 2, 3, 100]"
}
```

Empty string matches all users.

### Model Filter

Match specific model names:
```json
{
  "filter_models": "[\"gpt-4\", \"gpt-3.5-turbo\", \"claude-2\"]"
}
```

Empty string matches all models.

### Token Filter

Match specific token IDs:
```json
{
  "filter_tokens": "[100, 200, 300]"
}
```

Empty string matches all tokens.

### Combined Filters

All non-empty filters must match for the hook to execute:
```json
{
  "filter_users": "[1, 2]",
  "filter_models": "[\"gpt-4\"]",
  "filter_tokens": "[100]"
}
```

This hook only executes when:
- User ID is 1 OR 2, AND
- Model is "gpt-4", AND
- Token ID is 100

## Execution Flow

1. Request arrives at `/v1/chat/completions`
2. Authentication and channel selection occur
3. Message Hook middleware executes:
   - Check if hooks are globally enabled
   - Load enabled hooks sorted by priority
   - For each hook:
     - Check if filters match
     - Execute hook (Lua or HTTP)
     - If modified=true, update messages
     - If abort=true, stop and return error
     - If error/timeout, log and continue
4. Modified request continues to LLM

## Error Handling

Hooks use graceful degradation:
- Individual hook failures don't break the request chain
- Timeouts are logged but don't stop processing
- Only `abort=true` stops the request
- Errors are logged with hook name and details

## Performance

**Targets:**
- Middleware overhead (disabled): <1ms
- Middleware overhead (enabled): <20ms
- Lua execution: <10ms
- Cache hit: <5ms
- Database query: <50ms

## Security

**Lua Sandbox:**
- No file system access
- No network access
- No process execution
- Memory limited to 10MB
- Dangerous modules disabled

**HTTP Hooks:**
- HTTPS required
- Private IPs blocked (SSRF protection)
- Timeout enforced
- Connection pooling

**Access Control:**
- All endpoints require administrator role
- All operations are logged with user ID
- Hook content is stored securely

## Rate Limiting

(Optional feature - not yet implemented)

Per-user and global rate limits can be configured to prevent abuse.

## Examples

See `/examples/message_hooks/` for complete examples:
- Privacy filtering
- Prompt enhancement
- Content moderation
- Context injection

## Support

For issues or questions:
1. Check the examples directory
2. Review the integration guides
3. Test hooks using the test endpoint
4. Check logs for error details
