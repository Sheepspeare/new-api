# new-api 消息钩子(Message Hook)机制完整实现文档

## 文档说明

本文档详细记录了 new-api 项目中消息钩子(Message Hook)功能的完整实现，包括前端、后端、数据库、中间件等所有相关代码和机制。

**目标用途**: 为原始无Hook机制的项目提供完整的实现参考，方便重新添加该功能。

**创建日期**: 2025年
**项目**: new-api (基于 OneAPI)
**维护者**: QuantumNous

---

## 目录

1. [功能概述](#功能概述)
2. [架构设计](#架构设计)
3. [数据库层](#数据库层)
4. [后端实现](#后端实现)
5. [前端实现](#前端实现)
6. [配置与环境变量](#配置与环境变量)
7. [使用示例](#使用示例)
8. [测试](#测试)
9. [部署注意事项](#部署注意事项)

---

## 功能概述

### 什么是消息钩子(Message Hook)?

消息钩子是一个中间件机制，允许在AI请求发送到上游模型之前对消息进行预处理。主要用途包括:

- **内容审核**: 过滤不当内容
- **提示词增强**: 自动添加系统提示或上下文
- **隐私保护**: 脱敏敏感信息
- **内容注入**: 添加搜索结果、知识库内容等
- **请求拦截**: 根据规则阻止某些请求

### 核心特性

1. **双执行器支持**:
   - Lua脚本执行器: 轻量级、沙箱化、高性能
   - HTTP执行器: 调用外部Webhook服务

2. **优先级调度**: 支持多个Hook按优先级顺序执行

3. **过滤器系统**: 
   - 用户过滤 (filter_users)
   - 模型过滤 (filter_models)
   - 令牌过滤 (filter_tokens)

4. **统计与监控**:
   - 调用次数统计
   - 成功率追踪
   - 平均执行时间

5. **安全机制**:
   - Lua沙箱: 禁用危险函数(io, os, debug等)
   - SSRF保护: HTTP请求防护
   - 超时控制: 防止长时间阻塞

---

## 架构设计

### 整体架构

\\\
Client Request
    ↓
Gin Router (relay-router.go)
    ↓
TokenAuth Middleware
    ↓
MessageHook Middleware ← 【核心入口】
    ↓
Distribute Middleware
    ↓
Relay Handler
    ↓
Upstream AI Provider
\\\

### 分层架构

\\\
┌─────────────────────────────────────────┐
│         Frontend (React)                │
│  - MessageHook管理页面                   │
│  - 创建/编辑/测试界面                     │
│  - 统计展示                              │
└─────────────────────────────────────────┘
                  ↓ HTTP API
┌─────────────────────────────────────────┐
│         Controller Layer                │
│  - message_hook.go                      │
│  - CRUD API处理                          │
└─────────────────────────────────────────┘
                  ↓
┌─────────────────────────────────────────┐
│         Service Layer                   │
│  - message_hook_service.go              │
│  - lua_executor.go                      │
│  - http_executor.go                     │
└─────────────────────────────────────────┘
                  ↓
┌─────────────────────────────────────────┐
│         Model Layer                     │
│  - message_hook.go (GORM)               │
│  - 数据库操作                            │
└─────────────────────────────────────────┘
                  ↓
┌─────────────────────────────────────────┐
│         Database                        │
│  - message_hooks表                       │
│  - SQLite/MySQL/PostgreSQL支持           │
└─────────────────────────────────────────┘
\\\

### 执行流程

\\\
1. 请求到达 → MessageHook中间件
2. 解析请求体 → 提取messages
3. 查询启用的Hooks → 按优先级排序
4. 遍历执行Hooks:
   a. 检查过滤条件
   b. 选择执行器(Lua/HTTP)
   c. 执行Hook
   d. 处理返回结果
5. 如果abort=true → 返回错误，终止请求
6. 如果modified=true → 替换请求体
7. 继续后续中间件
\\\

---

## 数据库层

### 表结构 (message_hooks)

位置: \model/message_hook.go\

\\\go
type MessageHook struct {
    Id          int    \json:"id" gorm:"primaryKey;autoIncrement"\
    Name        string \json:"name" gorm:"type:varchar(128);not null;uniqueIndex:idx_message_hook_name"\
    Description string \json:"description" gorm:"type:text"\
    Type        int    \json:"type" gorm:"type:int;not null;default:1;index:idx_message_hook_type"\
    Content     string \json:"content" gorm:"type:text"\
    Enabled     bool   \json:"enabled" gorm:"type:boolean;default:false;index:idx_message_hook_enabled"\
    Priority    int    \json:"priority" gorm:"type:int;default:0;index:idx_message_hook_priority"\
    Timeout     int    \json:"timeout" gorm:"type:int;default:5000"\
    
    // 过滤条件 (JSON格式存储为TEXT)
    FilterUsers  string \json:"filter_users" gorm:"type:text"\
    FilterModels string \json:"filter_models" gorm:"type:text"\
    FilterTokens string \json:"filter_tokens" gorm:"type:text"\
    
    // 统计字段
    CallCount    int64   \json:"call_count" gorm:"type:bigint;default:0"\
    SuccessCount int64   \json:"success_count" gorm:"type:bigint;default:0"\
    ErrorCount   int64   \json:"error_count" gorm:"type:bigint;default:0"\
    AvgDuration  float64 \json:"avg_duration" gorm:"type:double precision;default:0"\
    
    CreatedTime int64 \json:"created_time" gorm:"bigint;index:idx_message_hook_created_time"\
    UpdatedTime int64 \json:"updated_time" gorm:"bigint"\
}
\\\

### 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| id | int | 主键，自增 |
| name | varchar(128) | Hook名称，唯一索引 |
| description | text | Hook描述 |
| type | int | 类型: 1=Lua, 2=HTTP |
| content | text | Lua脚本或HTTP URL |
| enabled | boolean | 是否启用 |
| priority | int | 优先级(数字越小优先级越高) |
| timeout | int | 超时时间(毫秒) |
| filter_users | text | 用户ID过滤(JSON数组) |
| filter_models | text | 模型名称过滤(JSON数组) |
| filter_tokens | text | 令牌ID过滤(JSON数组) |
| call_count | bigint | 总调用次数 |
| success_count | bigint | 成功次数 |
| error_count | bigint | 失败次数 |
| avg_duration | double | 平均执行时间(毫秒) |
| created_time | bigint | 创建时间(Unix时间戳) |
| updated_time | bigint | 更新时间(Unix时间戳) |

### 数据库迁移

在 \model/main.go\ 的 \migrateDB()\ 函数中添加:

\\\go
err := DB.AutoMigrate(
    // ... 其他模型
    &MessageHook{},
)
\\\

### CRUD操作

\model/message_hook.go\ 提供的主要方法:

\\\go
// 查询
func GetMessageHook(id int) (*MessageHook, error)
func GetAllMessageHooks(page, pageSize int, enabled *bool) ([]*MessageHook, int64, error)
func GetEnabledMessageHooks() ([]*MessageHook, error)

// 创建
func CreateMessageHook(hook *MessageHook) error

// 更新
func UpdateMessageHook(hook *MessageHook) error

// 删除
func DeleteMessageHook(id int) error

// 统计更新
func UpdateMessageHookStats(hookId int, success bool, duration time.Duration) error
\\\

---

## 后端实现

### 1. 常量定义

位置: \constant/message_hook.go\

\\\go
// Hook类型
const (
    MessageHookTypeLua  = 1
    MessageHookTypeHTTP = 2
)

// 配置键
const (
    MessageHookDefaultTimeout = "MessageHookDefaultTimeout"
    MessageHookLuaPoolSize    = "MessageHookLuaPoolSize"
    MessageHookHTTPPoolSize   = "MessageHookHTTPPoolSize"
    MessageHookCacheTTL       = "MessageHookCacheTTL"
)

// 缓存键
const (
    MessageHookCacheKeyPrefix = "message_hook:"
    EnabledHooksCacheKey      = "message_hook:enabled_hooks"
)

// 上下文键
const (
    ContextKeyConversationId = "conversation_id"
)
\\\

### 2. DTO定义

位置: \dto/message_hook.go\

\\\go
// Hook输入
type HookInput struct {
    UserId         int       \json:"user_id"\
    ConversationId string    \json:"conversation_id,omitempty"\
    Messages       []Message \json:"messages"\
    Model          string    \json:"model"\
    TokenId        int       \json:"token_id,omitempty"\
}

// Hook输出
type HookOutput struct {
    Modified bool      \json:"modified"\
    Messages []Message \json:"messages,omitempty"\
    Abort    bool      \json:"abort"\
    Reason   string    \json:"reason,omitempty"\
}

// 验证函数
func ValidateHookInput(input *HookInput) error
func ValidateHookOutput(output *HookOutput) error
\\\

### 3. Lua执行器

位置: \service/lua_executor.go\

**核心特性**:
- 使用 \gopher-lua\ 库
- 状态池化(sync.Pool)提高性能
- 沙箱化: 禁用危险函数
- 超时控制

**沙箱限制**:
\\\go
// 禁用的全局函数
L.SetGlobal("dofile", lua.LNil)
L.SetGlobal("loadfile", lua.LNil)
L.SetGlobal("require", lua.LNil)
L.SetGlobal("load", lua.LNil)
L.SetGlobal("loadstring", lua.LNil)

// 禁用的模块
L.SetGlobal("os", lua.LNil)
L.SetGlobal("io", lua.LNil)
L.SetGlobal("package", lua.LNil)
L.SetGlobal("debug", lua.LNil)
\\\

**允许的库**:
- base (基础函数)
- table (表操作)
- string (字符串操作)
- math (数学函数)

**执行流程**:
\\\go
func (e *luaExecutor) Execute(script string, input *dto.HookInput, timeout time.Duration) (*dto.HookOutput, error) {
    // 1. 从池中获取Lua状态
    L := e.pool.Get().(*lua.LState)
    defer e.pool.Put(L)
    
    // 2. 设置超时
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    
    // 3. 转换输入为Lua表
    inputTable := convertInputToLuaTable(L, input)
    L.SetGlobal("input", inputTable)
    
    // 4. 执行脚本
    err := L.DoString(script)
    
    // 5. 获取输出
    outputValue := L.GetGlobal("output")
    output := convertLuaTableToOutput(L, outputValue)
    
    return output, nil
}
\\\

### 4. HTTP执行器

位置: \service/http_executor.go\

**核心特性**:
- 连接池化
- SSRF保护
- 超时控制
- 支持自签名证书

**SSRF保护机制**:

\\\go
// 环境变量配置
MESSAGE_HOOK_DISABLE_SSRF_PROTECTION=false  // 是否禁用SSRF保护
MESSAGE_HOOK_ALLOW_HTTP=false               // 是否允许HTTP协议
MESSAGE_HOOK_ALLOWED_HOSTS=localhost,127.0.0.1  // 白名单主机

// 默认白名单端口: 55566-55569 (无需配置)
\\\

**URL验证逻辑**:
1. 检查协议(HTTPS/HTTP)
2. 解析主机名
3. 解析IP地址
4. 检查是否为私有IP
5. 如果是私有IP:
   - 检查端口是否在55566-55569范围
   - 检查主机是否在白名单
6. 公网IP要求HTTPS(除非允许HTTP)

**执行流程**:
\\\go
func (e *httpExecutor) Execute(hookURL string, input *dto.HookInput, timeout time.Duration) (*dto.HookOutput, error) {
    // 1. 验证URL
    validateHookURL(hookURL)
    
    // 2. 序列化输入
    jsonData := common.Marshal(input)
    
    // 3. 创建HTTP请求
    req := http.NewRequestWithContext(ctx, "POST", hookURL, bytes.NewReader(jsonData))
    req.Header.Set("Content-Type", "application/json")
    
    // 4. 执行请求
    resp := e.client.Do(req)
    
    // 5. 解析响应
    output := &dto.HookOutput{}
    common.Unmarshal(body, output)
    
    return output, nil
}
\\\


### 5. Service层

位置: \service/message_hook_service.go\

**接口定义**:
\\\go
type MessageHookService interface {
    // CRUD
    CreateHook(hook *model.MessageHook) error
    UpdateHook(hook *model.MessageHook) error
    DeleteHook(id int) error
    GetHook(id int) (*model.MessageHook, error)
    ListHooks(page, pageSize int, enabled *bool) ([]*model.MessageHook, int64, error)
    
    // 执行
    GetEnabledHooks() ([]*model.MessageHook, error)
    ExecuteHooks(hooks []*model.MessageHook, input *dto.HookInput) (*dto.HookOutput, error)
    
    // 统计
    UpdateHookStats(hookId int, success bool, duration time.Duration)
    
    // 测试
    TestHook(hook *model.MessageHook, input *dto.HookInput) (*dto.HookOutput, error)
    
    // 缓存
    InvalidateCache() error
}
\\\

**核心方法 - ExecuteHooks**:
\\\go
func (s *messageHookService) ExecuteHooks(hooks []*model.MessageHook, input *dto.HookInput) (*dto.HookOutput, error) {
    currentMessages := input.Messages
    anyModified := false
    
    // 按优先级顺序执行
    for _, hook := range hooks {
        // 检查过滤器
        if !s.matchesFilters(hook, input) {
            continue
        }
        
        // 更新输入
        input.Messages = currentMessages
        
        // 执行Hook
        startTime := time.Now()
        output, err := s.executeHook(hook, input)
        duration := time.Since(startTime)
        
        // 异步更新统计
        go s.UpdateHookStats(hook.Id, err == nil && !output.Abort, duration)
        
        if err != nil {
            // 优雅降级: 继续执行下一个Hook
            continue
        }
        
        // 处理中止
        if output.Abort {
            return output, nil
        }
        
        // 处理修改
        if output.Modified && len(output.Messages) > 0 {
            anyModified = true
            currentMessages = output.Messages
        }
    }
    
    return &dto.HookOutput{
        Modified: anyModified,
        Messages: currentMessages,
        Abort:    false,
    }, nil
}
\\\

**过滤器匹配**:
\\\go
func (s *messageHookService) matchesFilters(hook *model.MessageHook, input *dto.HookInput) bool {
    // 用户过滤
    if hook.FilterUsers != "" {
        var users []int
        common.Unmarshal([]byte(hook.FilterUsers), &users)
        if !contains(users, input.UserId) {
            return false
        }
    }
    
    // 模型过滤
    if hook.FilterModels != "" {
        var models []string
        common.Unmarshal([]byte(hook.FilterModels), &models)
        if !containsString(models, input.Model) {
            return false
        }
    }
    
    // 令牌过滤
    if hook.FilterTokens != "" {
        var tokens []int
        common.Unmarshal([]byte(hook.FilterTokens), &tokens)
        if !contains(tokens, input.TokenId) {
            return false
        }
    }
    
    return true
}
\\\

### 6. Controller层

位置: \controller/message_hook.go\

**API端点**:
\\\go
// GET /api/message-hooks - 获取Hook列表
func GetMessageHooks(c *gin.Context)

// GET /api/message-hooks/:id - 获取单个Hook
func GetMessageHook(c *gin.Context)

// POST /api/message-hooks - 创建Hook
func CreateMessageHook(c *gin.Context)

// PUT /api/message-hooks/:id - 更新Hook
func UpdateMessageHook(c *gin.Context)

// DELETE /api/message-hooks/:id - 删除Hook
func DeleteMessageHook(c *gin.Context)

// GET /api/message-hooks/:id/stats - 获取统计信息
func GetMessageHookStats(c *gin.Context)

// POST /api/message-hooks/test - 测试Hook
func TestMessageHook(c *gin.Context)
\\\

**示例 - 创建Hook**:
\\\go
func CreateMessageHook(c *gin.Context) {
    // 检查管理员权限
    if !isAdmin(c) {
        c.JSON(http.StatusForbidden, gin.H{
            "success": false,
            "message": "Admin access required",
        })
        return
    }
    
    // 解析请求
    var hook model.MessageHook
    if err := c.ShouldBindJSON(&hook); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "success": false,
            "message": fmt.Sprintf("Invalid request body: %v", err),
        })
        return
    }
    
    // 创建Hook
    if err := messageHookService.CreateHook(&hook); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "success": false,
            "message": fmt.Sprintf("Failed to create hook: %v", err),
        })
        return
    }
    
    // 记录日志
    userId := c.GetInt("id")
    common.SysLog(fmt.Sprintf("User %d created message hook: %s (id: %d)", userId, hook.Name, hook.Id))
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "data":    hook,
        "message": "Hook created successfully",
    })
}
\\\

### 7. 中间件

位置: \middleware/message_hook.go\

**注册位置**: \outer/relay-router.go\

\\\go
httpRouter := relayV1Router.Group("")
httpRouter.Use(middleware.MessageHook())  // ← 在这里注册
httpRouter.Use(middleware.Distribute())
\\\

**中间件实现**:
\\\go
func MessageHook() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 1. 初始化服务
        initMessageHookService()
        
        // 2. 检查是否为聊天完成请求
        if !isChatCompletionRequest(c) {
            c.Next()
            return
        }
        
        // 3. 获取用户ID
        userId := c.GetInt("id")
        if userId == 0 {
            c.Next()
            return
        }
        
        // 4. 解析请求体
        var request dto.GeneralOpenAIRequest
        err := common.UnmarshalBodyReusable(c, &request)
        if err != nil || len(request.Messages) == 0 {
            c.Next()
            return
        }
        
        // 5. 构建Hook输入
        input := &dto.HookInput{
            UserId:         userId,
            ConversationId: c.GetString(constant.ContextKeyConversationId),
            Messages:       request.Messages,
            Model:          request.Model,
            TokenId:        c.GetInt(string(constant.ContextKeyTokenId)),
        }
        
        // 6. 获取启用的Hooks
        hooks, err := messageHookService.GetEnabledHooks()
        if err != nil || len(hooks) == 0 {
            c.Next()
            return
        }
        
        // 7. 执行Hooks
        output, err := messageHookService.ExecuteHooks(hooks, input)
        if err != nil {
            // 优雅降级
            c.Next()
            return
        }
        
        // 8. 处理中止
        if output.Abort {
            reason := output.Reason
            if reason == "" {
                reason = "Request aborted by message hook"
            }
            c.JSON(http.StatusBadRequest, gin.H{
                "error": gin.H{
                    "message": reason,
                    "type":    "hook_abort",
                    "code":    "hook_abort",
                },
            })
            c.Abort()
            return
        }
        
        // 9. 处理修改
        if output.Modified && len(output.Messages) > 0 {
            request.Messages = output.Messages
            
            // 重新序列化
            modifiedBody, err := common.Marshal(request)
            if err != nil {
                c.Next()
                return
            }
            
            // 替换请求体
            newStorage, _ := common.CreateBodyStorage(modifiedBody)
            c.Set(common.KeyBodyStorage, newStorage)
            c.Request.ContentLength = int64(len(modifiedBody))
            c.Request.Body = io.NopCloser(bytes.NewReader(modifiedBody))
        }
        
        c.Next()
    }
}
\\\

