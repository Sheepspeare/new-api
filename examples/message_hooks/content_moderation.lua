-- Content Moderation Hook
-- This hook blocks inappropriate content based on keyword filtering

local input = input
local output = {
    modified = false,
    messages = {},
    abort = false,
    reason = ""
}

-- Blocked keywords (case-insensitive)
local blocked_keywords = {
    "hack",
    "exploit",
    "malware",
    "virus",
    "crack",
    "pirate",
    "illegal"
}

-- Check all messages for blocked content
for i, msg in ipairs(input.messages) do
    local content_lower = string.lower(msg.content)
    
    -- Check for blocked keywords
    for j, keyword in ipairs(blocked_keywords) do
        if string.find(content_lower, keyword, 1, true) then
            output.abort = true
            output.reason = "Content policy violation: inappropriate content detected"
            _G.output = output
            return
        end
    end
end

-- No violations found, continue unchanged
output.messages = input.messages
_G.output = output
