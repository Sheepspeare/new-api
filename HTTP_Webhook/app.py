"""
Flask HTTP Webhook服务 - 集成Tavily搜索增强prompt
用于new-api项目的消息钩子功能测试
"""

from flask import Flask, request, jsonify
from tavily import TavilyClient
import os
import re
from dotenv import load_dotenv
import logging

# 加载环境变量
load_dotenv()

# 配置日志
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

app = Flask(__name__)

# 初始化Tavily客户端
TAVILY_API_KEY = os.getenv('TAVILY_API_KEY')
if not TAVILY_API_KEY:
    logger.warning("TAVILY_API_KEY not found in environment variables")
    tavily_client = None
else:
    tavily_client = TavilyClient(api_key=TAVILY_API_KEY)
    logger.info("Tavily client initialized successfully")


def should_search(content):
    """
    判断是否需要进行搜索
    检测问题模式：为什么xxx、xxx是什么、什么是xxx等
    """
    if not content:
        return False, None
    
    # 问题模式列表
    patterns = [
        r'为什么(.+)',           # 为什么xxx
        r'(.+)是什么',           # xxx是什么
        r'什么是(.+)',           # 什么是xxx
        r'(.+)怎么样',           # xxx怎么样
        r'如何(.+)',             # 如何xxx
        r'怎么(.+)',             # 怎么xxx
        r'(.+)的原因',           # xxx的原因
        r'(.+)的定义',           # xxx的定义
        r'解释(.+)',             # 解释xxx
        r'介绍(.+)',             # 介绍xxx
        r'what is (.+)',         # what is xxx (英文)
        r'why (.+)',             # why xxx (英文)
        r'how (.+)',             # how xxx (英文)
        r'explain (.+)',         # explain xxx (英文)
    ]
    
    for pattern in patterns:
        match = re.search(pattern, content, re.IGNORECASE)
        if match:
            # 提取关键词
            keyword = match.group(1).strip()
            # 过滤掉过短的关键词
            if len(keyword) > 1:
                logger.info(f"Detected search pattern: '{pattern}' -> keyword: '{keyword}'")
                return True, keyword
    
    return False, None


def search_with_tavily(query, max_results=3):
    """
    使用Tavily进行搜索
    """
    if not tavily_client:
        logger.error("Tavily client not initialized")
        return None
    
    try:
        logger.info(f"Searching with Tavily: {query}")
        response = tavily_client.search(
            query=query,
            max_results=max_results,
            search_depth="basic",  # 可选: "basic" 或 "advanced"
            include_answer=True,   # 包含AI生成的答案摘要
            include_raw_content=False
        )
        logger.info(f"Tavily search completed: {len(response.get('results', []))} results")
        return response
    except Exception as e:
        logger.error(f"Tavily search failed: {str(e)}")
        return None


def format_search_results(search_response):
    """
    格式化搜索结果为可读文本
    """
    if not search_response:
        return ""
    
    formatted = "\n\n【搜索增强信息】\n"
    
    # 添加AI生成的答案摘要
    if 'answer' in search_response and search_response['answer']:
        formatted += f"\n📝 摘要: {search_response['answer']}\n"
    
    # 添加搜索结果
    results = search_response.get('results', [])
    if results:
        formatted += "\n📚 相关资料:\n"
        for i, result in enumerate(results[:3], 1):  # 最多3条
            title = result.get('title', '无标题')
            url = result.get('url', '')
            content = result.get('content', '')
            
            formatted += f"\n{i}. {title}\n"
            if content:
                # 限制内容长度
                content_preview = content[:2000] + "..." if len(content) > 2000 else content
                formatted += f"   {content_preview}\n"
            if url:
                formatted += f"   来源: {url}\n"
    
    formatted += "\n---\n"
    return formatted


@app.route('/health', methods=['GET'])
def health_check():
    """健康检查端点"""
    return jsonify({
        'status': 'healthy',
        'tavily_enabled': tavily_client is not None
    }), 200


@app.route('/webhook/message-hook', methods=['POST'])
def message_hook():
    """
    消息钩子端点
    接收new-api发送的消息，进行处理后返回
    """
    try:
        # 解析请求数据
        data = request.get_json()
        
        if not data:
            logger.error("No JSON data received")
            return jsonify({
                'error': 'No JSON data provided'
            }), 400
        
        logger.info(f"Received hook request: userId={data.get('user_id')}, model={data.get('model')}")
        
        # 提取消息列表
        messages = data.get('messages', [])
        if not messages:
            logger.warning("No messages in request")
            return jsonify({
                'modified': False,
                'messages': messages,
                'abort': False
            }), 200
        
        # 检查最后一条用户消息
        modified = False
        for i in range(len(messages) - 1, -1, -1):
            msg = messages[i]
            if msg.get('role') == 'user':
                content = msg.get('content', '')
                
                # 判断是否需要搜索
                need_search, keyword = should_search(content)
                
                if need_search and keyword and tavily_client:
                    logger.info(f"Triggering search for keyword: {keyword}")
                    
                    # 执行搜索
                    search_results = search_with_tavily(keyword)
                    
                    if search_results:
                        # 格式化搜索结果
                        enhanced_info = format_search_results(search_results)
                        
                        # 将搜索结果添加到用户消息后面
                        messages[i]['content'] = content + enhanced_info
                        modified = True
                        
                        logger.info(f"Message enhanced with search results (length: {len(enhanced_info)})")
                    else:
                        logger.warning("Search returned no results")
                
                # 只处理最后一条用户消息
                break
        
        # 返回处理结果
        response = {
            'modified': modified,
            'messages': messages,
            'abort': False
        }
        
        logger.info(f"Returning response: modified={modified}")
        return jsonify(response), 200
        
    except Exception as e:
        logger.error(f"Error processing hook: {str(e)}", exc_info=True)
        return jsonify({
            'error': str(e),
            'modified': False,
            'messages': data.get('messages', []),
            'abort': False
        }), 500


@app.route('/webhook/test', methods=['POST'])
def test_search():
    """
    测试端点 - 直接测试Tavily搜索功能
    """
    try:
        data = request.get_json()
        query = data.get('query', '')
        
        if not query:
            return jsonify({'error': 'No query provided'}), 400
        
        if not tavily_client:
            return jsonify({'error': 'Tavily client not initialized'}), 500
        
        # 执行搜索
        results = search_with_tavily(query)
        
        if results:
            formatted = format_search_results(results)
            return jsonify({
                'success': True,
                'query': query,
                'raw_results': results,
                'formatted': formatted
            }), 200
        else:
            return jsonify({
                'success': False,
                'error': 'Search failed'
            }), 500
            
    except Exception as e:
        logger.error(f"Test search error: {str(e)}", exc_info=True)
        return jsonify({'error': str(e)}), 500


if __name__ == '__main__':
    # 使用默认白名单端口（55566-55569）避免SSRF拦截
    port = int(os.getenv('PORT', 55566))
    debug = os.getenv('DEBUG', 'False').lower() == 'true'
    
    logger.info(f"Starting Flask server on port {port}")
    logger.info(f"Webhook URL: http://127.0.0.1:{port}/webhook/message-hook")
    logger.info("This port is in the default SSRF whitelist (55566-55569)")
    app.run(host='0.0.0.0', port=port, debug=debug)
