# 渠道调用机制深度分析

## 概述

new-api 使用**基于优先级和权重的智能路由系统**，而不是简单的轮询。这是一个高度优化的负载均衡机制。

## 1. 渠道选择流程

### 1.1 请求处理流程

```
客户端请求
    ↓
middleware/distributor.go (Distribute 中间件)
    ↓
解析模型名称 (getModelRequest)
    ↓
检查 Token 权限和模型限制
    ↓
【渠道亲和性检查】service.GetPreferredChannelByAffinity()
    ↓ (如果没有亲和性缓存)
【智能选择】service.CacheGetRandomSatisfiedChannel()
    ↓
model.GetRandomSatisfiedChannel() - 核心选择逻辑
    ↓
SetupContextForSelectedChannel() - 设置上下文
    ↓
执行请求
    ↓
【记录亲和性】service.RecordChannelAffinity() (成功时)
```

## 2. 核心选择算法

### 2.1 优先级 + 权重算法

```go
// 1. 按优先级分组
uniquePriorities := make(map[int]bool)
for _, channelId := range channels {
    uniquePriorities[int(channel.GetPriority())] = true
}

// 2. 排序优先级（降序）
sort.Sort(sort.Reverse(sort.IntSlice(sortedUniquePriorities)))

// 3. 根据重试次数选择优先级层级
if retry >= len(uniquePriorities) {
    retry = len(uniquePriorities) - 1
}
targetPriority := int64(sortedUniquePriorities[retry])

// 4. 在同一优先级内，按权重随机选择
sumWeight := 0
for _, channel := range targetChannels {
    sumWeight += channel.GetWeight()
}

randomWeight := rand.Intn(totalWeight)
for _, channel := range targetChannels {
    randomWeight -= channel.GetWeight()
    if randomWeight < 0 {
        return channel  // 选中！
    }
}
```

### 2.2 算法特点

✅ **优先级优先**：优先使用高优先级渠道
✅ **权重负载均衡**：同优先级内按权重分配流量
✅ **自动降级**：失败后自动尝试低优先级渠道
✅ **平滑因子**：避免权重过小导致的不均衡

## 3. 渠道亲和性（Channel Affinity）

### 3.1 什么是渠道亲和性？

渠道亲和性是一种**智能缓存机制**，记住用户/模型/组合最近成功使用的渠道，优先复用。

### 3.2 工作原理

```go
// 1. 请求前：检查是否有缓存的首选渠道
if preferredChannelID, found := service.GetPreferredChannelByAffinity(c, modelRequest.Model, usingGroup); found {
    preferred, err := model.CacheGetChannel(preferredChannelID)
    if err == nil && preferred.Status == common.ChannelStatusEnabled {
        channel = preferred  // 直接使用缓存的渠道
        service.MarkChannelAffinityUsed(c, usingGroup, preferred.Id)
    }
}

// 2. 请求成功后：记录亲和性
if channel != nil && c.Writer.Status() < http.StatusBadRequest {
    service.RecordChannelAffinity(c, channel.Id)
}
```

### 3.3 亲和性优势

✅ **减少延迟**：避免每次都重新选择渠道
✅ **提高稳定性**：优先使用已验证可用的渠道
✅ **降低失败率**：避免频繁切换到不稳定渠道
✅ **用户体验**：同一用户/模型保持一致的响应特性

## 4. 回答你的问题

### Q1: 是轮询吗？

**不是简单轮询**，而是：

1. **优先级分层**：先按优先级选择
2. **权重随机**：同优先级内按权重随机（加权随机，不是轮询）
3. **亲和性优先**：有缓存时直接使用缓存渠道
4. **多 Key 轮询**：单个渠道内的多个 API Key 可以配置轮询模式

```go
// 多 Key 轮询示例
if channel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
    // 保留轮询索引
    channel.ChannelInfo.MultiKeyPollingIndex = oldChannel.ChannelInfo.MultiKeyPollingIndex
}
```

### Q2: 如果只有一个渠道有该模型会怎样？

**完全没问题**，系统会：