**关键点**:
1. 使用 \common.UnmarshalBodyReusable\ 可重用地读取请求体
2. 使用 \common.CreateBodyStorage\ 创建新的请求体存储
3. 修改后需要更新 \c.Request.Body\ 和 \ContentLength\

### 8. 路由配置

位置: \outer/api-router.go\

\\\go
// Message Hook routes (admin only)
messageHookRoute := apiRouter.Group("/message-hooks")
messageHookRoute.Use(middleware.AdminAuth())
{
    messageHookRoute.GET("/", controller.GetMessageHooks)
    messageHookRoute.GET("/:id", controller.GetMessageHook)
    messageHookRoute.POST("/", controller.CreateMessageHook)
    messageHookRoute.PUT("/:id", controller.UpdateMessageHook)
    messageHookRoute.DELETE("/:id", controller.DeleteMessageHook)
    messageHookRoute.GET("/:id/stats", controller.GetMessageHookStats)
    messageHookRoute.POST("/test", controller.TestMessageHook)
}
\\\

---

## 前端实现

### 1. 页面结构

\\\
web/src/pages/MessageHook/
├── index.jsx              # Hook列表页面
├── EditMessageHook.jsx    # 创建/编辑页面
├── TestMessageHook.jsx    # 测试页面
└── MessageHook.css        # 样式文件

