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
import { Form, Button, Card, Space, TextArea, InputNumber, Switch, Select, Typography, Modal } from '@douyinfe/semi-ui';
import { useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';
import './MessageHook.css';

const { Text } = Typography;

// Lua脚本模板 - 简洁版本，说明信息将显示在文本框外部
const LUA_TEMPLATE = `-- 获取输入的消息数组
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
    -- 不修改消息
    output = {
        modified = false,
        messages = messages,
        abort = false
    }
end`;

// HTTP模板 - 只保留URL，说明信息将显示在文本框外部
const HTTP_TEMPLATE = `https://your-api-server.com/webhook/message-hook`;

// Lua脚本说明文档
const LUA_HELP_TEXT = `【Lua脚本说明】

1. 脚本可以直接访问全局变量 'input'（系统自动注入）
2. 脚本必须设置全局变量 'output'（系统会读取此变量）
3. 不需要定义函数，直接编写处理逻辑即可

输入参数 (input)：
• input.user_id         - 用户ID (number, 必填)
• input.conversation_id - 对话ID (string, 可选)
• input.messages        - 消息数组 (array, 必填)
  - [].role             - 角色: "system", "user", "assistant", "tool"
  - [].content          - 消息内容 (string)
• input.model           - 模型名称 (string, 必填)
• input.token_id        - 令牌ID (number, 可选)

输出参数 (output) - 必须设置此全局变量：
• output.modified  - 是否修改了消息 (boolean, 必填)
• output.messages  - 修改后的消息数组 (array, modified=true时必填)
• output.abort     - 是否中止请求 (boolean, 必填)
• output.reason    - 中止原因 (string, abort=true时建议提供)`;

// HTTP钩子说明文档
const HTTP_HELP_TEXT = `【HTTP钩子说明】

在"内容"框中填写你的完整URL（支持HTTPS和HTTP）

默认白名单端口：
系统默认允许内网/本地地址使用端口 55566-55569，无需额外配置。
示例：http://127.0.0.1:55566/mywebhook/filter
     http://192.168.1.100:55569/mywebhook/process

安全配置说明：
系统默认启用SSRF保护，只允许访问公网HTTPS地址。
如需访问其他内网服务，请联系管理员配置以下环境变量

URL示例：
• https://api.example.com/hooks/filter
• http://127.0.0.1:55566/mywebhook/filter    （默认白名单端口）
• http://192.168.1.100:55567/mywebhook/xxx   （默认白名单端口）
• https://192.168.1.100:8080/hook            （需添加到白名单）
• http://localhost:3000/hook                 （需允许HTTP + 添加到白名单）

系统发送的请求格式：
POST {your-url}
Content-Type: application/json

{
  "user_id": 1,
  "conversation_id": "conv-123",
  "messages": [{"role": "user", "content": "Hello"}],
  "model": "gpt-4",
  "token_id": 10
}

你的API必须返回：
HTTP/1.1 200 OK
Content-Type: application/json

{
  "modified": true,
  "messages": [{"role": "system", "content": "..."}, ...],
  "abort": false,
  "reason": ""
}`;

const EditMessageHook = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { id } = useParams();
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [showTestModal, setShowTestModal] = useState(false);
  const [createdHookId, setCreatedHookId] = useState(null);
  const [selectedType, setSelectedType] = useState(1);
  const [formData, setFormData] = useState({
    name: '',
    description: '',
    type: 1,
    content: LUA_TEMPLATE,
    enabled: true,
    priority: 100,
    timeout: 5000,
    filter_users: '',
    filter_models: '',
    filter_tokens: '',
  });

  useEffect(() => {
    if (id) {
      loadHook();
    }
  }, [id]);

  const loadHook = async () => {
    setLoading(true);
    try {
      const res = await API.get(`/api/message-hooks/${id}`);
      const { success, message, data } = res.data;
      if (success) {
        // 转换content中的\n为真实换行符
        if (data.content) {
          data.content = data.content.replace(/\\n/g, '\n');
        }
        setFormData(data);
        setSelectedType(data.type || 1);
      } else {
        showError(message || t('加载失败'));
      }
    } catch (error) {
      showError(t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async (values) => {
    setSubmitting(true);
    try {
      const payload = {
        ...values,
        filter_users: values.filter_users?.trim() || '',
        filter_models: values.filter_models?.trim() || '',
        filter_tokens: values.filter_tokens?.trim() || '',
      };

      const res = id
        ? await API.put(`/api/message-hooks/${id}`, payload)
        : await API.post('/api/message-hooks', payload);

      const { success, message, data } = res.data;
      if (success) {
        showSuccess(id ? t('更新成功') : t('创建成功'));
        if (!id && data?.id) {
          // 创建成功后，询问是否要测试
          setCreatedHookId(data.id);
          setShowTestModal(true);
        } else {
          navigate('/console/message-hooks');
        }
      } else {
        // 显示详细错误信息
        showError(message || (id ? t('更新失败') : t('创建失败')));
      }
    } catch (error) {
      // 显示详细错误信息
      const errorMsg = error.response?.data?.message || error.message || (id ? t('更新失败') : t('创建失败'));
      showError(errorMsg);
    } finally {
      setSubmitting(false);
    }
  };

  const handleTypeChange = (type, formApi) => {
    setSelectedType(type);
    // 当类型改变时，更新内容模板
    const newContent = type === 1 ? LUA_TEMPLATE : HTTP_TEMPLATE;
    const currentContent = formApi.getValue('content');
    
    // 如果当前内容是空的或者是另一种类型的模板，则替换
    if (!currentContent || 
        currentContent === LUA_TEMPLATE || 
        currentContent === HTTP_TEMPLATE ||
        currentContent.trim() === '') {
      formApi.setValue('content', newContent);
    }
  };

  const handleTestModalClose = () => {
    setShowTestModal(false);
    setCreatedHookId(null);
    navigate('/console/message-hooks');
  };

  const handleGoToTest = () => {
    setShowTestModal(false);
    navigate(`/console/message-hooks/test?hookId=${createdHookId}`);
  };

  return (
    <div className='mt-[60px] px-2'>
      <Card
        title={id ? t('编辑消息钩子') : t('创建消息钩子')}
        loading={loading}
      >
        <Form
          initValues={formData}
          onSubmit={handleSubmit}
          labelPosition='left'
          labelWidth='150px'
          getFormApi={(formApi) => {
            // 保存formApi引用以便在handleTypeChange中使用
            window._messageHookFormApi = formApi;
          }}
        >
          <Form.Input
            field='name'
            label={t('名称')}
            rules={[{ required: true, message: t('请输入名称') }]}
            placeholder={t('请输入钩子名称')}
          />

          <Form.TextArea
            field='description'
            label={t('描述')}
            placeholder={t('请输入钩子描述')}
            className='resizable-textarea'
            autosize
            style={{ resize: 'vertical' }}
          />

          <Form.Select
            field='type'
            label={t('类型')}
            rules={[{ required: true, message: t('请选择类型') }]}
            onChange={(type) => handleTypeChange(type, window._messageHookFormApi)}
          >
            <Select.Option value={1}>Lua</Select.Option>
            <Select.Option value={2}>HTTP</Select.Option>
          </Form.Select>

          {/* 显示对应类型的帮助文档 */}
          <div style={{ marginBottom: 16, padding: '12px', backgroundColor: 'var(--semi-color-fill-0)', borderRadius: '4px', fontSize: '12px', whiteSpace: 'pre-wrap', fontFamily: 'monospace', maxHeight: '300px', overflow: 'auto' }}>
            <Text>{selectedType === 1 ? LUA_HELP_TEXT : HTTP_HELP_TEXT}</Text>
          </div>

          <Form.TextArea
            field='content'
            label={t('内容')}
            rules={[{ required: true, message: t('请输入内容') }]}
            placeholder={selectedType === 1 ? t('请输入Lua脚本') : t('请输入HTTPS URL')}
            className='resizable-textarea'
            autosize
            style={{ fontFamily: 'monospace', resize: 'vertical' }}
          />

          <Form.Switch
            field='enabled'
            label={t('启用')}
          />

          <Form.InputNumber
            field='priority'
            label={t('优先级')}
            rules={[{ required: true, message: t('请输入优先级') }]}
            placeholder={t('数字越小优先级越高')}
            min={0}
            max={1000}
          />

          <Form.InputNumber
            field='timeout'
            label={t('超时时间(ms)')}
            rules={[{ required: true, message: t('请输入超时时间') }]}
            min={100}
            max={30000}
          />

          <Form.TextArea
            field='filter_users'
            label={t('用户过滤')}
            placeholder={t('JSON数组，例如: [1,2,3]，留空表示匹配所有用户')}
            className='resizable-textarea'
            autosize
            style={{ resize: 'vertical' }}
          />

          <Form.TextArea
            field='filter_models'
            label={t('模型过滤')}
            placeholder={t('JSON数组，例如: ["gpt-4","claude-3"]，留空表示匹配所有模型')}
            className='resizable-textarea'
            autosize
            style={{ resize: 'vertical' }}
          />

          <Form.TextArea
            field='filter_tokens'
            label={t('令牌过滤')}
            placeholder={t('JSON数组，例如: [1,2,3]，留空表示匹配所有令牌')}
            className='resizable-textarea'
            autosize
            style={{ resize: 'vertical' }}
          />

          <Space style={{ marginTop: 16 }}>
            <Button
              htmlType='submit'
              type='primary'
              theme='solid'
              loading={submitting}
            >
              {id ? t('更新') : t('创建')}
            </Button>
            <Button onClick={() => navigate('/console/message-hooks')}>
              {t('取消')}
            </Button>
          </Space>
        </Form>

        {/* 测试确认模态框 */}
        <Modal
          title={t('钩子创建成功')}
          visible={showTestModal}
          onCancel={handleTestModalClose}
          footer={
            <Space>
              <Button onClick={handleTestModalClose}>
                {t('稍后测试')}
              </Button>
              <Button type='primary' onClick={handleGoToTest}>
                {t('立即测试')}
              </Button>
            </Space>
          }
        >
          <Text>{t('钩子已成功创建！是否要立即测试钩子的运行效果？')}</Text>
        </Modal>
      </Card>
    </div>
  );
};

export default EditMessageHook;
