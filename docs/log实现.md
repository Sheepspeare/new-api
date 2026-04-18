# 日志详情系统实现指南 (Log Detail System Implementation Guide)

本文档详细记录了 new-api 项目中请求/响应日志详情记录功能的完整实现，方便在其他项目中复现该功能。

---

## 📋 目录

1. [功能概述](#功能概述)
2. [系统架构](#系统架构)
3. [数据库层实现](#数据库层实现)
4. [后端实现](#后端实现)
5. [前端实现](#前端实现)
6. [配置管理](#配置管理)
7. [完整实现步骤](#完整实现步骤)
8. [测试验证](#测试验证)

---

## 功能概述

### 核心功能
日志详情系统用于记录和查看 AI API 请求的完整请求体和响应体，帮助管理员：
- 调试问题请求
- 审计 API 使用情况
- 分析模型响应质量
- 提取流式响应的文本内容

### 关键特性
✅ 异步记录，不影响主请求性能
✅ 支持流式和非流式响应
✅ 自动提取响应文本内容
✅ 内容大小限制和截断
✅ 自动清理过期数据
✅ 仅管理员可查看
✅ 支持 SQLite/MySQL/PostgreSQL

---

## 系统架构

### 数据流程图
```
客户端请求
    ↓
TokenAuth 中间件 (设置 request_id)
    ↓
MessageHook 中间件 (可选)
    ↓
Distribute 中间件 (选择渠道)
    ↓
Relay Handler (转发请求)
    ├─ 保存请求体到 context: log_detail_request_body
    ├─ 调用上游 API
    ├─ 处理响应 (流式/非流式)
    ├─ 保存响应体到 context: log_detail_response_body
    └─ 提取文本内容到 context: log_detail_extracted_content
    ↓
Quota 结算 (service/quota.go)
    └─ 异步调用 model.RecordLogDetail()
        └─ 写入 log_details 表
```

### 核心组件

| 组件 | 文件路径 | 作用 |
|------|---------|------|
| 数据模型 | `model/log_detail.go` | LogDetail 结构体、CRUD 操作 |
| 日志控制器 | `controller/log.go` | API 端点处理 |
| 配置常量 | `common/constants.go` | 全局配置变量 |
| 选项管理 | `model/option.go` | 配置持久化 |
| Relay 处理器 | `relay/channel/*/relay-*.go` | 捕获请求/响应 |
| 前端组件 | `web/src/components/LogDetailModal.jsx` | 详情展示 |
| 前端表格 | `web/src/components/table/usage-logs/` | 日志列表 |

---

## 数据库层实现

### 1. 数据表结构

**文件**: `model/log_detail.go`

```go
type LogDetail struct {
    Id                int    `json:"id" gorm:"primaryKey;autoIncrement"`
    RequestId         string `json:"request_id" gorm:"type:varchar(64);uniqueIndex:idx_log_details_request_id;not null"`
    RequestBody       string `json:"request_body" gorm:"type:mediumtext"`
    ResponseBody      string `json:"response_body" gorm:"type:mediumtext"`
    ExtractedContent  string `json:"extracted_content" gorm:"type:mediumtext"`
    CreatedAt         int64  `json:"created_at" gorm:"bigint;index"`
}

func (LogDetail) TableName() string {
    return "log_details"
}
```

**字段说明**:
- `request_id`: 关联 logs 表的唯一请求 ID (唯一索引)
- `request_body`: 完整请求体 JSON (mediumtext, 最大 16MB)
- `response_body`: 完整响应体 (流式为 SSE 格式，非流式为 JSON)
- `extracted_content`: 提取的纯文本内容 (从响应中提取)
- `created_at`: Unix 时间戳 (用于自动清理)

### 2. 数据库迁移

**文件**: `model/main.go`

```go
// 在 InitLogDB() 函数中添加
if err = LOG_DB.AutoMigrate(&LogDetail{}); err != nil {
    return err
}
```

### 3. CRUD 操作

**文件**: `model/log_detail.go`

#### 记录日志详情 (异步)
```go
func RecordLogDetail(requestId string, requestBody string, responseBody string, extractedContent string) {
    if !common.LogDetailEnabled || requestId == "" {
        return
    }

    maxSize := common.LogDetailMaxSize
    requestBody = truncateToMaxSize(requestBody, maxSize)
    responseBody = truncateToMaxSize(responseBody, maxSize)
    extractedContent = truncateToMaxSize(extractedContent, maxSize)

    gopool.Go(func() {
        detail := &LogDetail{
            RequestId:        requestId,
            RequestBody:      requestBody,
            ResponseBody:     responseBody,
            ExtractedContent: extractedContent,
            CreatedAt:        common.GetTimestamp(),
        }
        err := LOG_DB.Create(detail).Error
        if err != nil {
            common.SysLog("failed to record log detail: " + err.Error())
        }
    })
}
```

#### 查询日志详情
```go
func GetLogDetailByRequestId(requestId string) (*LogDetail, error) {
    var detail LogDetail
    err := LOG_DB.Where("request_id = ?", requestId).First(&detail).Error
    if err != nil {
        return nil, err
    }
    return &detail, nil
}
```

#### 删除过期数据 (支持三种数据库)
```go
func DeleteOldLogDetail(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
    var total int64 = 0
    for {
        if nil != ctx.Err() {
            return total, ctx.Err()
        }
        
        var result *gorm.DB
        if common.UsingPostgreSQL {
            // PostgreSQL: 使用子查询
            result = LOG_DB.Where("id IN (?)",
                LOG_DB.Model(&LogDetail{}).
                    Select("id").
                    Where("created_at < ?", targetTimestamp).
                    Limit(limit),
            ).Delete(&LogDetail{})
        } else {
            // MySQL/SQLite: 直接使用 LIMIT
            result = LOG_DB.Where("created_at < ?", targetTimestamp).Limit(limit).Delete(&LogDetail{})
        }
        
        if nil != result.Error {
            return total, result.Error
        }
        total += result.RowsAffected
        if result.RowsAffected < int64(limit) {
            break
        }
        
        // 添加延迟避免持续占用资源
        if total > 0 && result.RowsAffected == int64(limit) {
            select {
            case <-ctx.Done():
                return total, ctx.Err()
            case <-time.After(100 * time.Millisecond):
            }
        }
    }
    return total, nil
}
```

#### 内容截断函数 (UTF-8 安全)
```go
func truncateToMaxSize(s string, maxSize int) string {
    if maxSize <= 0 || len(s) <= maxSize {
        return s
    }
    truncated := s[:maxSize]
    // 确保不在 UTF-8 多字节字符中间截断
    for i := len(truncated) - 1; i >= len(truncated)-4 && i >= 0; i-- {
        if truncated[i]&0xC0 != 0x80 {
            r := truncated[i]
            var charLen int
            switch {
            case r&0x80 == 0:
                charLen = 1
            case r&0xE0 == 0xC0:
                charLen = 2
            case r&0xF0 == 0xE0:
                charLen = 3
            case r&0xF8 == 0xF0:
                charLen = 4
            default:
                charLen = 1
            }
            if i+charLen > len(truncated) {
                truncated = truncated[:i]
            }
            break
        }
    }
    return truncated + "\n...[内容已截断]"
}
```

---

## 后端实现

### 1. 配置常量

**文件**: `common/constants.go`

```go
var LogDetailEnabled = false             // 是否启用请求/响应详情记录
var LogDetailMaxSize = 128 * 1024        // 单条记录最大字节数，默认128KB
var LogDetailAutoCleanEnabled = false    // 是否启用自动清理
var LogDetailAutoCleanDays = 30          // 自动清理天数
```

### 2. 配置持久化

**文件**: `model/option.go`

#### 初始化配置映射
```go
func InitOptionMap() {
    common.OptionMapRWMutex.Lock()
    common.OptionMap = make(map[string]string)
    
    // ... 其他配置 ...
    
    common.OptionMap["LogDetailEnabled"] = strconv.FormatBool(common.LogDetailEnabled)
    common.OptionMap["LogDetailMaxSize"] = strconv.Itoa(common.LogDetailMaxSize)
    common.OptionMap["LogDetailAutoCleanEnabled"] = strconv.FormatBool(common.LogDetailAutoCleanEnabled)
    common.OptionMap["LogDetailAutoCleanDays"] = strconv.Itoa(common.LogDetailAutoCleanDays)
    
    // ... 其他配置 ...
}
```

#### 更新配置
```go
func updateConfig(key string, value string) {
    switch key {
    case "LogDetailEnabled":
        common.LogDetailEnabled, _ = strconv.ParseBool(value)
    case "LogDetailAutoCleanEnabled":
        common.LogDetailAutoCleanEnabled, _ = strconv.ParseBool(value)
    case "LogDetailMaxSize":
        common.LogDetailMaxSize, _ = strconv.Atoi(value)
    case "LogDetailAutoCleanDays":
        common.LogDetailAutoCleanDays, _ = strconv.Atoi(value)
    }
}
```

### 3. API 控制器

**文件**: `controller/log.go`

#### 获取日志详情
```go
func GetLogDetail(c *gin.Context) {
    requestId := c.Param("request_id")
    if requestId == "" {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": "request_id is required",
        })
        return
    }
    detail, err := model.GetLogDetailByRequestId(requestId)
    if err != nil {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": "未找到该请求的详情记录",
        })
        return
    }
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "",
        "data":    detail,
    })
}
```

#### 删除历史日志详情
```go
func DeleteHistoryLogDetails(c *gin.Context) {
    targetTimestamp, _ := strconv.ParseInt(c.Query("target_timestamp"), 10, 64)
    if targetTimestamp == 0 {
        c.JSON(http.StatusOK, gin.H{
            "success": false,
            "message": "target timestamp is required",
        })
        return
    }
    count, err := model.DeleteOldLogDetail(c.Request.Context(), targetTimestamp, 100)
    if err != nil {
        common.ApiError(c, err)
        return
    }
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "",
        "data":    count,
    })
}
```

### 4. API 路由

**文件**: `router/api-router.go`

```go
logRoute := api.Group("/log")
{
    logRoute.GET("/", middleware.AdminAuth(), controller.GetAllLogs)
    logRoute.DELETE("/", middleware.AdminAuth(), controller.DeleteHistoryLogs)
    logRoute.GET("/detail/:request_id", middleware.AdminAuth(), controller.GetLogDetail)
    logRoute.DELETE("/detail", middleware.AdminAuth(), controller.DeleteHistoryLogDetails)
    // ... 其他路由 ...
}
```

### 5. Relay 处理器中捕获请求/响应

#### OpenAI 流式响应处理

**文件**: `relay/channel/openai/relay-openai.go`

```go
func OaiStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (usage any, err *types.NewAPIError) {
    var responseBodyBuilder strings.Builder
    var responseTextBuilder strings.Builder
    
    // ... 流式处理逻辑 ...
    
    scanner := bufio.NewScanner(resp.Body)
    scanner.Split(bufio.ScanLines)
    
    for scanner.Scan() {
        data := scanner.Text()
        
        // 累积响应体用于日志详情
        if common.LogDetailEnabled {
            responseBodyBuilder.WriteString("data: ")
            responseBodyBuilder.WriteString(data)
            responseBodyBuilder.WriteString("\n\n")
        }
        
        // 解析并提取文本内容
        if strings.HasPrefix(data, "data: ") {
            data = strings.TrimPrefix(data, "data: ")
            if data == "[DONE]" {
                continue
            }
            var streamResponse dto.ChatCompletionsStreamResponse
            if err := common.Unmarshal([]byte(data), &streamResponse); err == nil {
                for _, choice := range streamResponse.Choices {
                    if choice.Delta.Content != "" {
                        responseTextBuilder.WriteString(choice.Delta.Content)
                    }
                }
            }
        }
        
        // ... 发送给客户端 ...
    }
    
    // 保存完整响应体到 context
    if common.LogDetailEnabled {
        c.Set("log_detail_response_body", responseBodyBuilder.String())
        c.Set("log_detail_extracted_content", responseTextBuilder.String())
    }
    
    return usage, nil
}
```

#### OpenAI 非流式响应处理

**文件**: `relay/channel/openai/relay-openai.go`

```go
func OpenaiHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (usage any, err *types.NewAPIError) {
    responseBody, err := io.ReadAll(resp.Body)
    
    // 保存响应体到 context
    if common.LogDetailEnabled {
        c.Set("log_detail_response_body", string(responseBody))
        
        // 提取文本内容
        var extractedText strings.Builder
        var response dto.OpenAITextResponse
        if err := common.Unmarshal(responseBody, &response); err == nil {
            for _, choice := range response.Choices {
                if choice.Message.StringContent() != "" {
                    extractedText.WriteString(choice.Message.StringContent())
                }
            }
        }
        if extractedText.Len() > 0 {
            c.Set("log_detail_extracted_content", extractedText.String())
        }
    }
    
    // ... 处理响应 ...
}
```

#### 保存请求体

**文件**: `relay/compatible_handler.go`

```go
func Handler(c *gin.Context, relayMode int, name string) *dto.OpenAIErrorWithStatusCode {
    // ... 解析请求 ...
    
    jsonData, err := common.Marshal(textRequest)
    
    // 保存请求体到 context
    if common.LogDetailEnabled {
        c.Set("log_detail_request_body", string(jsonData))
    }
    
    // ... 转发请求 ...
}
```

### 6. 在 Quota 结算时记录

**文件**: `service/quota.go` 或 `relay/compatible_handler.go`

```go
// 记录请求/响应详情
if common.LogDetailEnabled {
    requestId := ctx.GetString(common.RequestIdKey)
    if requestId != "" {
        reqBody, reqExists := ctx.Get("log_detail_request_body")
        respBody, respExists := ctx.Get("log_detail_response_body")
        extractedContent, extractedExists := ctx.Get("log_detail_extracted_content")
        
        var reqBodyStr, respBodyStr, extractedContentStr string
        if reqExists {
            reqBodyStr, _ = reqBody.(string)
        }
        if respExists {
            respBodyStr, _ = respBody.(string)
        }
        if extractedExists {
            extractedContentStr, _ = extractedContent.(string)
        }
        
        // 异步记录
        model.RecordLogDetail(requestId, reqBodyStr, respBodyStr, extractedContentStr)
    }
}
```

### 7. 自动清理任务

**文件**: `main.go`

```go
// 日志详情自动清理任务
if common.LogDetailAutoCleanEnabled {
    gopool.Go(func() {
        // 立即执行一次清理
        common.SysLog(fmt.Sprintf("log detail auto cleanup enabled, will clean logs older than %d days", common.LogDetailAutoCleanDays))
        targetTimestamp := time.Now().AddDate(0, 0, -common.LogDetailAutoCleanDays).Unix()
        deleted, err := model.DeleteOldLogDetail(context.Background(), targetTimestamp, 1000)
        if err != nil {
            common.SysError(fmt.Sprintf("initial auto clean log details failed: %v", err))
        } else {
            common.SysLog(fmt.Sprintf("initial auto clean log details success, deleted %d records", deleted))
        }
        
        // 定时清理 (每天一次)
        ticker := time.NewTicker(24 * time.Hour)
        defer ticker.Stop()
        for range ticker.C {
            if !common.LogDetailAutoCleanEnabled {
                continue
            }
            targetTimestamp := time.Now().AddDate(0, 0, -common.LogDetailAutoCleanDays).Unix()
            deleted, err := model.DeleteOldLogDetail(context.Background(), targetTimestamp, 1000)
            if err != nil {
                common.SysError(fmt.Sprintf("auto clean log details failed: %v", err))
            } else {
                common.SysLog(fmt.Sprintf("auto clean log details success, deleted %d records", deleted))
            }
        }
    })
}
```

### 8. 响应体日志模块化改造建议

这一块是值得单独抽模块的，而且建议先做“轻改造版”，不要一上来就把所有 relay handler 全部重写。

当前实现的问题不是“不能用”，而是有三个明显痛点：

1. 响应体日志采集入口分散在多个 handler 中，后续每加一种响应类型都要重复写一遍。
2. “记录原始上游响应”与“记录最终返回给客户端的响应”这两个概念混在一起，后续很容易越改越乱。
3. 文本提取逻辑跟具体 provider 强绑定，OpenAI、Claude、Gemini、Responses、Task 类响应的解析方式并不一致。

因此更合适的做法不是把日志逻辑塞进某一个公共函数，而是单独做一个“小型编排模块”，负责：

- 统一缓存 request/response 日志数据
- 统一做大小限制和截断
- 统一做文本提取分发
- 统一把结果回填到 `gin.Context`
- 尽量不接管原有业务 handler 的核心转发流程

#### 推荐模块边界

建议新增一个独立包，例如：

```text
relay/logcapture/
  context.go
  collector.go
  parser.go
  parser_openai.go
  parser_claude.go
  parser_gemini.go
  parser_generic.go
```

其中职责尽量拆开：

- `context.go`
  - 统一定义 `log_detail_request_body`
  - 统一定义 `log_detail_response_body`
  - 统一定义 `log_detail_extracted_content`
  - 对外提供 `SetRequestBody`、`FinalizeToContext` 之类的辅助函数
- `collector.go`
  - 负责字节累积、大小限制、流式分块缓存
  - 不关心具体 provider 语义
- `parser.go`
  - 定义统一解析接口
  - 只负责“如何从响应中提取文本/摘要”
- `parser_xxx.go`
  - 负责各 provider 或各响应模式的具体解析

这样做的好处是：业务 handler 仍然只管请求转换和转发，日志模块只管“旁路采集”和“解析提取”，耦合会明显比现在低。

#### 推荐先保留的数据语义

为了不改数据库结构，第一阶段建议继续沿用现有三字段语义：

- `request_body`
  - 发给上游前的最终请求体
- `response_body`
  - 最终返回给客户端的响应体
- `extracted_content`
  - 从响应体中提取出来的可读文本

这里有一个重要取舍：

- 先不要在第一阶段强行区分 `raw_upstream_body` 和 `client_response_body`
- 因为当前表结构只有一个 `response_body`
- 很多兼容模式下，上游响应和客户端响应本来就不是同一个格式

如果后面确实需要同时保留“上游原始响应”和“客户端最终响应”，建议第二阶段再扩表，不要在第一阶段把现有逻辑复杂化。

#### 核心接口示例

下面是一种比较稳妥的接口设计，核心思路是“采集器”和“解析器”分离：

```go
package logcapture

import (
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

type ParseResult struct {
	ResponseBody     string
	ExtractedContent string
	SkipStore        bool
}

type ResponseParser interface {
	Name() string
	Match(info *relaycommon.RelayInfo, contentType string) bool
	ParseResponse(info *relaycommon.RelayInfo, body []byte, contentType string) (*ParseResult, error)
}

type StreamParser interface {
	Name() string
	Match(info *relaycommon.RelayInfo, contentType string) bool
	ParseChunk(line string) (deltaText string, shouldKeep bool, err error)
}

type Collector struct {
	maxSize         int
	responseBuilder strings.Builder
	textBuilder     strings.Builder
}

func NewCollector(maxSize int) *Collector
func (c *Collector) AppendResponseChunk(chunk string)
func (c *Collector) AppendTextDelta(delta string)
func (c *Collector) Finalize() (responseBody string, extractedContent string)
```

这个设计的重点在于：

- `Collector` 只管收集，不关心这是 OpenAI 还是 Claude
- `ResponseParser` 只管解析，不关心数据最后怎么写入 `gin.Context`
- handler 最多只需要增加 1 到 2 行调用

#### 推荐接入方式

建议分两期。

#### 第一阶段：轻改造，优先落地

目标是把“日志采集代码”从各 handler 中抽掉一层，但不改变原来的响应处理主流程。

接入方式大致如下：

1. 请求体阶段
   - 在现有 `compatible_handler.go`、`audio_handler.go`、`responses_handler.go` 等入口，改为统一调用 `logcapture.SetRequestBody(c, bodyBytes)`。
2. 非流式响应阶段
   - 当前很多 handler 已经有 `io.ReadAll(resp.Body)`，读取后不直接 `c.Set(...)`
   - 改成调用 `logcapture.CaptureResponse(c, info, responseBody, contentType)`
3. 流式响应阶段
   - 保留原有 scanner/stream loop
   - 每个 chunk 处理完后调用 `collector.AppendResponseChunk(...)`
   - 文本增量统一交给 parser 或 handler 提供给 `collector.AppendTextDelta(...)`
4. 结算阶段
   - `service/quota.go` 和 `relay/compatible_handler.go` 继续只从 context 读最终结果
   - 不需要知道具体是哪种 provider

这一阶段的优点：

- 改动面可控
- 不需要改数据库
- 不需要一次性统一所有 relay handler 的实现风格
- 出问题时容易回滚

#### 第二阶段：深改造，可选

如果后面希望连“最终写给客户端的响应体”也统一捕获，可以再把 `relay/common/response_writer.go` 纳入进来，做成一个通用兜底层。

适合第二阶段做的事情：

- 所有非流式 JSON 返回尽量统一走 `LogResponseWriter`
- 所有 SSE 输出尽量统一通过包装 writer 采集
- 对未适配 parser 的渠道至少保留原始文本/事件流
- 必要时扩表保存 `upstream_response_body`

这一阶段复杂度会明显上升，因为它开始触碰“响应写回”链路，不再只是旁路记录。

#### 建议覆盖的响应场景

这个任务真正复杂的地方，不是“写一个模块”，而是“不同场景下 response body 到底长什么样”。

至少要分成下面几类：

1. 非流式 JSON 文本响应
   - 例如 `chat/completions`、`responses`、`claude messages`、`gemini generateContent`
   - 这类最容易做，读完整 body 后解析即可
2. 流式 SSE 响应
   - 例如 OpenAI/Claude/Gemini 的流式输出
   - 需要在 chunk 级别做累积和文本提取
3. 兼容转换响应
   - 例如上游走 `responses`，客户端拿到的是 `chat/completions`
   - 这类要明确 `response_body` 存哪一份，第一阶段建议存最终回给客户端的那一份
4. 图片/多模态响应
   - 可能返回 `url`、`b64_json`、tool call、图片生成元数据
   - 这类通常不适合提取纯文本，可只记录响应体，必要时提取摘要
5. 音频/二进制响应
   - 例如 `audio/speech`
   - 不建议直接保存原始二进制到 `response_body`
   - 建议记录占位信息，例如 `content-type`、大小、是否已省略
6. 异步 Task 提交/查询响应
   - 提交时通常返回任务 ID
   - 查询时返回状态、进度、结果 URL
   - 这类更像结构化 JSON，不适合复用聊天文本提取逻辑
7. 错误响应
   - 包括上游非 200、网关转换失败、SSE 中途断流
   - 如果希望日志体系完整，这部分后面也应该接入，而不是只记录成功路径

#### 建议的解析策略

不要试图做一个“万能 JSON 提取器”吃掉所有场景，这样很快会变成规则泥团。更稳妥的策略是“注册式解析器”：

```go
type ParserRegistry struct {
	responseParsers []ResponseParser
	streamParsers   []StreamParser
}

func (r *ParserRegistry) FindResponseParser(info *relaycommon.RelayInfo, contentType string) ResponseParser
func (r *ParserRegistry) FindStreamParser(info *relaycommon.RelayInfo, contentType string) StreamParser
```

推荐优先做下面几类 parser：

- `OpenAIChatParser`
- `OpenAIResponsesParser`
- `ClaudeParser`
- `GeminiParser`
- `GenericJSONParser`
- `BinaryResponseParser`
- `TaskJSONParser`

其中：

- `GenericJSONParser` 负责兜底，只记录 body，不强行提取复杂文本
- `BinaryResponseParser` 负责避免把音频/文件直接塞进数据库
- `TaskJSONParser` 只提取 `task_id`、`status`、`reason`、`url` 这类摘要信息

#### 对现有代码的最小侵入接法

如果目标是“尽量不与源码耦合得很紧”，那就不要让日志模块反向依赖太多业务 DTO。

比较合适的做法是：

1. 核心模块只依赖：
   - `gin.Context`
   - `relay/common.RelayInfo`
   - `common` 包里的配置与 JSON 包装函数
2. provider 特有 DTO 只放在各自 parser 文件中
3. handler 中只留下极薄的一层调用：

```go
if common.LogDetailEnabled {
	logcapture.CaptureResponse(c, info, responseBody, resp.Header.Get("Content-Type"))
}
```

流式场景则类似：

```go
collector := logcapture.NewCollector(common.LogDetailMaxSize)

for scanner.Scan() {
	line := scanner.Text()
	delta, keep, err := parser.ParseChunk(line)
	if keep {
		collector.AppendResponseChunk(line + "\n")
	}
	if delta != "" {
		collector.AppendTextDelta(delta)
	}
}

logcapture.FinalizeCollectorToContext(c, collector)
```

这样即使以后要换 parser 或增加新 provider，业务 handler 也不需要跟着改很多。

#### 复杂度评估

如果只做第一阶段，也就是：

- 抽出统一的 `logcapture` 模块
- 收敛 `c.Set(...)` 的写法
- 收敛响应体大小限制
- 收敛文本提取分发
- 先覆盖 OpenAI、Claude、Gemini、Responses、Task 几大类

那么复杂度我认为是“中等偏上”，不是特别简单，但完全可控。

复杂的地方主要有这几个：

1. 流式和非流式是两套处理模型
2. `responses` 与 `chat/completions` 存在互转场景
3. 图片、音频、任务类响应并不适合复用聊天提取逻辑
4. 错误路径现在并没有完全纳入同一条记录链路
5. 现有日志入库时机在 quota 结算附近，不是所有分支都天然会走到这里

但如果把目标控制在“先模块化、先统一入口、先覆盖主流文本渠道”，这件事并不会失控。

更直接地说：

- 先做模块化：复杂度中等
- 想一次性覆盖所有渠道且统一精准提取：复杂度偏高
- 想连错误链路、二进制响应、上游原始响应也一起做完整：复杂度高

#### 实施建议

建议按下面顺序推进：

1. 先做 `relay/logcapture` 模块骨架
2. 先接 OpenAI 非流式与流式
3. 再接 Claude、Gemini
4. 再补 `responses` 与 `chat via responses`
5. 再补 task、image、audio 这类非纯文本响应
6. 最后再考虑是否把 `LogResponseWriter` 纳入统一兜底层

如果只问结论：

- 可以单独抽模块
- 而且值得抽
- 但建议先做轻改造版本
- 多场景响应体判断解析不算简单，属于中等偏上复杂度
- 只要分阶段做，不需要把它看成一个特别重的大工程

---
## 前端实现

### 1. 日志详情模态框组件

**文件**: `web/src/components/LogDetailModal.jsx`

```jsx
import React, { useEffect, useState } from 'react';
import { Modal, Spin, Typography, Button, Toast } from '@douyinfe/semi-ui';
import { IconCopy } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, showError } from '../helpers';

const { Text } = Typography;

const LogDetailModal = ({ visible, onCancel, requestId }) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [detail, setDetail] = useState(null);

  useEffect(() => {
    if (visible && requestId) {
      fetchLogDetail();
    }
  }, [visible, requestId]);

  const fetchLogDetail = async () => {
    setLoading(true);
    try {
      const res = await API.get(`/api/log/detail/${requestId}`);
      const { success, message, data } = res.data;
      if (success) {
        setDetail(data);
      } else {
        showError(message || t('获取日志详情失败'));
      }
    } catch (error) {
      showError(t('获取日志详情失败：') + error.message);
    } finally {
      setLoading(false);
    }
  };

  const copyToClipboard = async (text, label) => {
    if (!text) {
      Toast.warning(t('内容为空，无法复制'));
      return;
    }

    // 尝试使用现代 Clipboard API
    if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
      try {
        await navigator.clipboard.writeText(text);
        Toast.success(t('已复制') + label);
        return;
      } catch (err) {
        console.warn('Clipboard API failed, falling back to execCommand:', err);
      }
    }

    // 降级方案：使用 execCommand
    try {
      const textArea = document.createElement('textarea');
      textArea.value = text;
      textArea.style.position = 'fixed';
      textArea.style.left = '-999999px';
      textArea.style.top = '-999999px';
      document.body.appendChild(textArea);
      textArea.focus();
      textArea.select();
      
      const successful = document.execCommand('copy');
      document.body.removeChild(textArea);
      
      if (successful) {
        Toast.success(t('已复制') + label);
      } else {
        Toast.error(t('复制失败'));
      }
    } catch (err) {
      Toast.error(t('复制失败：') + err.message);
    }
  };

  const formatJSON = (jsonString) => {
    if (!jsonString) return t('无内容');
    try {
      const obj = JSON.parse(jsonString);
      return JSON.stringify(obj, null, 2);
    } catch (e) {
      return jsonString;
    }
  };

  return (
    <Modal
      title={t('日志详情')}
      visible={visible}
      onCancel={onCancel}
      footer={null}
      width={900}
      style={{ maxWidth: '95vw' }}
      bodyStyle={{ maxHeight: '70vh', overflow: 'auto' }}
    >
      <Spin spinning={loading}>
        {detail ? (
          <div>
            {/* 请求内容 */}
            <div style={{ marginBottom: 20 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                <Text strong>{t('请求内容')}</Text>
                <Button icon={<IconCopy />} size='small' onClick={() => copyToClipboard(detail.request_body, t('请求内容'))}>
                  {t('复制')}
                </Button>
              </div>
              <pre style={{ background: '#f5f5f5', padding: 12, borderRadius: 4, overflow: 'auto', maxHeight: 300, fontSize: 12, lineHeight: 1.5 }}>
                {formatJSON(detail.request_body)}
              </pre>
            </div>

            {/* 提取的文本内容 */}
            {detail.extracted_content && (
              <div style={{ marginBottom: 20 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                  <Text strong style={{ color: '#1890ff' }}>{t('提取的文本内容')}</Text>
                  <Button icon={<IconCopy />} size='small' onClick={() => copyToClipboard(detail.extracted_content, t('提取的文本内容'))}>
                    {t('复制')}
                  </Button>
                </div>
                <pre style={{ background: '#e6f7ff', padding: 12, borderRadius: 4, overflow: 'auto', maxHeight: 300, fontSize: 12, lineHeight: 1.5, border: '1px solid #91d5ff' }}>
                  {detail.extracted_content || t('无内容')}
                </pre>
              </div>
            )}

            {/* 响应内容 */}
            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                <Text strong>{t('响应内容（原始）')}</Text>
                <Button icon={<IconCopy />} size='small' onClick={() => copyToClipboard(detail.response_body, t('响应内容'))}>
                  {t('复制')}
                </Button>
              </div>
              <pre style={{ background: '#f5f5f5', padding: 12, borderRadius: 4, overflow: 'auto', maxHeight: 300, fontSize: 12, lineHeight: 1.5 }}>
                {formatJSON(detail.response_body)}
              </pre>
            </div>
          </div>
        ) : (
          !loading && <Text type='tertiary'>{t('暂无数据')}</Text>
        )}
      </Spin>
    </Modal>
  );
};

export default LogDetailModal;
```

### 2. 日志表格列定义 - 添加详情按钮

**文件**: `web/src/components/table/usage-logs/UsageLogsColumnDefs.jsx`

在 `DETAILS` 列的 render 函数中添加：

```jsx
{
  key: COLUMN_KEYS.DETAILS,
  title: t('详情'),
  dataIndex: 'content',
  fixed: 'right',
  render: (text, record, index) => {
    // ... 现有的详情渲染逻辑 ...
    
    return (
      <div>
        <Typography.Paragraph ellipsis={{ rows: 3 }} style={{ maxWidth: 240, whiteSpace: 'pre-line', marginBottom: 8 }}>
          {content}
        </Typography.Paragraph>
        
        {/* 添加查看详情按钮 (仅管理员且有 request_id) */}
        {isAdminUser && record.request_id && onViewDetail && (
          <Button
            size='small'
            type='tertiary'
            onClick={(e) => {
              e.stopPropagation();
              onViewDetail(record.request_id);
            }}
          >
            {t('查看请求/响应')}
          </Button>
        )}
      </div>
    );
  },
}
```

### 3. 日志表格主组件 - 集成模态框

**文件**: `web/src/components/table/usage-logs/index.jsx`

```jsx
import LogDetailModal from '../../LogDetailModal';

const UsageLogsTable = () => {
  const logsData = useLogsData();

  return (
    <>
      {/* 其他模态框 */}
      <UserInfoModal {...logsData} />
      <ChannelAffinityUsageCacheModal {...logsData} />
      
      {/* 日志详情模态框 */}
      <LogDetailModal
        visible={logsData.showLogDetailModal}
        onCancel={() => logsData.setShowLogDetailModal(false)}
        requestId={logsData.logDetailRequestId}
      />

      {/* 表格组件 */}
      <CardPro>
        <LogsActions {...logsData} />
        <LogsFilters {...logsData} />
        <LogsTable {...logsData} />
      </CardPro>
    </>
  );
};
```

### 4. 日志数据 Hook - 添加状态管理

**文件**: `web/src/hooks/usage-logs/useUsageLogsData.jsx`

```jsx
export const useLogsData = () => {
  // ... 现有状态 ...
  
  // Log detail modal state (admin only)
  const [showLogDetailModal, setShowLogDetailModal] = useState(false);
  const [logDetailRequestId, setLogDetailRequestId] = useState(null);

  // Log detail function
  const handleViewLogDetail = (requestId) => {
    if (!isAdminUser) {
      return;
    }
    setLogDetailRequestId(requestId);
    setShowLogDetailModal(true);
  };

  return {
    // ... 现有返回值 ...
    
    // Log detail modal
    showLogDetailModal,
    setShowLogDetailModal,
    logDetailRequestId,
    handleViewLogDetail,
  };
};
```

### 5. 设置页面 - 日志详情配置

**文件**: `web/src/pages/Setting/Operation/SettingsLog.jsx`

```jsx
const SettingsLog = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [loadingCleanHistoryLogDetail, setLoadingCleanHistoryLogDetail] = useState(false);
  
  const [inputs, setInputs] = useState({
    LogConsumeEnabled: false,
    LogDetailEnabled: false,
    LogDetailMaxSize: 131072,
    LogDetailAutoCleanEnabled: false,
    LogDetailAutoCleanDays: 30,
    historyTimestamp: dayjs().subtract(1, 'month').toDate(),
    historyDetailTimestamp: dayjs().subtract(1, 'month').toDate(),
  });

  // 加载配置
  useEffect(() => {
    const loadOptions = async () => {
      const res = await API.get('/api/option/');
      const { success, data } = res.data;
      if (success) {
        setInputs({
          ...inputs,
          LogConsumeEnabled: data.LogConsumeEnabled === 'true',
          LogDetailEnabled: data.LogDetailEnabled === 'true',
          LogDetailMaxSize: parseInt(data.LogDetailMaxSize) || 131072,
          LogDetailAutoCleanEnabled: data.LogDetailAutoCleanEnabled === 'true',
          LogDetailAutoCleanDays: parseInt(data.LogDetailAutoCleanDays) || 30,
        });
      }
    };
    loadOptions();
  }, []);

  // 保存配置
  const onSubmit = async () => {
    setLoading(true);
    try {
      const res = await API.put('/api/option/', {
        LogConsumeEnabled: String(inputs.LogConsumeEnabled),
        LogDetailEnabled: String(inputs.LogDetailEnabled),
        LogDetailMaxSize: String(inputs.LogDetailMaxSize),
        LogDetailAutoCleanEnabled: String(inputs.LogDetailAutoCleanEnabled),
        LogDetailAutoCleanDays: String(inputs.LogDetailAutoCleanDays),
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('保存成功'));
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    } finally {
      setLoading(false);
    }
  };

  // 清除历史日志详情
  const onCleanHistoryLogDetail = async () => {
    if (!inputs.historyDetailTimestamp) {
      showError(t('请选择日志详情记录时间'));
      return;
    }
    
    Modal.confirm({
      title: t('确认清除'),
      content: t('确定要清除选定时间之前的所有日志详情吗？此操作不可恢复。'),
      onOk: async () => {
        try {
          setLoadingCleanHistoryLogDetail(true);
          const res = await API.delete(
            `/api/log/detail?target_timestamp=${Date.parse(inputs.historyDetailTimestamp) / 1000}`,
          );
          const { success, message, data } = res.data;
          if (success) {
            showSuccess(t('已清除 {count} 条日志详情记录', { count: data }));
          } else {
            showError(message);
          }
        } catch (error) {
          showError(error.message);
        } finally {
          setLoadingCleanHistoryLogDetail(false);
        }
      },
    });
  };

  return (
    <Form>
      <Row gutter={16}>
        {/* 启用请求/响应详情记录 */}
        <Col xs={24} sm={12} md={8} lg={8} xl={8}>
          <Form.Switch
            field={'LogDetailEnabled'}
            label={t('启用请求/响应详情记录')}
            size='default'
            checked={inputs.LogDetailEnabled}
            onChange={(value) => {
              setInputs({ ...inputs, LogDetailEnabled: value });
            }}
          />
        </Col>

        {/* 详情最大存储大小 */}
        <Col xs={24} sm={12} md={8} lg={8} xl={8}>
          <Form.InputNumber
            field={'LogDetailMaxSize'}
            label={t('详情最大存储大小（字节）')}
            placeholder={t('默认 131072 (128KB)')}
            value={inputs.LogDetailMaxSize}
            onChange={(value) => {
              setInputs({ ...inputs, LogDetailMaxSize: value });
            }}
          />
        </Col>

        {/* 启用详情自动清理 */}
        <Col xs={24} sm={12} md={8} lg={8} xl={8}>
          <Form.Switch
            field={'LogDetailAutoCleanEnabled'}
            label={t('启用详情自动清理')}
            size='default'
            checked={inputs.LogDetailAutoCleanEnabled}
            onChange={(value) => {
              setInputs({ ...inputs, LogDetailAutoCleanEnabled: value });
            }}
          />
        </Col>

        {/* 详情自动清理天数 */}
        <Col xs={24} sm={12} md={8} lg={8} xl={8}>
          <Form.InputNumber
            field={'LogDetailAutoCleanDays'}
            label={t('详情自动清理天数')}
            placeholder={t('默认 30 天')}
            value={inputs.LogDetailAutoCleanDays}
            onChange={(value) => {
              setInputs({ ...inputs, LogDetailAutoCleanDays: value });
            }}
          />
        </Col>

        {/* 清除历史日志详情 */}
        <Col xs={24} sm={12} md={8} lg={8} xl={8}>
          <Spin spinning={loadingCleanHistoryLogDetail}>
            <Form.DatePicker
              label={t('清除历史日志详情')}
              field='historyDetailTimestamp'
              value={inputs.historyDetailTimestamp}
              onChange={(value) => {
                setInputs({ ...inputs, historyDetailTimestamp: value });
              }}
            />
            <Button
              style={{ marginTop: 8 }}
              size='default'
              type='danger'
              onClick={onCleanHistoryLogDetail}
            >
              {t('清除历史日志详情')}
            </Button>
          </Spin>
        </Col>
      </Row>

      {/* 保存按钮 */}
      <Button loading={loading} onClick={onSubmit}>
        {t('保存')}
      </Button>
    </Form>
  );
};
```

### 6. 国际化翻译

**文件**: `web/src/i18n/locales/zh-CN.json`

添加以下翻译键：

```json
{
  "日志详情": "日志详情",
  "请求内容": "请求内容",
  "响应内容（原始）": "响应内容（原始）",
  "提取的文本内容": "提取的文本内容",
  "查看请求/响应": "查看请求/响应",
  "复制": "复制",
  "已复制": "已复制",
  "复制失败": "复制失败",
  "内容为空，无法复制": "内容为空，无法复制",
  "获取日志详情失败": "获取日志详情失败",
  "无内容": "无内容",
  "启用请求/响应详情记录": "启用请求/响应详情记录",
  "详情最大存储大小（字节）": "详情最大存储大小（字节）",
  "启用详情自动清理": "启用详情自动清理",
  "详情自动清理天数": "详情自动清理天数",
  "清除历史日志详情": "清除历史日志详情",
  "请选择日志详情记录时间": "请选择日志详情记录时间",
  "确定要清除选定时间之前的所有日志详情吗？此操作不可恢复。": "确定要清除选定时间之前的所有日志详情吗？此操作不可恢复。",
  "已清除 {count} 条日志详情记录": "已清除 {{count}} 条日志详情记录"
}
```

---
## 配置管理

### 环境变量支持 (可选)

虽然当前实现主要通过数据库配置，但也可以添加环境变量支持：

**文件**: `.env.example`

```bash
# 日志详情配置
# LOG_DETAIL_ENABLED=false
# LOG_DETAIL_MAX_SIZE=131072
# LOG_DETAIL_AUTO_CLEAN_ENABLED=false
# LOG_DETAIL_AUTO_CLEAN_DAYS=30
```

### 配置优先级

1. 数据库配置 (options 表) - 最高优先级
2. 环境变量 (如果实现)
3. 代码默认值

### 配置说明

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `LogDetailEnabled` | bool | false | 是否启用日志详情记录 |
| `LogDetailMaxSize` | int | 131072 (128KB) | 单条记录最大字节数 |
| `LogDetailAutoCleanEnabled` | bool | false | 是否启用自动清理 |
| `LogDetailAutoCleanDays` | int | 30 | 自动清理天数 |

---

## 完整实现步骤

### 步骤 1: 数据库层

1. **创建模型文件** `model/log_detail.go`
   - 定义 `LogDetail` 结构体
   - 实现 `RecordLogDetail()` 异步记录函数
   - 实现 `GetLogDetailByRequestId()` 查询函数
   - 实现 `DeleteOldLogDetail()` 清理函数
   - 实现 `truncateToMaxSize()` UTF-8 安全截断函数

2. **注册数据库迁移** `model/main.go`
   ```go
   if err = LOG_DB.AutoMigrate(&LogDetail{}); err != nil {
       return err
   }
   ```

### 步骤 2: 后端配置

1. **添加配置常量** `common/constants.go`
   ```go
   var LogDetailEnabled = false
   var LogDetailMaxSize = 128 * 1024
   var LogDetailAutoCleanEnabled = false
   var LogDetailAutoCleanDays = 30
   ```

2. **配置持久化** `model/option.go`
   - 在 `InitOptionMap()` 中添加配置映射
   - 在配置更新函数中处理新配置

### 步骤 3: API 层

1. **创建控制器方法** `controller/log.go`
   - `GetLogDetail()` - 获取日志详情
   - `DeleteHistoryLogDetails()` - 删除历史详情

2. **注册路由** `router/api-router.go`
   ```go
   logRoute.GET("/detail/:request_id", middleware.AdminAuth(), controller.GetLogDetail)
   logRoute.DELETE("/detail", middleware.AdminAuth(), controller.DeleteHistoryLogDetails)
   ```

### 步骤 4: Relay 处理器集成

需要在所有 relay 处理器中添加日志详情捕获逻辑：

1. **OpenAI 处理器** `relay/channel/openai/relay-openai.go`
   - 流式: `OaiStreamHandler()`
   - 非流式: `OpenaiHandler()`

2. **Claude 处理器** `relay/channel/claude/relay-claude.go`
   - 流式: `ClaudeStreamHandler()`
   - 非流式: `ClaudeHandler()`

3. **Gemini 处理器** `relay/channel/gemini/relay-gemini.go`
   - 流式: `GeminiStreamHandler()`
   - 非流式: `GeminiHandler()`

4. **通用处理器** `relay/compatible_handler.go`
   - 在请求转发前保存请求体
   - 在 quota 结算时调用 `RecordLogDetail()`

**关键代码模式**:

```go
// 1. 保存请求体
if common.LogDetailEnabled {
    c.Set("log_detail_request_body", string(jsonData))
}

// 2. 累积响应体 (流式)
if common.LogDetailEnabled {
    responseBodyBuilder.WriteString("data: " + data + "\n\n")
    responseTextBuilder.WriteString(extractedText)
}

// 3. 保存到 context
if common.LogDetailEnabled {
    c.Set("log_detail_response_body", responseBodyBuilder.String())
    c.Set("log_detail_extracted_content", responseTextBuilder.String())
}

// 4. 在 quota 结算时记录
if common.LogDetailEnabled {
    requestId := ctx.GetString(common.RequestIdKey)
    reqBody, _ := ctx.Get("log_detail_request_body")
    respBody, _ := ctx.Get("log_detail_response_body")
    extractedContent, _ := ctx.Get("log_detail_extracted_content")
    
    model.RecordLogDetail(requestId, reqBodyStr, respBodyStr, extractedContentStr)
}
```

### 步骤 5: 自动清理任务

**文件**: `main.go`

在主函数中添加自动清理 goroutine：

```go
if common.LogDetailAutoCleanEnabled {
    gopool.Go(func() {
        // 立即执行一次
        targetTimestamp := time.Now().AddDate(0, 0, -common.LogDetailAutoCleanDays).Unix()
        deleted, err := model.DeleteOldLogDetail(context.Background(), targetTimestamp, 1000)
        // ... 错误处理 ...
        
        // 定时清理
        ticker := time.NewTicker(24 * time.Hour)
        defer ticker.Stop()
        for range ticker.C {
            if !common.LogDetailAutoCleanEnabled {
                continue
            }
            // ... 清理逻辑 ...
        }
    })
}
```

### 步骤 6: 前端实现

1. **创建模态框组件** `web/src/components/LogDetailModal.jsx`
   - 实现详情展示
   - 实现复制功能
   - 实现 JSON 格式化

2. **修改日志表格列定义** `web/src/components/table/usage-logs/UsageLogsColumnDefs.jsx`
   - 在 DETAILS 列添加"查看请求/响应"按钮
   - 传递 `onViewDetail` 回调

3. **集成到日志表格** `web/src/components/table/usage-logs/index.jsx`
   - 导入 `LogDetailModal`
   - 传递状态和回调

4. **添加状态管理** `web/src/hooks/usage-logs/useUsageLogsData.jsx`
   - 添加 `showLogDetailModal` 状态
   - 添加 `logDetailRequestId` 状态
   - 实现 `handleViewLogDetail` 函数

5. **创建设置页面** `web/src/pages/Setting/Operation/SettingsLog.jsx`
   - 实现配置表单
   - 实现保存功能
   - 实现清理功能

6. **添加国际化** `web/src/i18n/locales/*.json`
   - 添加所有相关翻译键

### 步骤 7: 测试验证

见下一节

---

## 测试验证

### 1. 数据库测试

```sql
-- 检查表是否创建
SHOW TABLES LIKE 'log_details';

-- 检查表结构
DESC log_details;

-- 查询记录
SELECT * FROM log_details ORDER BY created_at DESC LIMIT 10;
```

### 2. 功能测试清单

#### 后端测试

- [ ] 启用日志详情记录功能
- [ ] 发送测试请求 (流式)
- [ ] 发送测试请求 (非流式)
- [ ] 检查 `log_details` 表是否有记录
- [ ] 验证 `request_body` 字段内容
- [ ] 验证 `response_body` 字段内容
- [ ] 验证 `extracted_content` 字段内容
- [ ] 测试内容截断 (发送超大请求)
- [ ] 测试 UTF-8 字符截断安全性
- [ ] 测试 API 端点: `GET /api/log/detail/:request_id`
- [ ] 测试自动清理功能
- [ ] 测试手动清理 API: `DELETE /api/log/detail?target_timestamp=xxx`

#### 前端测试

- [ ] 打开日志页面
- [ ] 查看日志列表中的"查看请求/响应"按钮
- [ ] 点击按钮打开详情模态框
- [ ] 验证请求内容显示
- [ ] 验证响应内容显示
- [ ] 验证提取的文本内容显示
- [ ] 测试复制功能 (请求内容)
- [ ] 测试复制功能 (响应内容)
- [ ] 测试复制功能 (提取的文本)
- [ ] 测试 JSON 格式化
- [ ] 打开设置页面
- [ ] 修改配置并保存
- [ ] 测试清除历史日志详情功能

#### 性能测试

- [ ] 测试异步记录不影响主请求性能
- [ ] 测试大量并发请求时的记录性能
- [ ] 测试自动清理对数据库的影响
- [ ] 监控数据库存储空间增长

#### 兼容性测试

- [ ] SQLite 数据库测试
- [ ] MySQL 数据库测试
- [ ] PostgreSQL 数据库测试
- [ ] 测试不同 provider (OpenAI, Claude, Gemini)
- [ ] 测试流式和非流式响应

### 3. 测试脚本示例

**测试请求记录**:

```bash
# 1. 启用日志详情
curl -X PUT http://localhost:3000/api/option/ \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"LogDetailEnabled": "true"}'

# 2. 发送测试请求
curl -X POST http://localhost:3000/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": false
  }'

# 3. 查询日志列表获取 request_id
curl http://localhost:3000/api/log/ \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN"

# 4. 查询日志详情
curl http://localhost:3000/api/log/detail/REQUEST_ID \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN"
```

**测试清理功能**:

```bash
# 清除 30 天前的日志详情
TIMESTAMP=$(date -d "30 days ago" +%s)
curl -X DELETE "http://localhost:3000/api/log/detail?target_timestamp=$TIMESTAMP" \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN"
```

### 4. 常见问题排查

| 问题 | 可能原因 | 解决方案 |
|------|---------|---------|
| 日志详情未记录 | `LogDetailEnabled` 未启用 | 检查配置，确保已启用 |
| 找不到详情记录 | `request_id` 不匹配 | 检查 logs 表和 log_details 表的 request_id |
| 内容被截断 | 超过 `LogDetailMaxSize` | 增加配置值或优化请求大小 |
| 响应内容为空 | Relay 处理器未集成 | 检查对应 provider 的处理器代码 |
| 提取的文本为空 | 响应格式不匹配 | 检查文本提取逻辑 |
| 数据库空间不足 | 未启用自动清理 | 启用自动清理或手动清理 |
| 前端按钮不显示 | 非管理员或无 request_id | 确认用户权限和日志记录 |

---

## 关键注意事项

### 1. 性能考虑

✅ **异步记录**: 使用 `gopool.Go()` 异步记录，不阻塞主请求
✅ **内容限制**: 通过 `LogDetailMaxSize` 限制单条记录大小
✅ **批量清理**: 清理时使用 LIMIT 分批删除，避免长时间锁表
✅ **索引优化**: `request_id` 唯一索引，`created_at` 普通索引

### 2. 安全考虑

🔒 **权限控制**: 仅管理员可查看日志详情
🔒 **敏感信息**: 可能包含 API Key 等敏感信息，需谨慎处理
🔒 **数据清理**: 定期清理避免敏感信息长期存储

### 3. 存储考虑

💾 **存储空间**: mediumtext 最大 16MB，需监控数据库大小
💾 **自动清理**: 建议启用自动清理，默认保留 30 天
💾 **备份策略**: 清理前考虑是否需要备份

### 4. 数据库兼容性

🗄️ **字段类型**: 使用 `mediumtext` 兼容三种数据库
🗄️ **删除逻辑**: PostgreSQL 不支持 DELETE...LIMIT，需使用子查询
🗄️ **索引策略**: 确保索引在三种数据库中都有效

### 5. 扩展建议

🚀 **压缩存储**: 对大内容使用 gzip 压缩
🚀 **对象存储**: 超大内容存储到 S3/OSS
🚀 **搜索功能**: 添加全文搜索支持
🚀 **导出功能**: 支持导出为 JSON/CSV
🚀 **统计分析**: 基于日志详情的统计分析

---

## 文件清单

### 后端文件

```
model/
  ├── log_detail.go          # 日志详情模型 (新建)
  ├── log.go                 # 日志模型 (已存在)
  ├── option.go              # 配置管理 (修改)
  └── main.go                # 数据库迁移 (修改)

controller/
  └── log.go                 # 日志控制器 (修改)

router/
  └── api-router.go          # API 路由 (修改)

common/
  └── constants.go           # 配置常量 (修改)

relay/
  ├── compatible_handler.go  # 通用处理器 (修改)
  └── channel/
      ├── openai/
      │   └── relay-openai.go    # OpenAI 处理器 (修改)
      ├── claude/
      │   └── relay-claude.go    # Claude 处理器 (修改)
      └── gemini/
          └── relay-gemini.go    # Gemini 处理器 (修改)

service/
  └── quota.go               # Quota 结算 (修改)

main.go                      # 主程序 (修改 - 添加自动清理)
```

### 前端文件

```
web/src/
  ├── components/
  │   ├── LogDetailModal.jsx                    # 日志详情模态框 (新建)
  │   └── table/
  │       └── usage-logs/
  │           ├── index.jsx                     # 日志表格主组件 (修改)
  │           ├── UsageLogsTable.jsx            # 表格组件 (已存在)
  │           └── UsageLogsColumnDefs.jsx       # 列定义 (修改)
  │
  ├── hooks/
  │   └── usage-logs/
  │       └── useUsageLogsData.jsx              # 数据 Hook (修改)
  │
  ├── pages/
  │   └── Setting/
  │       └── Operation/
  │           └── SettingsLog.jsx               # 日志设置页面 (修改)
  │
  └── i18n/
      └── locales/
          ├── zh-CN.json                        # 中文翻译 (修改)
          ├── en.json                           # 英文翻译 (修改)
          └── ...                               # 其他语言 (修改)
```

---

## 总结

本文档详细记录了 new-api 项目中日志详情系统的完整实现，包括：

✅ 数据库层设计和实现
✅ 后端 API 和配置管理
✅ Relay 处理器集成
✅ 前端组件和页面
✅ 自动清理机制
✅ 测试验证方法

通过本文档，您可以在其他项目中完整复现该功能，或对现有实现进行维护和扩展。

---

**文档版本**: 1.0
**最后更新**: 2026-03-17
**维护者**: QuantumNous Team