web/src/components/table/message-hooks/
└── index.jsx              # Hook表格组件

web/src/pages/Setting/Operation/
└── SettingsMessageHook.jsx  # 设置页面
\\\

### 2. 路由配置

位置: \web/src/App.jsx\

\\\jsx
<Route
  path='/console/message-hooks'
  element={
    <AdminRoute>
      <MessageHook />
    </AdminRoute>
  }
/>
<Route
  path='/console/message-hooks/create'
  element={
    <AdminRoute>
      <EditMessageHook />
    </AdminRoute>
  }
/>
<Route
  path='/console/message-hooks/:id/edit'
  element={
    <AdminRoute>
      <EditMessageHook />
    </AdminRoute>
  }
/>
<Route
  path='/console/message-hooks/test'
  element={
    <AdminRoute>
      <TestMessageHook />
    </AdminRoute>
  }
/>
\\\


### 3. Hook列表页面

位置: \web/src/components/table/message-hooks/index.jsx\

**核心功能**:
- 显示Hook列表(表格)
- 启用/禁用切换
- 编辑/删除/测试操作
- 显示统计信息(调用次数、成功率、平均耗时)

**关键代码**:
\\\jsx
const MessageHooksTable = () => {
  const [hooks, setHooks] = useState([]);
  const [pagination, setPagination] = useState({
    currentPage: 1,
    pageSize: 10,
    total: 0,
  });

  const loadHooks = async (page = 1) => {
    const res = await API.get(\/api/message-hooks?page=\&page_size=\\);
    if (res.data.success) {
      setHooks(res.data.data || []);
      setPagination({
        ...pagination,
        currentPage: page,
        total: res.data.total || 0,
      });
    }
  };

  const handleToggleEnabled = async (hook) => {
    const updatedHook = { ...hook, enabled: !hook.enabled };
    const res = await API.put(\/api/message-hooks/\\, updatedHook);
    if (res.data.success) {
      loadHooks(pagination.currentPage);
    }
  };

  const columns = [
    { title: '名称', dataIndex: 'name' },
    { title: '类型', dataIndex: 'type', render: (type) => type === 1 ? 'Lua' : 'HTTP' },
    { title: '优先级', dataIndex: 'priority' },
    { title: '调用次数', dataIndex: 'call_count' },
    { 
      title: '成功率', 
      render: (_, record) => {
        if (record.call_count === 0) return '-';
        const rate = ((record.success_count / record.call_count) * 100).toFixed(1);
        return \\%\;
      }
    },
    { title: '平均耗时(ms)', dataIndex: 'avg_duration' },
    { 
      title: '状态', 
      render: (_, record) => (
        <Switch checked={record.enabled} onChange={() => handleToggleEnabled(record)} />
      )
    },
    { title: '操作', render: (_, record) => (/* 编辑/测试/删除按钮 */) },
  ];

  return <Table columns={columns} dataSource={hooks} pagination={pagination} />;
};
\\\

