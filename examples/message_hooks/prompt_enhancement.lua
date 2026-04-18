-- Prompt Enhancement Hook
-- This hook enhances user queries by adding context and instructions

local input = input
local output = {
    modified = false,
    messages = {},
    abort = false,
    reason = ""
}

-- Enhancement templates based on message content
local function enhance_prompt(content)
    -- Check if it's a question
    if string.find(content, "%?") then
        return "Please provide a detailed and accurate answer to the following question: " .. content
    end
    
    -- Check if it's a code request
    if string.find(string.lower(content), "code") or 
       string.find(string.lower(content), "function") or
       string.find(string.lower(content), "implement") then
        return "Please provide well-documented code with explanations for: " .. content
    end
    
    -- Check if it's a creative request
    if string.find(string.lower(content), "write") or 
       string.find(string.lower(content), "create") or
       string.find(string.lower(content), "story") then
        return "Please be creative and engaging when responding to: " .. content
    end
    
    -- Default enhancement
    return "Please provide a helpful and comprehensive response to: " .. content
end

-- Process each message
for i, msg in ipairs(input.messages) do
    if msg.role == "user" then
        -- Enhance user messages
        local enhanced = enhance_prompt(msg.content)
        
        table.insert(output.messages, {
            role = msg.role,
            content = enhanced
        })
        output.modified = true
    else
        -- Keep other messages unchanged
        table.insert(output.messages, msg)
    end
end

_G.output = output
