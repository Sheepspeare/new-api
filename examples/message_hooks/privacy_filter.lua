-- Privacy Filter Hook
-- This hook replaces sensitive information (emails, phone numbers, SSNs) with placeholders

local input = input
local output = {
    modified = false,
    messages = {},
    abort = false,
    reason = ""
}

-- Patterns for sensitive data
local patterns = {
    -- Email pattern: username@domain.com
    email = "[%w%._%%-]+@[%w%._%%-]+%.%w+",
    -- Phone pattern: 123-456-7890 or (123) 456-7890
    phone = "%d%d%d[%-%s]?%d%d%d[%-%s]?%d%d%d%d",
    -- SSN pattern: 123-45-6789
    ssn = "%d%d%d%-%d%d%-%d%d%d%d",
    -- Credit card pattern: 1234-5678-9012-3456
    credit_card = "%d%d%d%d[%-%s]?%d%d%d%d[%-%s]?%d%d%d%d[%-%s]?%d%d%d%d"
}

-- Process each message
for i, msg in ipairs(input.messages) do
    local content = msg.content
    local original = content
    
    -- Replace sensitive patterns
    content = string.gsub(content, patterns.email, "[EMAIL_REDACTED]")
    content = string.gsub(content, patterns.phone, "[PHONE_REDACTED]")
    content = string.gsub(content, patterns.ssn, "[SSN_REDACTED]")
    content = string.gsub(content, patterns.credit_card, "[CARD_REDACTED]")
    
    -- Check if modified
    if content ~= original then
        output.modified = true
    end
    
    -- Add to output messages
    table.insert(output.messages, {
        role = msg.role,
        content = content
    })
end

-- Return output
_G.output = output