### 4. 创建/编辑页面

位置: \web/src/pages/MessageHook/EditMessageHook.jsx\

**核心功能**:
- 表单验证
- Lua/HTTP类型切换
- 模板提供
- 内联帮助文档

**Lua脚本模板**:
\\\lua
-- 获取输入的消息数组
local messages = input.messages

-- 检查消息数组是否为空
if not messages or #messages == 0 then
    output = {
        modified = false,
        messages = {},
        abort = true,
        reason = "消息数组为空"
    }
    return
end

-- 示例：添加系统提示
local system_prompt = {
    role = "system",
    content = "你是一个专业的AI助手，请提供准确和有用的回答。"
}

-- 检查是否已有系统消息
local has_system = false
for i, message in ipairs(messages) do
    if message.role == "system" then
        has_system = true
        break
    end
end

-- 如果没有系统消息，添加一个
if not has_system then
    table.insert(messages, 1, system_prompt)
    output = {
        modified = true,
        messages = messages,
        abort = false
    }
else
    output = {
        modified = false,
        messages = messages,
        abort = false
    }
end
\\\

**HTTP URL模板**:
\\\
https://your-api-server.com/webhook/message-hook
\\\

**表单字段**:
\\\jsx
<Form onSubmit={handleSubmit}>
  <Form.Input field='name' label='名称' required />
  <Form.TextArea field='description' label='描述' />
  <Form.Select field='type' label='类型' required>
    <Select.Option value={1}>Lua</Select.Option>
    <Select.Option value={2}>HTTP</Select.Option>
  </Form.Select>
  
  {/* 帮助文档 */}
  <div style={{ marginBottom: 16, padding: 12, backgroundColor: 'var(--semi-color-fill-0)' }}>
    <Text>{selectedType === 1 ? LUA_HELP_TEXT : HTTP_HELP_TEXT}</Text>
  </div>
  
  <Form.TextArea field='content' label='内容' required />
  <Form.Switch field='enabled' label='启用' />
  <Form.InputNumber field='priority' label='优先级' min={0} max={1000} />
  <Form.InputNumber field='timeout' label='超时时间(ms)' min={100} max={30000} />
  <Form.TextArea field='filter_users' label='用户过滤' placeholder='JSON数组，例如: [1,2,3]' />
  <Form.TextArea field='filter_models' label='模型过滤' placeholder='JSON数组，例如: ["gpt-4","claude-3"]' />
  <Form.TextArea field='filter_tokens' label='令牌过滤' placeholder='JSON数组，例如: [1,2,3]' />
  
  <Button htmlType='submit' type='primary'>创建/更新</Button>
