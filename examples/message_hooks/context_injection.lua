-- Context Injection Hook
-- This hook adds system context or memory to conversations

local input = input
local output = {
    modified = true,
    messages = {},
    abort = false,
    reason = ""
}

-- Add system message with context
-- In a real implementation, this could query a vector database or memory store
local system_context = {
    role = "system",
    content = "You are a helpful AI assistant. Remember to be concise and accurate in your responses."
}

-- Check if system message already exists
local has_system = false
for i, msg in ipairs(input.messages) do
    if msg.role == "system" then
        has_system = true
        break
    end
end

-- Add system message if not present
if not has_system then
    table.insert(output.messages, system_context)
end

-- Add all original messages
for i, msg in ipairs(input.messages) do
    table.insert(output.messages, msg)
end

-- Add user-specific context based on user_id
-- This is a simple example - in production, you'd query a database
if input.user_id == 1 then
    -- Add context for specific user
    table.insert(output.messages, 1, {
        role = "system",
        content = "Note: This user prefers technical explanations."
    })
end

_G.output = output
