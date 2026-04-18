# Relay格式转换机制说明

## 概述

new-api项目支持多种API格式之间的相互转换，可以让客户端使用一种格式（如OpenAI格式）访问不同格式的上游服务（如Claude、Gemini等），也支持反向转换。

## 支持的格式类型

项目定义了以下Relay格式（`types/relay_format.go`）：

```go
const (
    RelayFormatOpenAI                    = "openai"
    RelayFormatClaude                    = "claude"
    RelayFormatGemini                    = "gemini"
    RelayFormatOpenAIResponses           = "openai_responses"
    RelayFormatOpenAIResponsesCompaction = "openai_responses_compaction"
    RelayFormatOpenAIAudio               = "openai_audio"
    RelayFormatOpenAIImage               = "openai_image"
    RelayFormatOpenAIRealtime            = "openai_realtime"
    RelayFormatRerank                    = "rerank"
    RelayFormatEmbedding                 = "embedding"
    RelayFormatTask                      = "task"
    RelayFormatMjProxy                   = "mj_proxy"
)
```

## 格式转换架构

### 1. 路由层定义格式

在 `router/relay-router.go` 中，不同的API端点被分配了不同的格式：

```go
// Claude格式端点
httpRouter.POST("/messages", func(c *gin.Context) {
    controller.Relay(c, types.RelayFormatClaude)
})

// OpenAI格式端点
httpRouter.POST("/chat/completions", func(c *gin.Context) {
    controller.Relay(c, types.RelayFormatOpenAI)
})

// Responses格式端点
httpRouter.POST("/responses", func(c *gin.Context) {
    controller.Relay(c, types.RelayFormatOpenAIResponses)
})

// Gemini格式端点
httpRouter.POST("/models/*path", func(c *gin.Context) {
    controller.Relay(c, types.RelayFormatGemini)
})
```

### 2. 适配器层实现转换

每个channel适配器（`relay/channel/*/adaptor.go`）实现了 `Adaptor` 接口，提供格式转换方法：

```go
type Adaptor interface {
    // 转换OpenAI格式请求到上游格式
    ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error)
    
    // 转换Claude格式请求到上游格式
    ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error)
    
    // 转换Gemini格式请求到上游格式
    ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error)
    
    // 其他格式转换方法...
}
```

### 3. 服务层提供转换逻辑

`service/convert.go` 提供了核心的格式转换函数：

- `ClaudeToOpenAIRequest()` - Claude格式 → OpenAI格式
- `GeminiToOpenAIRequest()` - Gemini格式 → OpenAI格式
- `RequestOpenAI2ClaudeMessage()` - OpenAI格式 → Claude格式

## 支持的转换场景

### 场景1: 客户端使用OpenAI格式访问Claude上游

**流程：**
1. 客户端发送请求到 `/v1/chat/completions` (OpenAI格式)
2. 路由识别为 `RelayFormatOpenAI`
3. Distribute中间件选择Claude类型的channel
4. Claude适配器的 `ConvertOpenAIRequest()` 被调用
5. 内部调用 `RequestOpenAI2ClaudeMessage()` 转换为Claude格式
6. 发送Claude格式请求到上游
7. 响应时，Claude适配器将响应转换回OpenAI格式返回客户端

**代码位置：**
- `relay/channel/claude/adaptor.go` - Claude适配器
- `relay/channel/claude/convert.go` - OpenAI → Claude转换逻辑
- `relay/channel/claude/handler.go` - Claude响应 → OpenAI响应转换

### 场景2: 客户端使用Claude格式访问OpenAI上游

**流程：**
1. 客户端发送请求到 `/v1/messages` (Claude格式)
2. 路由识别为 `RelayFormatClaude`
3. Distribute中间件选择OpenAI类型的channel
4. OpenAI适配器的 `ConvertClaudeRequest()` 被调用
5. 内部调用 `service.ClaudeToOpenAIRequest()` 转换为OpenAI格式
6. 发送OpenAI格式请求到上游
7. 响应时，OpenAI适配器将响应转换回Claude格式返回客户端

**代码位置：**
- `relay/channel/openai/adaptor.go` - OpenAI适配器的 `ConvertClaudeRequest()` 方法
- `service/convert.go` - `ClaudeToOpenAIRequest()` 函数

### 场景3: 客户端使用Gemini格式访问OpenAI上游

**流程：**
1. 客户端发送请求到 `/v1beta/models/{model}:generateContent` (Gemini格式)
2. 路由识别为 `RelayFormatGemini`
3. Distribute中间件选择OpenAI类型的channel
4. OpenAI适配器的 `ConvertGeminiRequest()` 被调用
5. 内部调用 `service.GeminiToOpenAIRequest()` 转换为OpenAI格式
6. 发送OpenAI格式请求到上游
7. 响应时转换回Gemini格式返回客户端

**代码位置：**
- `relay/channel/openai/adaptor.go` - OpenAI适配器的 `ConvertGeminiRequest()` 方法
- `service/convert.go` - `GeminiToOpenAIRequest()` 函数

### 场景4: 客户端使用OpenAI格式访问Gemini上游