</Form>
\\\

### 5. 测试页面

位置: \web/src/pages/MessageHook/TestMessageHook.jsx\

**核心功能**:
- 选择已有Hook或自定义Hook配置
- 提供测试输入模板
- 实时显示测试结果
- 参数说明

**测试输入模板**:
\\\json
{
  "user_id": 1,
  "conversation_id": "test-conversation-123",
  "messages": [
    {
      "role": "user",
      "content": "你好，请介绍一下你自己"
    }
  ],
  "model": "gpt-4",
  "token_id": 1
}
\\\

**测试API调用**:
\\\jsx
const handleTest = async () => {
  const hook = JSON.parse(hookConfig);
  hook.content = hookContent;
  const input = JSON.parse(testInput);

  const res = await API.post('/api/message-hooks/test', {
    hook,
    input,
  });

  if (res.data.success) {
    setResult({
      success: true,
      output: res.data.data,
    });
  } else {
    setResult({
      success: false,
      error: res.data.message,
    });
  }
};
\\\

### 6. 国际化

位置: \web/src/i18n/locales/zh-CN.json\

**需要添加的翻译键**:
\\\json
{
  "消息钩子": "消息钩子",
  "创建钩子": "创建钩子",
  "编辑消息钩子": "编辑消息钩子",
  "创建消息钩子": "创建消息钩子",
  "测试消息钩子": "测试消息钩子",
  "名称": "名称",
  "描述": "描述",
  "类型": "类型",
  "内容": "内容",
  "启用": "启用",
  "优先级": "优先级",
  "超时时间(ms)": "超时时间(ms)",
  "用户过滤": "用户过滤",
  "模型过滤": "模型过滤",
  "令牌过滤": "令牌过滤",
  "调用次数": "调用次数",
  "成功率": "成功率",
  "平均耗时(ms)": "平均耗时(ms)",
  "状态": "状态",
  "操作": "操作",
  "确定删除此钩子吗？": "确定删除此钩子吗？",
  "钩子创建成功": "钩子创建成功",
  "钩子已成功创建！是否要立即测试钩子的运行效果？": "钩子已成功创建！是否要立即测试钩子的运行效果？",
  "立即测试": "立即测试",
  "稍后测试": "稍后测试"
}
\\\

---

## 配置与环境变量

### 后端环境变量

\\\ash
# Message Hook SSRF保护配置
MESSAGE_HOOK_DISABLE_SSRF_PROTECTION=false  # 是否完全禁用SSRF保护
MESSAGE_HOOK_ALLOW_HTTP=false               # 是否允许HTTP协议(默认只允许HTTPS)
MESSAGE_HOOK_ALLOWED_HOSTS=localhost,127.0.0.1,192.168.1.100  # 白名单主机(逗号分隔)

# 默认白名单端口: 55566-55569 (无需配置，系统自动允许)
\\\

### 系统配置选项

在 \model/option.go\ 中添加:

