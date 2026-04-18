# Message Hook Examples

This directory contains example Lua scripts for the Message Hook middleware system.

## Available Examples

### 1. Privacy Filter (`privacy_filter.lua`)

**Purpose:** Automatically redacts sensitive information from user messages.

**Features:**
- Detects and replaces email addresses
- Detects and replaces phone numbers
- Detects and replaces SSN (Social Security Numbers)
- Detects and replaces credit card numbers

**Use Case:** Protect user privacy by preventing sensitive information from being sent to LLM providers.

**Configuration:**
```json
{
  "name": "Privacy Filter",
  "type": 1,
  "content": "<paste privacy_filter.lua content>",
  "enabled": true,
  "priority": 1,
  "timeout": 5000
}
```

### 2. Prompt Enhancement (`prompt_enhancement.lua`)

**Purpose:** Automatically enhance user prompts to get better responses.

**Features:**
- Adds context to questions
- Adds documentation requirements to code requests
- Adds creativity instructions to creative requests
- Provides default enhancement for other requests

**Use Case:** Improve response quality by providing better context to the LLM.

**Configuration:**
```json
{
  "name": "Prompt Enhancement",
  "type": 1,
  "content": "<paste prompt_enhancement.lua content>",
  "enabled": true,
  "priority": 5,
  "timeout": 5000
}
```

### 3. Content Moderation (`content_moderation.lua`)

**Purpose:** Block inappropriate or policy-violating content.

**Features:**
- Keyword-based filtering
- Aborts requests containing blocked keywords
- Returns clear error message to user

**Use Case:** Enforce content policies and prevent misuse.

**Configuration:**
```json
{
  "name": "Content Moderation",
  "type": 1,
  "content": "<paste content_moderation.lua content>",
  "enabled": true,
  "priority": 1,
  "timeout": 5000,
  "filter_models": ""
}
```

### 4. Context Injection (`context_injection.lua`)

**Purpose:** Add system context or memory to conversations.

**Features:**
- Adds system messages if not present
- Can inject user-specific context
- Supports memory/context retrieval (example shows structure)

**Use Case:** Provide consistent system instructions or user-specific context.

**Configuration:**
```json
{
  "name": "Context Injection",
  "type": 1,
  "content": "<paste context_injection.lua content>",
  "enabled": true,
  "priority": 2,
  "timeout": 5000
}
```

## Hook Execution Order

Hooks execute in ascending priority order. Recommended priorities:

1. **Priority 1:** Content Moderation (block bad content first)
2. **Priority 2:** Privacy Filter (redact sensitive info)
3. **Priority 3:** Context Injection (add system context)
4. **Priority 5:** Prompt Enhancement (enhance user prompts)

## Input Structure

All hooks receive an `input` table with the following structure:

```lua
{
    user_id = 123,              -- User ID
    conversation_id = "conv-1", -- Optional conversation ID
    model = "gpt-4",            -- Model name
    token_id = 456,             -- Optional token ID
    messages = {                -- Array of messages
        {
            role = "user",
            content = "Hello"
        }
    }
}
```

## Output Structure

All hooks must set a global `output` table:

```lua
output = {
    modified = false,     -- true if messages were changed
    messages = {},        -- Modified messages (required if modified=true)
    abort = false,        -- true to stop request
    reason = ""          -- Reason for abort (optional)
}
```

## Best Practices

1. **Always set `_G.output`:** The hook must set the global output variable.

2. **Handle errors gracefully:** Use `pcall()` for operations that might fail.

3. **Keep scripts fast:** Target execution time under 10ms.

4. **Validate input:** Check that required fields exist before using them.

5. **Use appropriate priorities:** Lower numbers execute first.

6. **Test thoroughly:** Use the test endpoint before enabling in production.

## Testing Hooks

Use the test endpoint to verify your hook works correctly:

```bash
POST /api/message-hooks/test
{
  "hook": {
    "type": 1,
    "content": "<your lua script>",
    "timeout": 5000
  },
  "input": {
    "user_id": 1,
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Test message"}
    ]
  }
}
```

## Security Notes

- Lua scripts run in a sandboxed environment
- Dangerous modules (os, io, package, debug) are disabled
- File system access is not available
- Network access is not available
- Memory is limited to 10MB per script

## Advanced Examples

For more complex use cases, consider:

- **Vector Database Integration:** Use HTTP hooks to query external vector databases
- **User Memory:** Store and retrieve conversation context
- **Smart Routing:** Route requests to different models based on content
- **Cost Optimization:** Compress or summarize long conversations
- **Multi-language Support:** Detect and translate messages

See the HTTP hook examples for external service integration patterns.
