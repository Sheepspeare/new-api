# Message Hooks Integration Guide

## Overview

This guide explains how to integrate external services with the Message Hooks system using HTTP hooks.

## HTTP Hook Service Requirements

### Endpoint Requirements

Your HTTP service must:
1. Accept POST requests
2. Use HTTPS (HTTP is blocked for security)
3. Return JSON responses
4. Respond within the configured timeout
5. Use a public IP address (private IPs are blocked for SSRF protection)

### Request Format

Your service will receive POST requests with this structure:

```http
POST https://your-service.com/hook
Content-Type: application/json
User-Agent: new-api-message-hook/1.0

{
  "user_id": 123,
  "conversation_id": "conv-456",
  "model": "gpt-4",
  "token_id": 789,
  "messages": [
    {
      "role": "user",
      "content": "Hello, world!"
    }
  ]
}
```

### Response Format

Your service must return a 200 OK response with this structure:

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "modified": true,
  "messages": [
    {
      "role": "user",
      "content": "Modified: Hello, world!"
    }
  ],
  "abort": false,
  "reason": ""
}
```

**Response Fields:**
- `modified` (boolean, required): Set to `true` if you modified the messages
- `messages` (array, required if modified=true): The modified messages array
- `abort` (boolean, required): Set to `true` to block the request
- `reason` (string, optional): Explanation if aborting

## Implementation Examples

### Python (Flask)

```python
from flask import Flask, request, jsonify
import re

app = Flask(__name__)

@app.route('/hook', methods=['POST'])
def message_hook():
    data = request.json
    
    # Extract input
    user_id = data['user_id']
    messages = data['messages']
    model = data['model']
    
    # Process messages
    modified = False
    output_messages = []
    
    for msg in messages:
        content = msg['content']
        original = content
        
        # Example: Redact email addresses
        content = re.sub(r'[\w\.-]+@[\w\.-]+\.\w+', '[EMAIL]', content)
        
        if content != original:
            modified = True
        
        output_messages.append({
            'role': msg['role'],
            'content': content
        })
    
    # Return response
    return jsonify({
        'modified': modified,
        'messages': output_messages if modified else [],
        'abort': False,
        'reason': ''
    })

if __name__ == '__main__':
    # Use HTTPS in production!
    app.run(host='0.0.0.0', port=443, ssl_context='adhoc')
```

### Node.js (Express)

```javascript
const express = require('express');
const https = require('https');
const fs = require('fs');

const app = express();
app.use(express.json());

app.post('/hook', (req, res) => {
    const { user_id, messages, model } = req.body;
    
    // Process messages
    let modified = false;
    const outputMessages = messages.map(msg => {
        let content = msg.content;
        const original = content;
        
        // Example: Redact phone numbers
        content = content.replace(/\d{3}-\d{3}-\d{4}/g, '[PHONE]');
        
        if (content !== original) {
            modified = true;
        }
        
        return {
            role: msg.role,
            content: content
        };
    });
    
    // Return response
    res.json({
        modified: modified,
        messages: modified ? outputMessages : [],
        abort: false,
        reason: ''
    });
});

// Use HTTPS in production
const options = {
    key: fs.readFileSync('key.pem'),
    cert: fs.readFileSync('cert.pem')
};

https.createServer(options, app).listen(443, () => {
    console.log('Hook service running on port 443');
});
```

### Go (Gin)

```go
package main

import (
    "net/http"
    "regexp"
    "github.com/gin-gonic/gin"
)

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type HookInput struct {
    UserId         int       `json:"user_id"`
    ConversationId string    `json:"conversation_id"`
    Messages       []Message `json:"messages"`
    Model          string    `json:"model"`
    TokenId        int       `json:"token_id"`
}

type HookOutput struct {
    Modified bool      `json:"modified"`
    Messages []Message `json:"messages,omitempty"`
    Abort    bool      `json:"abort"`
    Reason   string    `json:"reason,omitempty"`
}

