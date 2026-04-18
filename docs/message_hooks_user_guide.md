# Message Hooks User Guide

## Introduction

Message Hooks allow you to process and modify user messages before they reach the AI model. This powerful feature enables use cases like:

- **Privacy Protection**: Automatically redact sensitive information
- **Prompt Enhancement**: Improve user queries for better responses
- **Content Moderation**: Block inappropriate content
- **Context Injection**: Add relevant context from external sources
- **Smart Routing**: Route requests based on content analysis

## Getting Started

### Prerequisites

- Administrator access to new-api
- Basic understanding of Lua (for Lua hooks) or HTTP APIs (for HTTP hooks)

### Accessing the Hook Management Interface

1. Log in to new-api as an administrator
2. Navigate to the "Message Hooks" section in the sidebar
3. You'll see a list of existing hooks (if any)

## Creating Your First Hook

### Step 1: Click "Create Hook"

Click the "Create New Hook" button to open the hook creation form.

### Step 2: Fill in Basic Information

**Name**: Give your hook a unique, descriptive name
- Example: "Privacy Filter", "Prompt Enhancer"

**Description**: Explain what your hook does
- Example: "Redacts email addresses and phone numbers from messages"

**Type**: Choose between Lua Script or HTTP Service
- **Lua Script**: Fast, embedded scripts (recommended for simple operations)
- **HTTP Service**: External service calls (for complex integrations)

### Step 3: Write Your Hook Logic

#### For Lua Hooks:

```lua
-- Access input data
local input = input

-- Create output structure
local output = {
    modified = false,
    messages = {},
    abort = false,
    reason = ""
}

-- Process messages
for i, msg in ipairs(input.messages) do
    -- Your logic here
    table.insert(output.messages, {
        role = msg.role,
        content = msg.content  -- Modify as needed
    })
end

-- Set the output
_G.output = output
```

#### For HTTP Hooks:

Enter the HTTPS URL of your service:
```
https://your-service.com/hook
```

Your service will receive POST requests and must return the appropriate JSON response.

### Step 4: Configure Settings

**Priority**: Lower numbers execute first (default: 0)
- Use priority 1 for content moderation
- Use priority 5 for prompt enhancement
- Use priority 10 for logging

**Timeout**: Maximum execution time in milliseconds (100-30000)
- Recommended: 5000ms (5 seconds)
- Lua scripts typically need 100-1000ms
- HTTP services may need 3000-10000ms

**Enabled**: Toggle to enable/disable the hook
- Start with disabled while testing
- Enable after successful testing

### Step 5: Configure Filters (Optional)

Filters determine which requests trigger your hook.

**User Filter**: Comma-separated user IDs
- Example: `[1, 2, 3]`
- Leave empty to match all users

**Model Filter**: Comma-separated model names
- Example: `["gpt-4", "gpt-3.5-turbo"]`
- Leave empty to match all models

**Token Filter**: Comma-separated token IDs
- Example: `[100, 200]`
- Leave empty to match all tokens

### Step 6: Test Your Hook

Before enabling, use the "Test" button to verify your hook works correctly:

1. Click the "Test" button next to your hook
2. Enter sample input messages
3. Review the output
4. Check for errors or unexpected behavior

### Step 7: Enable and Monitor

1. Enable your hook using the toggle switch
2. Monitor the statistics dashboard
3. Check logs for any errors
4. Adjust as needed based on performance

## Common Use Cases

### Use Case 1: Privacy Protection

**Goal**: Automatically redact email addresses from messages

**Implementation**:
```lua
local input = input
local output = {
    modified = false,
    messages = {},
    abort = false
}

for i, msg in ipairs(input.messages) do
    local content = msg.content
    local original = content
    
    -- Replace email pattern
    content = string.gsub(content, "[%w%._%%-]+@[%w%._%%-]+%.%w+", "[EMAIL]")
    
    if content ~= original then
        output.modified = true
    end
    
    table.insert(output.messages, {
        role = msg.role,
        content = content
    })
end

_G.output = output
```

**Configuration**:
- Priority: 1 (execute early)
- Timeout: 1000ms
- Filters: None (apply to all requests)

### Use Case 2: Content Moderation

**Goal**: Block requests containing inappropriate keywords

**Implementation**:
```lua
local input = input
local output = {
    modified = false,
    messages = {},
    abort = false,
    reason = ""
}

local blocked_words = {"spam", "hack", "exploit"}

for i, msg in ipairs(input.messages) do
    local content_lower = string.lower(msg.content)
    
    for j, word in ipairs(blocked_words) do
        if string.find(content_lower, word) then
            output.abort = true
            output.reason = "Content policy violation"
            _G.output = output
            return
        end
    end
end

output.messages = input.messages
_G.output = output
```

**Configuration**:
- Priority: 1 (execute first)
- Timeout: 500ms
- Filters: None (apply to all)

### Use Case 3: Prompt Enhancement

**Goal**: Add helpful context to user questions

**Implementation**:
```lua
local input = input
local output = {
    modified = false,
    messages = {},
    abort = false
}

for i, msg in ipairs(input.messages) do
    if msg.role == "user" and string.find(msg.content, "%?") then
        -- It's a question, enhance it
        table.insert(output.messages, {
            role = msg.role,
            content = "Please provide a detailed answer to: " .. msg.content
        })
        output.modified = true
    else
        table.insert(output.messages, msg)
    end
end

_G.output = output
```

