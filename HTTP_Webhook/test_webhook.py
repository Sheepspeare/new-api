"""
测试脚本 - 用于测试HTTP Webhook服务
"""

import requests
import json

# 使用默认白名单端口（55566-55569）
BASE_URL = "http://localhost:55566"


def test_health():
    """测试健康检查端点"""
    print("=" * 50)
    print("测试健康检查...")
    print("=" * 50)
    
    response = requests.get(f"{BASE_URL}/health")
    print(f"状态码: {response.status_code}")
    print(f"响应: {json.dumps(response.json(), indent=2, ensure_ascii=False)}")
    print()


def test_message_hook(question):
    """测试消息钩子端点"""
    print("=" * 50)
    print(f"测试消息钩子: {question}")
    print("=" * 50)
    
    payload = {
        "user_id": 1,
        "conversation_id": "test-conv-123",
        "messages": [
            {
                "role": "user",
                "content": question
            }
        ],
        "model": "gpt-4",
        "token_id": 10
    }
    
    response = requests.post(
        f"{BASE_URL}/webhook/message-hook",
        json=payload,
        headers={"Content-Type": "application/json"}
    )
    
    print(f"状态码: {response.status_code}")
    result = response.json()
    print(f"Modified: {result.get('modified')}")
    print(f"Abort: {result.get('abort')}")
    
    if result.get('messages'):
        for i, msg in enumerate(result['messages']):
            print(f"\n消息 {i+1}:")
            print(f"角色: {msg.get('role')}")
            print(f"内容:\n{msg.get('content')}")
    print()


def test_search(query):
    """测试搜索端点"""
    print("=" * 50)
    print(f"测试搜索: {query}")
    print("=" * 50)
    
    payload = {"query": query}
    
    response = requests.post(
        f"{BASE_URL}/webhook/test",
        json=payload,
        headers={"Content-Type": "application/json"}
    )
    
    print(f"状态码: {response.status_code}")
    result = response.json()
    
    if result.get('success'):
        print(f"查询: {result.get('query')}")
        print(f"\n格式化结果:\n{result.get('formatted')}")
    else:
        print(f"错误: {result.get('error')}")
    print()


if __name__ == "__main__":
    try:
        # 测试健康检查
        test_health()
        
        # 测试不同类型的问题
        test_questions = [
            "为什么天空是蓝色的？",
            "什么是人工智能？",
            "Python是什么？",
            "如何学习编程？",
            "今天天气怎么样？",  # 不应该触发搜索
            "What is machine learning?",
        ]
        
        for question in test_questions:
            test_message_hook(question)
        
        # 测试直接搜索
        test_search("人工智能的发展历史")
        
    except requests.exceptions.ConnectionError:
        print("❌ 无法连接到服务器，请确认Flask服务正在运行")
        print("   运行命令: python app.py")
    except Exception as e:
        print(f"❌ 测试失败: {str(e)}")