func main() {
    r := gin.Default()
    
    r.POST("/hook", func(c *gin.Context) {
        var input HookInput
        if err := c.ShouldBindJSON(&input); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }
        
        // Process messages
        modified := false
        outputMessages := make([]Message, 0)
        emailRegex := regexp.MustCompile(`[\w\.-]+@[\w\.-]+\.\w+`)
        
        for _, msg := range input.Messages {
            content := msg.Content
            original := content
            
            // Redact emails
            content = emailRegex.ReplaceAllString(content, "[EMAIL]")
            
            if content != original {
                modified = true
            }
            
            outputMessages = append(outputMessages, Message{
                Role:    msg.Role,
                Content: content,
            })
        }
        
        // Return response
        output := HookOutput{
            Modified: modified,
            Abort:    false,
        }
        
        if modified {
            output.Messages = outputMessages
        }
        
        c.JSON(http.StatusOK, output)
    })
    
    // Use HTTPS in production
    r.RunTLS(":443", "cert.pem", "key.pem")
}
```

## Advanced Use Cases

### Vector Database Integration

Integrate with a vector database for semantic search and context retrieval:

```python
from flask import Flask, request, jsonify
import openai
from pinecone import Pinecone

app = Flask(__name__)
pc = Pinecone(api_key="your-api-key")
index = pc.Index("your-index")

@app.route('/hook', methods=['POST'])
def vector_search_hook():
    data = request.json
    messages = data['messages']
    
    # Get the last user message
    last_message = next((m for m in reversed(messages) if m['role'] == 'user'), None)
    if not last_message:
        return jsonify({'modified': False, 'abort': False})
    
    # Generate embedding
    embedding = openai.Embedding.create(
        input=last_message['content'],
        model="text-embedding-ada-002"
    )['data'][0]['embedding']
    
    # Search vector database
    results = index.query(vector=embedding, top_k=3)
    
    # Build context from results
    context = "\n".join([match['metadata']['text'] for match in results['matches']])
    
    # Inject context as system message
    output_messages = [
        {
            'role': 'system',
            'content': f'Relevant context:\n{context}'
        }
    ] + messages
    
    return jsonify({
        'modified': True,
        'messages': output_messages,
        'abort': False
    })
```

### Content Moderation Service

Integrate with an external moderation API:

```python
from flask import Flask, request, jsonify
import requests

app = Flask(__name__)

@app.route('/hook', methods=['POST'])
def moderation_hook():
    data = request.json
    messages = data['messages']
    
    # Check each message with moderation API
    for msg in messages:
        if msg['role'] == 'user':
            # Call moderation API
            response = requests.post(
                'https://moderation-api.example.com/check',
                json={'text': msg['content']},
                timeout=2
            )
            
            if response.json()['flagged']:
                return jsonify({
                    'modified': False,
                    'abort': True,
                    'reason': 'Content violates community guidelines'
                })
    
    # All messages passed moderation
    return jsonify({
        'modified': False,
        'abort': False
    })
```

### User Memory/Context Service

Store and retrieve user-specific context:

```python
from flask import Flask, request, jsonify
import redis

app = Flask(__name__)
r = redis.Redis(host='localhost', port=6379, db=0)

@app.route('/hook', methods=['POST'])
def memory_hook():
    data = request.json
    user_id = data['user_id']
    messages = data['messages']
    
    # Retrieve user context from Redis
    context_key = f'user_context:{user_id}'
    user_context = r.get(context_key)
    
    if user_context:
        # Inject context as system message
        output_messages = [
            {
                'role': 'system',
                'content': f'User context: {user_context.decode()}'
            }
        ] + messages
        
        return jsonify({
            'modified': True,
            'messages': output_messages,
            'abort': False
        })
    
    # No context available
    return jsonify({
        'modified': False,
        'abort': False
    })
```

## Security Best Practices

### 1. Use HTTPS

Always use HTTPS for your hook service. HTTP requests will be rejected.

```bash
# Generate self-signed certificate for testing
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes
```

### 2. Validate Input

Always validate the input structure:

```python
def validate_input(data):
    required_fields = ['user_id', 'messages', 'model']
    for field in required_fields:
        if field not in data:
            raise ValueError(f'Missing required field: {field}')
    
    if not isinstance(data['messages'], list) or len(data['messages']) == 0:
        raise ValueError('Messages must be a non-empty array')
    
    for msg in data['messages']:
        if 'role' not in msg or 'content' not in msg:
            raise ValueError('Each message must have role and content')
```

### 3. Implement Timeouts

Set appropriate timeouts for external API calls:

```python
import requests