1. **精确匹配**：首先查找完全匹配的渠道
   ```go
   channels := group2model2channels[group][model]
   ```

2. **模糊匹配**：如果没找到，尝试规范化模型名
   ```go
   if len(channels) == 0 {
       normalizedModel := ratio_setting.FormatMatchingModelName(model)
       channels = group2model2channels[group][normalizedModel]
   }
   ```

3. **唯一渠道**：如果只有一个渠道，直接返回
   ```go
   if len(channels) == 1 {
       return channelsIDM[channels[0]], nil
   }
   ```

4. **无渠道处理**：如果没有任何渠道支持该模型
   ```go
   if len(channels) == 0 {
       return nil, nil  // 返回 nil，上层会报错
   }
   ```

**错误提示**：
```
"当前分组 {group} 下对于模型 {model} 无可用渠道"
```

### Q3: 各渠道时延不一致会导致连接不稳定吗？

**不会**，系统有多重保护机制：

#### 3.1 优先级机制

```
高优先级渠道（快速、稳定）
    ↓ 失败
中优先级渠道（备用）
    ↓ 失败
低优先级渠道（兜底）
```

管理员可以将：
- **低延迟渠道** → 高优先级
- **高延迟渠道** → 低优先级

#### 3.2 自动重试机制

```go
// 重试参数
type RetryParam struct {
    Ctx        *gin.Context
    ModelName  string
    TokenGroup string
    Retry      *int  // 重试次数，每次失败 +1
}

// 自动降级到低优先级
targetPriority := sortedUniquePriorities[retry]
```

**重试流程**：
```
第 0 次：尝试最高优先级渠道
    ↓ 失败
第 1 次：尝试次高优先级渠道
    ↓ 失败
第 2 次：尝试第三优先级渠道
    ↓ ...
```

#### 3.3 自动禁用机制

```go
func ShouldDisableChannel(channelType int, err *types.NewAPIError) bool {
    if !common.AutomaticDisableChannelEnabled {
        return false
    }
    
    // 检查错误类型
    if err.StatusCode == http.StatusUnauthorized {
        return true  // 认证失败，禁用
    }
    
    // 检查错误关键词
    search, _ := AcSearch(lowerMessage, operation_setting.AutomaticDisableKeywords, true)
    return search
}
```

**自动禁用触发条件**：
- 401 Unauthorized（API Key 无效）
- 403 Forbidden（权限不足）
- 余额不足
- 账户被封禁
- 自定义关键词匹配

#### 3.4 渠道亲和性缓存

```go
// 成功的渠道会被缓存
service.RecordChannelAffinity(c, channel.Id)

// 下次优先使用
if preferredChannelID, found := service.GetPreferredChannelByAffinity(...); found {
    channel = preferred  // 直接使用，避免重新选择
}
```

**效果**：
- 用户会"粘"在响应快的渠道上
- 避免频繁切换到慢速渠道
- 提供一致的用户体验

#### 3.5 超时保护

虽然代码中没有明确的超时配置，但 HTTP 客户端通常有默认超时：

```go
// service/http_client.go (推测)
client := &http.Client{
    Timeout: 60 * time.Second,  // 典型配置
}
```

超时后会触发重试机制，自动切换到其他渠道。

## 5. 渠道配置最佳实践

### 5.1 优先级配置

| 渠道类型 | 建议优先级 | 原因 |
|---------|-----------|------|
| 官方 API（低延迟） | 100 | 最快、最稳定 |
| 代理 API（中延迟） | 50 | 备用 |
| 免费 API（高延迟） | 10 | 兜底 |

### 5.2 权重配置

**同优先级内**：
- 高性能渠道：权重 100
- 中性能渠道：权重 50
- 低性能渠道：权重 10

**效果**：
- 高性能渠道获得 100/(100+50+10) = 62.5% 流量
- 中性能渠道获得 50/(100+50+10) = 31.25% 流量
- 低性能渠道获得 10/(100+50+10) = 6.25% 流量

### 5.3 多模型配置

