package constant

// Message Hook Types
const (
	MessageHookTypeLua  = 1
	MessageHookTypeHTTP = 2
)

// Message Hook Configuration Keys
const (
	MessageHookDefaultTimeout = "MessageHookDefaultTimeout"
	MessageHookLuaPoolSize    = "MessageHookLuaPoolSize"
	MessageHookHTTPPoolSize   = "MessageHookHTTPPoolSize"
	MessageHookCacheTTL       = "MessageHookCacheTTL"
)

// Message Hook Cache Keys
const (
	MessageHookCacheKeyPrefix = "message_hook:"
	EnabledHooksCacheKey      = "message_hook:enabled_hooks"
)

// Message Hook Context Keys
const (
	ContextKeyConversationId = "conversation_id"
)