**流程：**
1. 客户端发送请求到 `/v1/chat/completions` (OpenAI格式)
2. 路由识别为 `RelayFormatOpenAI`
3. Distribute中间件选择Gemini类型的channel
4. Gemini适配器的 `ConvertOpenAIRequest()` 被调用
5. 转换为Gemini原生格式
6. 发送Gemini格式请求到上游
7. 响应时转换回OpenAI格式返回客户端

**代码位置：**
- `relay/channel/gemini/adaptor.go` - Gemini适配器
- `relay/channel/gemini/convert.go` - OpenAI → Gemini转换逻辑

## 转换矩阵

| 客户端格式 ↓ / 上游格式 → | OpenAI | Claude | Gemini | 其他OpenAI兼容 |
|------------------------|--------|--------|--------|--------------|
| **OpenAI**             | ✅ 直通 | ✅ 支持 | ✅ 支持 | ✅ 支持 |
| **Claude**             | ✅ 支持 | ✅ 直通 | ❌ 不支持 | ❌ 不支持 |
| **Gemini**             | ✅ 支持 | ❌ 不支持 | ✅ 直通 | ❌ 不支持 |
| **Responses**          | ✅ 支持 | ❌ 不支持 | ❌ 不支持 | ❌ 不支持 |

## 关键代码文件

### 核心转换逻辑
- `service/convert.go` - 格式转换核心函数
  - `ClaudeToOpenAIRequest()` - Claude → OpenAI
  - `GeminiToOpenAIRequest()` - Gemini → OpenAI
  - 消息、工具调用、流式响应等转换

### 适配器实现
- `relay/channel/openai/adaptor.go` - OpenAI适配器（支持接收Claude/Gemini格式）
- `relay/channel/claude/adaptor.go` - Claude适配器（支持接收OpenAI格式）
- `relay/channel/claude/convert.go` - OpenAI → Claude转换
- `relay/channel/gemini/adaptor.go` - Gemini适配器（支持接收OpenAI格式）
- `relay/channel/gemini/convert.go` - OpenAI → Gemini转换

### 响应处理
- `relay/channel/openai/handler.go` - OpenAI响应处理
- `relay/channel/claude/handler.go` - Claude响应处理（转换为OpenAI格式）
- `relay/channel/gemini/handler.go` - Gemini响应处理（转换为OpenAI格式）

## 特殊格式支持

### 1. Responses API
OpenAI的Responses API是一种特殊格式，支持：
- `/v1/responses` - 标准Responses格式
- `/v1/responses/compact` - 紧凑格式

**特点：**
- 主要用于OpenAI上游
- Azure OpenAI也支持（需要特殊API版本）
- 其他provider不支持

### 2. Realtime API
WebSocket实时通信格式：
- `/v1/realtime` - WebSocket连接
- 支持实时语音对话

### 3. Task API
异步任务格式，用于视频生成等长时任务：
- `/suno/submit/:action` - Suno音乐生成
- 各种视频生成服务（Kling、Vidu、Sora等）

## 如何添加新的格式转换

### 步骤1: 定义新格式
在 `types/relay_format.go` 中添加新格式常量：
```go
const (
    RelayFormatYourNewFormat = "your_new_format"
)
```

### 步骤2: 添加路由
在 `router/relay-router.go` 中添加新端点：
```go
httpRouter.POST("/your/endpoint", func(c *gin.Context) {
    controller.Relay(c, types.RelayFormatYourNewFormat)
})
```

### 步骤3: 实现转换逻辑
在 `service/convert.go` 中添加转换函数：
```go
func YourFormatToOpenAIRequest(request YourFormatRequest) (*dto.GeneralOpenAIRequest, error) {
    // 实现转换逻辑
}

func OpenAIToYourFormatRequest(request dto.GeneralOpenAIRequest) (*YourFormatRequest, error) {
    // 实现转换逻辑
}
```

### 步骤4: 更新适配器
在相关适配器中实现转换方法：
```go
func (a *Adaptor) ConvertYourFormatRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.YourFormatRequest) (any, error) {
    return service.YourFormatToOpenAIRequest(*request)
}
```

### 步骤5: 实现响应转换
在handler中实现响应格式转换，将上游响应转换回客户端期望的格式。

## 注意事项

1. **格式兼容性**：不是所有格式之间都能完美转换，某些特性可能会丢失
2. **性能影响**：格式转换会增加一定的延迟和CPU开销
3. **流式响应**：流式响应的转换需要特别处理，确保实时性
4. **错误处理**：转换失败时需要返回清晰的错误信息
5. **测试覆盖**：每种转换场景都应该有对应的测试用例

## 实际应用场景

1. **统一客户端接口**：客户端只需实现OpenAI格式，即可访问所有provider
2. **Provider迁移**：从一个provider迁移到另一个时，无需修改客户端代码
3. **多provider负载均衡**：可以在不同格式的provider之间进行负载均衡
4. **格式兼容性测试**：测试不同provider对同一请求的响应差异

## 总结

new-api的格式转换机制非常灵活和强大，核心优势：

✅ **支持多种格式互转**：OpenAI ↔ Claude ↔ Gemini
✅ **透明转换**：客户端无感知，自动处理
✅ **可扩展**：易于添加新格式支持
✅ **完整支持**：包括流式响应、工具调用、多模态等高级特性

这使得new-api可以作为一个真正的"统一API网关"，屏蔽不同AI provider的API差异。
