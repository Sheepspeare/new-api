/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useState, useEffect } from 'react';
import { Card, Button, TextArea, Typography, Space, Select } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router-dom';
import { API, showError, showSuccess } from '../../helpers';
import './MessageHook.css';

const { Text, Title } = Typography;

const TestMessageHook = () => {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const [testing, setTesting] = useState(false);
  const [hooks, setHooks] = useState([]);
  const [selectedHookId, setSelectedHookId] = useState(null);
  const [hookConfig, setHookConfig] = useState(`{\n  "name": "test-hook",\n  "type": 1,\n  "timeout": 5000\n}`);
  const [hookContent, setHookContent] = useState(`-- 简单的测试脚本\nlocal messages = input.messages\noutput = {\n    modified = false,\n    messages = messages,\n    abort = false\n}`);
  const [testInput, setTestInput] = useState(`{
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
}`);
  const [result, setResult] = useState(null);

  // 加载钩子列表
  useEffect(() => {
    loadHooks();
  }, []);

  // 处理URL参数中的hookId
  useEffect(() => {
    const hookId = searchParams.get('hookId');
    if (hookId && hooks.length > 0) {
      const hook = hooks.find(h => h.id === parseInt(hookId));
      if (hook) {
        setSelectedHookId(parseInt(hookId));
        // 处理content中的\n转换为真实换行符
        const content = hook.content.replace(/\\n/g, '\n');
        setHookContent(content);
        setHookConfig(JSON.stringify({
          name: hook.name,
          type: hook.type,
          timeout: hook.timeout
        }, null, 2));
      }
    }
  }, [searchParams, hooks]);

  const loadHooks = async () => {
    try {
      const res = await API.get('/api/message-hooks?page=1&page_size=100');
      const { success, data } = res.data;
      if (success) {
        setHooks(data || []);
      }
    } catch (error) {
      console.error('Failed to load hooks:', error);
    }
  };

  const handleHookSelect = (hookId) => {
    const hook = hooks.find(h => h.id === hookId);
    if (hook) {
      setSelectedHookId(hookId);
      // 处理content中的\n转换为真实换行符
      const content = hook.content.replace(/\\n/g, '\n');
      setHookContent(content);
      setHookConfig(JSON.stringify({
        name: hook.name,
        type: hook.type,
        timeout: hook.timeout
      }, null, 2));
    }
  };

  const loadTestTemplate = (type) => {
    if (type === 'simple') {
      setTestInput(`{
  "user_id": 1,
  "messages": [
    {
      "role": "user",
      "content": "Hello, world!"
    }
  ],
  "model": "gpt-4"
}`);
    } else if (type === 'conversation') {
      setTestInput(`{
  "user_id": 1,
  "conversation_id": "test-conversation-123",
  "messages": [
    {
      "role": "system",
      "content": "你是一个专业的AI助手"
    },
    {
      "role": "user",
      "content": "你好，请介绍一下你自己"
    },
    {
      "role": "assistant", 
      "content": "你好！我是一个AI助手，很高兴为您服务。"
    },
    {
      "role": "user",
      "content": "你能帮我做什么？"
    }
  ],
  "model": "gpt-4",
  "token_id": 1
}`);
    } else if (type === 'multiuser') {
      setTestInput(`{
  "user_id": 2,
  "conversation_id": "business-chat-456",
  "messages": [
    {
      "role": "user",
      "content": "请帮我分析一下这个商业计划"
    }
  ],
  "model": "claude-3",
  "token_id": 5
}`);
    }
  };

  const handleTest = async () => {
    setTesting(true);
    setResult(null);
    try {
      const hook = JSON.parse(hookConfig);
      hook.content = hookContent;
      const input = JSON.parse(testInput);

      const res = await API.post('/api/message-hooks/test', {
        hook,
        input,
      });

      const { success, message, data } = res.data;
      if (success) {
        setResult({
          success: true,
          output: data,
        });
        showSuccess(t('测试成功'));
      } else {
        setResult({
          success: false,
          error: message,
        });
        showError(message || t('测试失败'));
      }
    } catch (error) {
      const errorMsg = error.response?.data?.message || error.message || t('测试失败');
      setResult({
        success: false,
        error: errorMsg,
      });
      showError(errorMsg);
    } finally {
      setTesting(false);
    }
  };

  return (
    <div className='mt-[60px] px-2'>
      <Card title={t('测试消息钩子')}>
        {/* 钩子选择 */}
        <div style={{ marginBottom: 16 }}>
          <Title heading={6} style={{ marginBottom: 8 }}>{t('选择钩子')}</Title>
          <Select
            value={selectedHookId}
            onChange={handleHookSelect}
            placeholder={t('选择要测试的钩子')}
            style={{ width: '100%' }}
          >
            {hooks.map(hook => (
              <Select.Option key={hook.id} value={hook.id}>
                {hook.name} ({hook.type === 1 ? 'Lua' : 'HTTP'})
              </Select.Option>
            ))}
          </Select>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '20px' }}>
          {/* 左列：钩子配置 */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
            {/* 基本配置 */}
            <div>
              <Title heading={6} style={{ marginBottom: 8 }}>{t('基本配置 (JSON)')}</Title>
              <TextArea
                value={hookConfig}
                onChange={setHookConfig}
                className='resizable-textarea'
                autosize
                style={{ fontFamily: 'monospace', fontSize: '13px' }}
                placeholder={t('请输入钩子配置JSON')}
              />
            </div>

            {/* 脚本 / URL 内容 */}
            <div>
              <Title heading={6} style={{ marginBottom: 8 }}>{t('脚本 / URL 内容')}</Title>
              <TextArea
                value={hookContent}
                onChange={setHookContent}
                className='resizable-textarea'
                autosize
                style={{ fontFamily: 'monospace', fontSize: '13px' }}
                placeholder={t('请输入Lua脚本或HTTP URL')}
              />
            </div>
          </div>

          {/* 右列：测试输入 */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                <Title heading={6}>{t('测试输入')}</Title>
                <Space>
                  <Button size='small' onClick={() => loadTestTemplate('simple')}>
                    {t('简单对话')}
                  </Button>
                  <Button size='small' onClick={() => loadTestTemplate('conversation')}>
                    {t('多轮对话')}
                  </Button>
                  <Button size='small' onClick={() => loadTestTemplate('multiuser')}>
                    {t('多用户场景')}
                  </Button>
                </Space>
              </div>
              <TextArea
                value={testInput}
                onChange={setTestInput}
                className='resizable-textarea'
                autosize
                style={{ fontFamily: 'monospace', fontSize: '13px' }}
                placeholder={t('请输入测试输入JSON')}
              />
              <div style={{ marginTop: 8, padding: '8px', backgroundColor: 'var(--semi-color-fill-0)', borderRadius: '4px', fontSize: '12px', color: 'var(--semi-color-text-2)' }}>
                <Text strong>{t('参数说明：')}</Text>
                <div style={{ marginTop: 4 }}>
                  <Text>• user_id: {t('用户ID（必填）')}</Text><br/>
                  <Text>• conversation_id: {t('对话ID（可选）')}</Text><br/>
                  <Text>• messages: {t('消息数组（必填）')}</Text><br/>
                  <Text>• model: {t('模型名称（必填）')}</Text><br/>
                  <Text>• token_id: {t('令牌ID（可选）')}</Text>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* 执行测试按钮 - 固定在中间 */}
        <div style={{ marginTop: 20, marginBottom: 20 }}>
          <Button
            type='primary'
            theme='solid'
            onClick={handleTest}
            loading={testing}
            size='large'
            block
          >
            {t('执行测试')}
          </Button>
        </div>

        {/* 测试结果 */}
        {result && (
          <div>
            <Title heading={6} style={{ marginBottom: 8 }}>
              {result.success ? t('测试结果') : t('错误信息')}
            </Title>
            <TextArea
              value={JSON.stringify(result.success ? result.output : { error: result.error }, null, 2)}
              className='resizable-textarea readable-textarea'
              autosize
              style={{ fontFamily: 'monospace', fontSize: '13px' }}
              readonly
            />
            {result.success && (
              <div style={{ marginTop: 8, padding: '8px', backgroundColor: 'var(--semi-color-success-light-default)', borderRadius: '4px', fontSize: '12px', color: 'var(--semi-color-text-2)' }}>
                <Text strong>{t('返回参数说明：')}</Text>
                <div style={{ marginTop: 4 }}>
                  <Text>• modified: {t('是否修改了消息')}</Text><br/>
                  <Text>• messages: {t('修改后的消息数组')}</Text><br/>
                  <Text>• abort: {t('是否中止请求')}</Text><br/>
                  <Text>• reason: {t('中止原因（如果有）')}</Text>
                </div>
              </div>
            )}
          </div>
        )}
      </Card>
    </div>
  );
};

export default TestMessageHook;