response = requests.post(
    'https://external-api.com/endpoint',
    json=data,
    timeout=5  # 5 second timeout
)
```

### 4. Handle Errors Gracefully

Return proper error responses:

```python
@app.errorhandler(Exception)
def handle_error(error):
    return jsonify({
        'modified': False,
        'abort': False,
        'reason': ''
    }), 200  # Return 200 even on error to prevent request failure
```

### 5. Rate Limiting

Implement rate limiting to prevent abuse:

```python
from flask_limiter import Limiter
from flask_limiter.util import get_remote_address

limiter = Limiter(
    app,
    key_func=get_remote_address,
    default_limits=["100 per minute"]
)

@app.route('/hook', methods=['POST'])
@limiter.limit("10 per second")
def message_hook():
    # Your hook logic
    pass
```

## Performance Optimization

### 1. Connection Pooling

Reuse HTTP connections:

```python
import requests
from requests.adapters import HTTPAdapter
from requests.packages.urllib3.util.retry import Retry

session = requests.Session()
retry = Retry(total=3, backoff_factor=0.1)
adapter = HTTPAdapter(max_retries=retry, pool_connections=10, pool_maxsize=10)
session.mount('https://', adapter)
```

### 2. Caching

Cache expensive operations:

```python
from functools import lru_cache
import hashlib

@lru_cache(maxsize=1000)
def get_user_context(user_id):
    # Expensive operation
    return fetch_from_database(user_id)
```

### 3. Async Processing

Use async for I/O-bound operations:

```python
from flask import Flask
import asyncio
import aiohttp

app = Flask(__name__)

async def fetch_context(user_id):
    async with aiohttp.ClientSession() as session:
        async with session.get(f'https://api.example.com/context/{user_id}') as resp:
            return await resp.json()

@app.route('/hook', methods=['POST'])
def async_hook():
    data = request.json
    user_id = data['user_id']
    
    # Run async operation
    loop = asyncio.new_event_loop()
    context = loop.run_until_complete(fetch_context(user_id))
    loop.close()
    
    # Process with context
    # ...
```

## Testing Your Hook Service

### 1. Local Testing

Test locally before deploying:

```bash
curl -X POST https://localhost:443/hook \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 1,
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Test message"}
    ]
  }'
```

### 2. Use the Test Endpoint

Test through the new-api test endpoint:

```bash
curl -X POST https://your-new-api.com/api/message-hooks/test \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "hook": {
      "type": 2,
      "content": "https://your-service.com/hook",
      "timeout": 5000
    },
    "input": {
      "user_id": 1,
      "model": "gpt-4",
      "messages": [
        {"role": "user", "content": "Test message"}
      ]
    }
  }'
```

### 3. Monitor Logs

Monitor your service logs for errors:

```python
import logging

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

@app.route('/hook', methods=['POST'])
def message_hook():
    try:
        data = request.json
        logger.info(f"Received request for user {data['user_id']}")
        # Process...
        logger.info("Request processed successfully")
        return jsonify(response)
    except Exception as e:
        logger.error(f"Error processing request: {e}")
        raise
```

## Deployment

### Docker Deployment

```dockerfile
FROM python:3.9-slim

WORKDIR /app
COPY requirements.txt .
RUN pip install -r requirements.txt

COPY . .

EXPOSE 443
CMD ["python", "app.py"]
```

### Environment Variables

Use environment variables for configuration:

```python
import os

REDIS_HOST = os.getenv('REDIS_HOST', 'localhost')
REDIS_PORT = int(os.getenv('REDIS_PORT', 6379))
API_KEY = os.getenv('API_KEY')
```

## Troubleshooting

### Common Issues

1. **HTTPS Required Error**
   - Ensure your service uses HTTPS
   - Check SSL certificate is valid

2. **Timeout Errors**
   - Optimize your service response time
   - Increase timeout in hook configuration
   - Add caching for expensive operations

3. **SSRF Protection Blocking**
   - Ensure your service uses a public IP
   - Private IPs (10.x, 172.16-31.x, 192.168.x, 127.x) are blocked

4. **Invalid Response Format**
   - Ensure response is valid JSON
   - Include all required fields
   - Return 200 status code

### Debug Mode

Enable debug logging:

```python
app.config['DEBUG'] = True
logging.basicConfig(level=logging.DEBUG)
```

## Support

For additional help:
- Review the API documentation
- Check the example scripts
- Test using the test endpoint
- Monitor hook statistics for errors