\\\go
// Message Hook配置
MessageHookDefaultTimeout = "MessageHookDefaultTimeout"  // 默认超时时间(毫秒)
MessageHookCacheTTL       = "MessageHookCacheTTL"        // 缓存TTL(秒)
\\\

在 \common/init.go\ 中初始化:

\\\go
// Message Hook配置
MessageHookDefaultTimeout = GetEnvOrDefault("MESSAGE_HOOK_DEFAULT_TIMEOUT", 5000)
MessageHookCacheTTL       = GetEnvOrDefault("MESSAGE_HOOK_CACHE_TTL", 60)
\\\

### Redis缓存

**缓存键**:
- \message_hook:enabled_hooks\ - 启用的Hook列表

**TTL**: 默认60秒

**缓存失效**:
- 创建Hook时
- 更新Hook时
- 删除Hook时

---

## 使用示例

### 示例1: 内容审核 (Lua)

\\\lua
-- Content Moderation Hook
local input = input
local output = {
    modified = false,
    messages = {},
    abort = false,
    reason = ""
}

-- 敏感词列表
local blocked_keywords = {
    "hack",
    "exploit",
    "malware",
    "virus"
}

-- 检查所有消息
for i, msg in ipairs(input.messages) do
    local content_lower = string.lower(msg.content)
    
    for j, keyword in ipairs(blocked_keywords) do
        if string.find(content_lower, keyword, 1, true) then
            output.abort = true
            output.reason = "内容违规：检测到不当内容"
            _G.output = output
            return
        end
    end
end

-- 未发现违规，继续
output.messages = input.messages
_G.output = output
\\\

### 示例2: 上下文注入 (Lua)

\\\lua
-- Context Injection Hook
local input = input
local output = {
    modified = true,
    messages = {},
    abort = false
}

-- 添加系统上下文
local system_context = {
    role = "system",
    content = "你是一个专业的AI助手。请简洁准确地回答问题。"
}

-- 检查是否已有系统消息
local has_system = false
for i, msg in ipairs(input.messages) do
    if msg.role == "system" then
        has_system = true
        break
    end
end

-- 添加系统消息
if not has_system then
    table.insert(output.messages, system_context)
end

-- 添加原始消息
for i, msg in ipairs(input.messages) do
    table.insert(output.messages, msg)
end

-- 为特定用户添加额外上下文
if input.user_id == 1 then
    table.insert(output.messages, 1, {
        role = "system",
        content = "注意：此用户偏好技术性解释。"
    })
end

_G.output = output
\\\

### 示例3: 提示词增强 (Lua)

\\\lua
-- Prompt Enhancement Hook
local input = input
local output = {
    modified = false,
    messages = {},
    abort = false
}

-- 增强函数
local function enhance_prompt(content)
    if string.find(content, "%?") then
        return "请详细准确地回答以下问题：" .. content
    end
    
    if string.find(string.lower(content), "code") or 
       string.find(string.lower(content), "代码") then
        return "请提供带注释的代码示例：" .. content
    end
    
    return "请提供有帮助的全面回答：" .. content
end

-- 处理每条消息
for i, msg in ipairs(input.messages) do
    if msg.role == "user" then
        local enhanced = enhance_prompt(msg.content)
        table.insert(output.messages, {
            role = msg.role,
            content = enhanced
        })
        output.modified = true
    else
        table.insert(output.messages, msg)
    end
end

_G.output = output
\\\

### 示例4: HTTP Webhook (Python Flask)

位置: \HTTP_Webhook/app.py\

\\\python
from flask import Flask, request, jsonify

app = Flask(__name__)

@app.route('/webhook/message-hook', methods=['POST'])
def message_hook():
    data = request.get_json()
    
    # 提取消息
    messages = data.get('messages', [])
    user_id = data.get('user_id')
    model = data.get('model')
    
    # 处理逻辑
    modified = False
    
    # 示例：为特定用户添加系统提示
    if user_id == 1:
        system_msg = {
            'role': 'system',
            'content': '你是一个专业的技术顾问。'
        }
        messages.insert(0, system_msg)
        modified = True
    
    # 返回结果
    return jsonify({
        'modified': modified,
        'messages': messages,
        'abort': False
    }), 200

if __name__ == '__main__':
    # 使用默认白名单端口
    app.run(host='0.0.0.0', port=55566)
\\\

**运行**:
\\\ash
cd HTTP_Webhook
pip install flask python-dotenv
python app.py
\\\

**配置Hook**:
- 类型: HTTP
- URL: \http://127.0.0.1:55566/webhook/message-hook\
- 超时: 5000ms


---

## 测试

### 单元测试

#### 1. DTO测试

位置: \dto/message_hook_test.go\

\\\go
func TestValidateHookInput(t *testing.T) {
    // 测试有效输入
    validInput := &HookInput{
        UserId: 1,
        Messages: []Message{
            {Role: "user", Content: "Hello"},
        },
        Model: "gpt-4",
    }
    err := ValidateHookInput(validInput)
    assert.NoError(t, err)
    
    // 测试无效输入
    invalidInput := &HookInput{
        UserId: 0,  // 无效
        Messages: []Message{},
        Model: "",
    }
    err = ValidateHookInput(invalidInput)
    assert.Error(t, err)
}

func TestValidateHookOutput(t *testing.T) {
    // 测试有效输出
    validOutput := &HookOutput{
        Modified: true,
        Messages: []Message{
            {Role: "user", Content: "Hello"},
        },
        Abort: false,
    }
    err := ValidateHookOutput(validOutput)
    assert.NoError(t, err)
}
\\\

#### 2. Lua执行器测试

位置: \service/lua_executor_test.go\

\\\go
func TestLuaExecutor_Execute(t *testing.T) {
    executor := NewLuaExecutor()
    
    script := \
        local messages = input.messages
        output = {
            modified = true,
            messages = messages,
            abort = false
        }
    \
    
    input := &dto.HookInput{
        UserId: 1,
        Messages: []dto.Message{
            {Role: "user", Content: "Hello"},
        },
        Model: "gpt-4",
    }
    
    output, err := executor.Execute(script, input, 5*time.Second)
    assert.NoError(t, err)
    assert.True(t, output.Modified)
    assert.False(t, output.Abort)
}

