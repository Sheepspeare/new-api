# HTTP Webhook服务 - Tavily搜索增强

这是一个Flask HTTP Webhook服务，用于new-api项目的消息钩子功能测试。集成了Tavily搜索API，可以自动检测用户问题并增强prompt。

## 功能特性

- 🔍 自动检测问题模式（为什么xxx、xxx是什么等）
- 🌐 使用Tavily API进行实时搜索
- 📝 将搜索结果格式化并添加到用户消息中
- 🚀 支持中英文问题检测
- 📊 详细的日志记录
- 🔒 使用默认白名单端口（55566）避免SSRF拦截

## 安装步骤

### 1. 安装Python依赖

```bash
cd HTTP_Webhook
pip install -r requirements.txt
```

### 2. 配置环境变量

复制 `.env.example` 到 `.env` 并填写你的Tavily API Key:

```bash
cp .env.example .env
```

编辑 `.env` 文件:
```
TAVILY_API_KEY=tvly-YOUR_ACTUAL_API_KEY
PORT=55566
DEBUG=True
```

**重要**: 使用端口 `55566`（或55567-55569）可以避免SSRF拦截，这些端口在new-api中默认白名单。

获取Tavily API Key: https://tavily.com/

### 3. 启动服务

```bash
python app.py
```

服务将在 `http://localhost:55566` 启动。

你会看到类似输出：
```
Starting Flask server on port 55566
Webhook URL: http://127.0.0.1:55566/webhook/message-hook
This port is in the default SSRF whitelist (55566-55569)
```

## API端点

### 1. 健康检查

```bash
GET http://localhost:55566/health
```

响应:
```json
{
  "status": "healthy",
  "tavily_enabled": true
}
```

### 2. 消息钩子端点（主要端点）

```bash
POST http://localhost:55566/webhook/message-hook
Content-Type: application/json

{
  "user_id": 1,
  "conversation_id": "conv-123",
  "messages": [
    {
      "role": "user",
      "content": "为什么天空是蓝色的？"
    }
  ],
  "model": "gpt-4",
  "token_id": 10
}
```

响应:
```json
{
  "modified": true,
  "messages": [
    {
      "role": "user",
      "content": "为什么天空是蓝色的？\n\n【搜索增强信息】\n\n📝 摘要: 天空呈现蓝色是因为...\n\n📚 相关资料:\n1. 标题\n   内容预览...\n   来源: https://...\n\n---\n"
    }
  ],
  "abort": false
}
```

### 3. 测试搜索端点

```bash
POST http://localhost:55566/webhook/test
Content-Type: application/json

{
  "query": "什么是人工智能"
}
```

## 支持的问题模式

服务会自动检测以下问题模式并触发搜索：

### 中文模式
- 为什么xxx
- xxx是什么
- 什么是xxx
- xxx怎么样
- 如何xxx
- 怎么xxx
- xxx的原因
- xxx的定义
- 解释xxx
- 介绍xxx

### 英文模式
- what is xxx
- why xxx
- how xxx
- explain xxx

## 在new-api中配置

### 方式1: 使用默认白名单端口（推荐）

**无需任何配置！** 端口55566-55569默认在白名单中。

1. **创建HTTP类型的消息钩子**

在new-api管理后台:
- 访问 `/console/message-hooks/create`
- 名称: `Tavily搜索增强`
- 类型: `HTTP`
- 内容（URL）: `http://127.0.0.1:55566/webhook/message-hook`
- 启用: ✅
- 优先级: `100`
- 超时: `10000` (10秒)
- 保存

2. **测试**

在Cherry Studio或其他客户端发送问题:
```
为什么Python这么流行？
```

你应该会收到增强了搜索结果的回复。

### 方式2: 使用其他端口（需要配置）

如果你想使用其他端口（如5000），需要在new-api的 `.env` 文件中添加:

```bash
# 允许HTTP协议
MESSAGE_HOOK_ALLOW_HTTP=true

# 白名单localhost和自定义端口
MESSAGE_HOOK_ALLOWED_HOSTS=localhost,127.0.0.1
```

然后修改Flask的 `.env`:
```bash
PORT=5000
```

### URL格式说明

根据new-api的SSRF保护规则：