**Configuration**:
- Priority: 5 (after moderation)
- Timeout: 1000ms
- Filters: None

### Use Case 4: Model-Specific Instructions

**Goal**: Add model-specific system messages

**Implementation**:
```lua
local input = input
local output = {
    modified = true,
    messages = {},
    abort = false
}

-- Add model-specific system message
if input.model == "gpt-4" then
    table.insert(output.messages, {
        role = "system",
        content = "You are using GPT-4. Provide detailed, technical responses."
    })
elseif input.model == "gpt-3.5-turbo" then
    table.insert(output.messages, {
        role = "system",
        content = "You are using GPT-3.5. Be concise and efficient."
    })
end

-- Add original messages
for i, msg in ipairs(input.messages) do
    table.insert(output.messages, msg)
end

_G.output = output
```

**Configuration**:
- Priority: 2
- Timeout: 500ms
- Filters: None

## Understanding Hook Statistics

The statistics dashboard shows:

**Call Count**: Total number of times the hook was executed
**Success Count**: Number of successful executions
**Error Count**: Number of failed executions
**Success Rate**: Percentage of successful executions
**Average Duration**: Average execution time in milliseconds

### Interpreting Statistics

- **High error count**: Check your hook logic for bugs
- **High average duration**: Optimize your code or increase timeout
- **Low success rate**: Review error logs and fix issues
- **Zero calls**: Check if filters are too restrictive

## Best Practices

### 1. Start Simple

Begin with simple hooks and gradually add complexity:
```lua
-- Start with this
output.messages = input.messages

-- Then add logic incrementally
```

### 2. Test Thoroughly

Always test before enabling:
- Test with various input types
- Test edge cases (empty messages, special characters)
- Test timeout scenarios
- Test with actual user data (in test mode)

### 3. Use Appropriate Priorities

Organize hooks by priority:
1. **Priority 1-2**: Security (moderation, privacy)
2. **Priority 3-5**: Enhancement (context, prompts)
3. **Priority 8-10**: Logging and analytics

### 4. Monitor Performance

Keep execution time low:
- Target <10ms for Lua hooks
- Target <100ms for HTTP hooks
- Optimize slow operations
- Use caching when possible

### 5. Handle Errors Gracefully

Always set output even on errors:
```lua
local success, error = pcall(function()
    -- Your risky operation
end)

if not success then
    -- Return unchanged
    output.messages = input.messages
    _G.output = output
    return
end
```

### 6. Document Your Hooks

Add clear descriptions:
- What the hook does
- Why it's needed
- Any special considerations
- Expected behavior

## Troubleshooting

### Hook Not Executing

**Check**:
1. Is the hook enabled?
2. Do the filters match your request?
3. Is the global hook system enabled?
4. Check the priority order

### Hook Timing Out

**Solutions**:
1. Increase the timeout value
2. Optimize your code
3. Remove expensive operations
4. Use caching

### Unexpected Results

**Debug Steps**:
1. Use the test endpoint with sample data
2. Add logging to your Lua script (use print statements)
3. Check the statistics for error patterns
4. Review the hook execution logs

### Hook Errors

**Common Causes**:
1. Syntax errors in Lua script
2. Missing `_G.output` assignment
3. Invalid output structure
4. Timeout exceeded
5. HTTP service unavailable

## Security Considerations

### Lua Sandbox Limitations

Lua hooks run in a sandbox with restrictions:
- No file system access
- No network access
- No process execution
- Limited memory (10MB)
- Disabled dangerous modules

### HTTP Hook Security

HTTP hooks have security measures:
- HTTPS required (HTTP blocked)
- Private IPs blocked (SSRF protection)
- Timeout enforcement
- Connection pooling

### Data Privacy

Be mindful of data handling:
- Don't log sensitive information
- Redact PII before external calls
- Use encryption for sensitive data
- Follow data protection regulations

## Advanced Topics

### Chaining Multiple Hooks

Hooks execute in priority order, with each hook receiving the output of the previous hook:

```
Request → Hook 1 (Priority 1) → Hook 2 (Priority 5) → Hook 3 (Priority 10) → LLM
```

### Conditional Logic

Use Lua's conditional statements:
```lua
if input.user_id == 123 then
    -- Special handling for user 123
elseif string.find(input.model, "gpt-4") then
    -- Special handling for GPT-4
else
    -- Default handling
end
```

### Pattern Matching

Use Lua patterns for text processing:
```lua
-- Match email
local email_pattern = "[%w%._%%-]+@[%w%._%%-]+%.%w+"

-- Match phone
local phone_pattern = "%d%d%d%-%d%d%d%-%d%d%d%d"

-- Match URL
local url_pattern = "https?://[%w%._%%-/]+"
```

## Getting Help

If you need assistance:

1. **Check the Examples**: Review the example scripts in `/examples/message_hooks/`
2. **Read the API Docs**: See `/docs/message_hooks_api.md`
3. **Test Your Hook**: Use the test endpoint to debug
4. **Check Logs**: Review system logs for error details
5. **Monitor Statistics**: Use the dashboard to identify issues

## Conclusion

Message Hooks provide powerful customization capabilities for your AI gateway. Start with simple use cases, test thoroughly, and gradually build more complex workflows as you become comfortable with the system.

Happy hooking! 🎣
