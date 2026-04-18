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
    console.log('copyToClipboard called with label:', label, 'text length:', text?.length);
    
    if (!text) {
      Toast.warning(t('内容为空，无法复制'));
      return;
    }

    // 尝试使用现代 Clipboard API
    if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
      console.log('Trying Clipboard API...');
      try {
        await navigator.clipboard.writeText(text);
        console.log('Clipboard API success');
        Toast.success(t('已复制') + label);
        return;
      } catch (err) {
        console.warn('Clipboard API failed, falling back to execCommand:', err);
      }
    } else {
      console.log('Clipboard API not available, using fallback');
    }

    // 降级方案：使用 execCommand (兼容旧浏览器和非 HTTPS 环境)
    console.log('Using execCommand fallback...');
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
      
      console.log('execCommand result:', successful);
      
      if (successful) {
        Toast.success(t('已复制') + label);
      } else {
        Toast.error(t('复制失败'));
      }
    } catch (err) {
      console.error('Copy failed:', err);
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
            <div style={{ marginBottom: 20 }}>
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  marginBottom: 8,
                }}
              >
                <Text strong>{t('请求内容')}</Text>
                <Button
                  icon={<IconCopy />}
                  size='small'
                  onClick={async () => {
                    await copyToClipboard(detail.request_body, t('请求内容'));
                  }}
                >
                  {t('复制')}
                </Button>
              </div>
              <pre
                style={{
                  background: '#f5f5f5',
                  padding: 12,
                  borderRadius: 4,
                  overflow: 'auto',
                  maxHeight: 300,
                  fontSize: 12,
                  lineHeight: 1.5,
                }}
              >
                {formatJSON(detail.request_body)}
              </pre>
            </div>

            {detail.extracted_content && (
              <div style={{ marginBottom: 20 }}>
                <div
                  style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    marginBottom: 8,
                  }}
                >
                  <Text strong style={{ color: '#1890ff' }}>
                    {t('提取的文本内容')}
                  </Text>
                  <Button
                    icon={<IconCopy />}
                    size='small'
                    onClick={async () => {
                      await copyToClipboard(
                        detail.extracted_content,
                        t('提取的文本内容')
                      );
                    }}
                  >
                    {t('复制')}
                  </Button>
                </div>
                <pre
                  style={{
                    background: '#e6f7ff',
                    padding: 12,
                    borderRadius: 4,
                    overflow: 'auto',
                    maxHeight: 300,
                    fontSize: 12,
                    lineHeight: 1.5,
                    border: '1px solid #91d5ff',
                  }}
                >
                  {detail.extracted_content || t('无内容')}
                </pre>
              </div>
            )}

            <div>
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  marginBottom: 8,
                }}
              >
                <Text strong>{t('响应内容（原始）')}</Text>
                <Button
                  icon={<IconCopy />}
                  size='small'
                  onClick={async () => {
                    await copyToClipboard(detail.response_body, t('响应内容'));
                  }}
                >
                  {t('复制')}
                </Button>
              </div>
              <pre
                style={{
                  background: '#f5f5f5',
                  padding: 12,
                  borderRadius: 4,
                  overflow: 'auto',
                  maxHeight: 300,
                  fontSize: 12,
                  lineHeight: 1.5,
                }}
              >
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