func TestLuaExecutor_Timeout(t *testing.T) {
    executor := NewLuaExecutor()
    
    // 无限循环脚本
    script := \
        while true do
        end
    \
    
    input := &dto.HookInput{
        UserId: 1,
        Messages: []dto.Message{{Role: "user", Content: "Hello"}},
        Model: "gpt-4",
    }
    
    _, err := executor.Execute(script, input, 100*time.Millisecond)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "timeout")
}
\\\

#### 3. HTTP执行器测试

位置: \service/http_executor_test.go\

\\\go
func TestHTTPExecutor_Execute(t *testing.T) {
    // 启动测试服务器
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        var input dto.HookInput
        json.NewDecoder(r.Body).Decode(&input)
        
        output := dto.HookOutput{
            Modified: true,
            Messages: input.Messages,
            Abort:    false,
        }
        
        json.NewEncoder(w).Encode(output)
    }))
    defer server.Close()
    
    executor := NewHTTPExecutor()
    input := &dto.HookInput{
        UserId: 1,
        Messages: []dto.Message{{Role: "user", Content: "Hello"}},
        Model: "gpt-4",
    }
    
    output, err := executor.Execute(server.URL, input, 5*time.Second)
    assert.NoError(t, err)
    assert.True(t, output.Modified)
}

func TestValidateHTTPHookURL_SSRF(t *testing.T) {
    // 测试私有IP拦截
    err := ValidateHTTPHookURL("http://127.0.0.1:8080/hook")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "private IP")
    
    // 测试白名单端口
    err = ValidateHTTPHookURL("http://127.0.0.1:55566/hook")
    assert.NoError(t, err)
    
    // 测试公网HTTPS
    err = ValidateHTTPHookURL("https://api.example.com/hook")
    assert.NoError(t, err)
}
\\\

#### 4. Service测试

位置: \service/message_hook_service_test.go\

\\\go
func TestMessageHookService_ExecuteHooks(t *testing.T) {
    service := NewMessageHookService()
    
    hooks := []*model.MessageHook{
        {
            Id:       1,
            Name:     "Test Hook",
            Type:     constant.MessageHookTypeLua,
            Content:  "output = {modified = false, messages = input.messages, abort = false}",
            Enabled:  true,
            Priority: 100,
            Timeout:  5000,
        },
    }
    
    input := &dto.HookInput{
        UserId: 1,
        Messages: []dto.Message{{Role: "user", Content: "Hello"}},
        Model: "gpt-4",
    }
    
    output, err := service.ExecuteHooks(hooks, input)
    assert.NoError(t, err)
    assert.NotNil(t, output)
}

func TestMessageHookService_MatchesFilters(t *testing.T) {
    service := &messageHookService{}
    
    hook := &model.MessageHook{
        FilterUsers: "[1,2,3]",
        FilterModels: "[\"gpt-4\",\"claude-3\"]",
    }
    
    input := &dto.HookInput{
        UserId: 1,
        Model:  "gpt-4",
    }
    
    matches := service.matchesFilters(hook, input)
    assert.True(t, matches)
    
    input.UserId = 999
    matches = service.matchesFilters(hook, input)
    assert.False(t, matches)
}
\\\

### 集成测试

#### 端到端测试流程

\\\ash
# 1. 启动服务
go run main.go

# 2. 创建测试Hook
curl -X POST http://localhost:3000/api/message-hooks \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test Hook",
    "type": 1,
    "content": "output = {modified = false, messages = input.messages, abort = false}",
    "enabled": true,
    "priority": 100,
    "timeout": 5000
  }'

# 3. 测试Hook
curl -X POST http://localhost:3000/api/message-hooks/test \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "hook": {
      "type": 1,
      "content": "output = {modified = false, messages = input.messages, abort = false}",
      "timeout": 5000
    },
    "input": {
      "user_id": 1,
      "messages": [{"role": "user", "content": "Hello"}],
      "model": "gpt-4"
    }
  }'

# 4. 发送实际请求测试中间件
curl -X POST http://localhost:3000/v1/chat/completions \
  -H "Authorization: Bearer YOUR_USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
\\\

---

## 部署注意事项

### 1. 数据库迁移

确保在 \model/main.go\ 中包含 \MessageHook\ 模型：

\\\go
err := DB.AutoMigrate(
    // ... 其他模型
    &MessageHook{},
)
\\\

### 2. 环境变量配置

生产环境建议配置：

\\\ash
# 启用SSRF保护（默认）
MESSAGE_HOOK_DISABLE_SSRF_PROTECTION=false

# 只允许HTTPS（默认）
MESSAGE_HOOK_ALLOW_HTTP=false

# 配置白名单主机（如果需要访问内网服务）
MESSAGE_HOOK_ALLOWED_HOSTS=internal-api.company.com,192.168.1.100

# 默认超时时间
MESSAGE_HOOK_DEFAULT_TIMEOUT=5000

# 缓存TTL
MESSAGE_HOOK_CACHE_TTL=60
\\\

### 3. Redis缓存

如果使用Redis，确保配置：

\\\ash
REDIS_CONN_STRING=redis://localhost:6379
\\\

Hook列表会被缓存，TTL默认60秒。

### 4. 性能优化

**Lua执行器**:
- 使用状态池化(\sync.Pool\)
- 预编译脚本（如果可能）
- 设置合理的超时时间

**HTTP执行器**:
- 使用连接池
- 设置合理的超时时间
- 考虑使用异步执行（对于非关键Hook）

**缓存策略**:
- 启用Redis缓存
- 合理设置TTL
- 在Hook变更时主动失效缓存