✅ **推荐（默认白名单）**:
- `http://127.0.0.1:55566/webhook/message-hook`
- `http://127.0.0.1:55567/webhook/message-hook`
- `http://127.0.0.1:55568/webhook/message-hook`
- `http://127.0.0.1:55569/webhook/message-hook`
- `http://localhost:55566/webhook/message-hook`

⚠️ **需要配置**:
- `http://127.0.0.1:5000/webhook/message-hook` （需要MESSAGE_HOOK_ALLOW_HTTP=true）
- `http://192.168.1.100:8080/webhook/message-hook` （需要添加到白名单）

✅ **公网HTTPS（无需配置）**:
- `https://your-domain.com/webhook/message-hook`

## 测试示例

### 使用curl测试

```bash
# 测试健康检查
curl http://localhost:55566/health

# 测试消息钩子
curl -X POST http://localhost:55566/webhook/message-hook \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 1,
    "messages": [
      {
        "role": "user",
        "content": "什么是机器学习？"
      }
    ],
    "model": "gpt-4"
  }'

# 测试搜索功能
curl -X POST http://localhost:55566/webhook/test \
  -H "Content-Type: application/json" \
  -d '{
    "query": "人工智能的发展历史"
  }'
```

### 使用Python测试脚本

项目已包含测试脚本：

```bash
python test_webhook.py
```

或手动测试：

```python
import requests

# 测试消息钩子
response = requests.post(
    'http://localhost:55566/webhook/message-hook',
    json={
        'user_id': 1,
        'messages': [
            {
                'role': 'user',
                'content': '为什么地球是圆的？'
            }
        ],
        'model': 'gpt-4'
    }
)

print(response.json())
```

## 日志

服务会输出详细的日志信息:

```
2026-03-15 17:30:00 - __main__ - INFO - Tavily client initialized successfully
2026-03-15 17:30:00 - __main__ - INFO - Starting Flask server on port 55566
2026-03-15 17:30:00 - __main__ - INFO - Webhook URL: http://127.0.0.1:55566/webhook/message-hook
2026-03-15 17:30:00 - __main__ - INFO - This port is in the default SSRF whitelist (55566-55569)
2026-03-15 17:30:10 - __main__ - INFO - Received hook request: userId=1, model=gpt-4
2026-03-15 17:30:10 - __main__ - INFO - Detected search pattern: '为什么(.+)' -> keyword: '天空是蓝色的'
2026-03-15 17:30:10 - __main__ - INFO - Searching with Tavily: 天空是蓝色的
2026-03-15 17:30:11 - __main__ - INFO - Tavily search completed: 3 results
2026-03-15 17:30:11 - __main__ - INFO - Message enhanced with search results (length: 456)
2026-03-15 17:30:11 - __main__ - INFO - Returning response: modified=true
```

## 故障排查

### 问题1: Tavily API Key无效

**错误**: `Tavily client not initialized`

**解决**: 
- 检查 `.env` 文件中的 `TAVILY_API_KEY` 是否正确
- 确认API Key有效且有足够的配额
- 访问 https://tavily.com/ 获取新的API Key

### 问题2: new-api无法连接到webhook

**错误**: `connection refused` 或 `SSRF protection`

**解决**:
- 确认Flask服务正在运行: `curl http://localhost:55566/health`
- 确认使用的是默认白名单端口（55566-55569）
- 如果使用其他端口，检查new-api的SSRF配置（见上文）
- 确认端口没有被占用: `netstat -ano | findstr 55566` (Windows) 或 `lsof -i :55566` (Linux/Mac)

### 问题3: 搜索没有触发

**检查**:
- 查看Flask日志，确认是否检测到问题模式
- 尝试不同的问题格式
- 使用 `/webhook/test` 端点直接测试搜索功能

## 生产环境部署

生产环境建议使用Gunicorn:

```bash
pip install gunicorn

gunicorn -w 4 -b 0.0.0.0:5000 app:app
```

或使用Docker:

```dockerfile
FROM python:3.11-slim

WORKDIR /app
COPY requirements.txt .
RUN pip install -r requirements.txt

COPY . .

CMD ["gunicorn", "-w", "4", "-b", "0.0.0.0:5000", "app:app"]
```

## 许可证

本示例代码仅用于测试和学习目的。