```
渠道 A: gpt-4, gpt-3.5-turbo (优先级 100)
渠道 B: gpt-4, gpt-3.5-turbo (优先级 50)
渠道 C: gpt-3.5-turbo (优先级 10)
```

**请求 gpt-4**：
- 第 0 次重试：A 或 B（优先级 100）
- 第 1 次重试：B（优先级 50）

**请求 gpt-3.5-turbo**：
- 第 0 次重试：A 或 B（优先级 100）
- 第 1 次重试：B（优先级 50）
- 第 2 次重试：C（优先级 10）

## 6. 性能优化

### 6.1 内存缓存

```go
var group2model2channels map[string]map[string][]int  // 渠道索引
var channelsIDM map[int]*Channel                      // 渠道详情

// 定期同步
func SyncChannelCache(frequency int) {
    for {
        time.Sleep(time.Duration(frequency) * time.Second)
        InitChannelCache()
    }
}
```

**优势**：
- 避免每次请求都查询数据库
- 毫秒级渠道选择
- 支持高并发

### 6.2 读写锁

```go
var channelSyncLock sync.RWMutex

// 读取时
channelSyncLock.RLock()
defer channelSyncLock.RUnlock()

// 更新时
channelSyncLock.Lock()
defer channelSyncLock.Unlock()
```

**效果**：
- 多个请求可以并发读取
- 更新时阻塞读取，保证一致性

## 7. 故障处理

### 7.1 渠道失败

```
请求 → 渠道 A（优先级 100）→ 失败
    ↓
重试 → 渠道 B（优先级 50）→ 失败
    ↓
重试 → 渠道 C（优先级 10）→ 成功
    ↓
记录亲和性：下次优先使用渠道 C
```

### 7.2 所有渠道失败

```go
if channel == nil {
    abortWithOpenAiMessage(c, http.StatusServiceUnavailable, 
        "当前分组下对于模型 xxx 无可用渠道")
    return
}
```

### 7.3 自动恢复

```go
func ShouldEnableChannel(newAPIError *types.NewAPIError, status int) bool {
    if !common.AutomaticEnableChannelEnabled {
        return false
    }
    if newAPIError != nil {
        return false  // 有错误，不恢复
    }
    if status != common.ChannelStatusAutoDisabled {
        return false  // 不是自动禁用的，不恢复
    }
    return true  // 成功请求，自动恢复
}
```

## 8. 总结

### ✅ 优势

1. **智能路由**：优先级 + 权重 + 亲和性
2. **高可用**：自动重试 + 自动禁用 + 自动恢复
3. **高性能**：内存缓存 + 读写锁
4. **灵活配置**：支持多种负载均衡策略
5. **用户体验**：亲和性保证一致性

### ⚠️ 注意事项

1. **优先级设置**：合理设置优先级，避免低质量渠道被频繁使用
2. **权重配置**：权重差异不要太大，避免某些渠道完全不被使用
3. **亲和性时间**：需要合理设置缓存时间，避免"粘"在故障渠道上
4. **监控告警**：及时发现和处理渠道故障

### 🎯 最佳实践

1. **分层部署**：官方 API（高优先级）+ 代理 API（中优先级）+ 免费 API（低优先级）
2. **权重调优**：根据实际性能调整权重
3. **启用自动禁用**：快速隔离故障渠道
4. **启用自动恢复**：自动恢复正常渠道
5. **定期测试**：使用"测试所有渠道"功能验证可用性

## 9. 与其他系统对比

| 特性 | new-api | 简单轮询 | Nginx 负载均衡 |
|------|---------|---------|---------------|
| 优先级 | ✅ | ❌ | ⚠️ (upstream) |
| 权重 | ✅ | ❌ | ✅ |
| 亲和性 | ✅ | ❌ | ⚠️ (ip_hash) |
| 自动重试 | ✅ | ❌ | ✅ |
| 自动禁用 | ✅ | ❌ | ✅ (health check) |
| 模型感知 | ✅ | ❌ | ❌ |
| 用户组感知 | ✅ | ❌ | ❌ |

**结论**：new-api 的渠道路由系统是专门为 AI API 网关设计的，比通用负载均衡器更智能、更灵活。