### 5. 监控与日志

**关键日志点**:
- Hook执行开始/结束
- Hook执行失败
- Hook中止请求
- Hook修改消息

**监控指标**:
- Hook调用次数
- Hook成功率
- Hook平均执行时间
- Hook超时次数

**日志示例**:
\\\go
common.SysLog(fmt.Sprintf("[MESSAGE_HOOK] Executing hook %d (%s) for userId=%d", 
    hook.Id, hook.Name, input.UserId))
\\\

### 6. 安全建议

**Lua脚本**:
- 严格的沙箱限制
- 禁用所有危险函数
- 限制脚本大小（1MB）
- 设置执行超时

**HTTP Webhook**:
- 启用SSRF保护
- 只允许HTTPS（生产环境）
- 使用白名单机制
- 验证响应内容

**访问控制**:
- 只有管理员可以创建/修改Hook
- 记录所有Hook操作日志
- 定期审计Hook配置

### 7. 故障处理

**优雅降级**:
- Hook执行失败不影响主流程
- 继续执行后续Hook
- 记录错误但不中断请求

**错误处理**:
\\\go
if err != nil {
    common.SysError(fmt.Sprintf("Hook %d execution failed: %v", hook.Id, err))
    // 继续执行，不中断
    continue
}
\\\

### 8. 备份与恢复

**数据库备份**:
定期备份 \message_hooks\ 表

**配置导出**:
\\\sql
-- 导出Hook配置
SELECT * FROM message_hooks WHERE enabled = true;
\\\

---

## 完整文件清单

### 后端文件

\\\
constant/
└── message_hook.go              # 常量定义

dto/
├── message_hook.go              # DTO定义
└── message_hook_test.go         # DTO测试

model/
├── message_hook.go              # 数据模型
├── message_hook_test.go         # 模型测试
└── main.go                      # 数据库迁移（添加MessageHook）

service/
├── message_hook_service.go      # 业务逻辑
├── message_hook_service_test.go # Service测试
├── lua_executor.go              # Lua执行器
├── lua_executor_test.go         # Lua测试
├── http_executor.go             # HTTP执行器
└── http_executor_test.go        # HTTP测试

controller/
└── message_hook.go              # API控制器

middleware/
└── message_hook.go              # 中间件

router/
├── api-router.go                # API路由（添加Hook路由）
└── relay-router.go              # Relay路由（注册中间件）
\\\

### 前端文件

\\\
web/src/
├── App.jsx                      # 路由配置（添加Hook路由）
├── pages/
│   └── MessageHook/
│       ├── index.jsx            # Hook列表页
│       ├── EditMessageHook.jsx  # 创建/编辑页
│       ├── TestMessageHook.jsx  # 测试页
│       └── MessageHook.css      # 样式
├── components/
│   ├── table/message-hooks/
│   │   └── index.jsx            # Hook表格组件
│   └── settings/
│       └── OperationSetting.jsx # 设置组件（添加Hook设置）
└── i18n/locales/
    └── zh-CN.json               # 中文翻译（添加Hook相关）
\\\

### 示例文件

\\\
examples/message_hooks/
├── README.md                    # 示例说明
├── content_moderation.lua       # 内容审核示例
├── context_injection.lua        # 上下文注入示例
├── prompt_enhancement.lua       # 提示词增强示例
└── privacy_filter.lua           # 隐私过滤示例

HTTP_Webhook/
├── app.py                       # Flask Webhook服务
├── .env                         # 环境变量
├── .env.example                 # 环境变量示例
└── README.md                    # 使用说明
\\\

### 文档文件

\\\
docs/
├── message_hooks_api.md         # API文档
├── message_hooks_user_guide.md  # 用户指南
├── message_hooks_integration.md # 集成指南
└── message_hooks_context_info.md # 上下文信息
\\\

---

## 常见问题

### Q1: Hook不执行？

**检查清单**:
1. Hook是否启用（\enabled = true\）
2. 过滤条件是否匹配
3. 中间件是否正确注册
4. 查看系统日志

### Q2: Lua脚本报错？

**常见错误**:
- 语法错误：检查Lua语法
- 全局变量未设置：确保设置 \output\ 全局变量
- 禁用函数：不要使用 \io\, \os\, \debug\ 等

### Q3: HTTP Webhook被拦截？

**解决方案**:
1. 使用默认白名单端口（55566-55569）
2. 添加主机到白名单：\MESSAGE_HOOK_ALLOWED_HOSTS\
3. 使用HTTPS协议
4. 临时禁用SSRF保护（仅开发环境）

### Q4: Hook执行超时？

**优化建议**:
- 增加超时时间
- 优化Lua脚本逻辑
- 优化HTTP Webhook响应速度
- 考虑异步执行

### Q5: 如何调试Hook？

**调试方法**:
1. 使用测试页面测试Hook
2. 查看系统日志
3. 使用 \common.SysLog\ 添加调试日志
4. 检查Hook统计信息

---

## 总结

本文档详细记录了 new-api 项目中消息钩子(Message Hook)功能的完整实现，包括：

1. **数据库层**: MessageHook模型定义和CRUD操作
2. **后端实现**: 
   - Lua执行器（沙箱化、状态池化）
   - HTTP执行器（SSRF保护、连接池化）
   - Service层（业务逻辑、过滤器、统计）
   - Controller层（API端点）
   - 中间件（请求拦截和处理）
3. **前端实现**:
   - Hook管理界面
   - 创建/编辑表单
   - 测试工具
   - 统计展示
4. **配置与部署**:
   - 环境变量配置
   - 安全机制
   - 性能优化
   - 监控日志

通过本文档，可以在原始无Hook机制的项目中完整重现该功能。

---

**文档版本**: 1.0  
**最后更新**: 2025年  
**维护者**: QuantumNous  
**项目**: new-api (https://github.com/QuantumNous/new-api)

---
