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
import { Table, Button, Space, Switch, Tag, Popconfirm, Typography } from '@douyinfe/semi-ui';
import { IconPlus, IconEdit, IconDelete, IconPlay } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../helpers';
import { useNavigate } from 'react-router-dom';

const { Text } = Typography;

const MessageHooksTable = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [hooks, setHooks] = useState([]);
  const [pagination, setPagination] = useState({
    currentPage: 1,
    pageSize: 10,
    total: 0,
  });

  const loadHooks = async (page = 1) => {
    setLoading(true);
    try {
      const res = await API.get(`/api/message-hooks?page=${page}&page_size=${pagination.pageSize}`);
      const { success, message, data } = res.data;
      if (success) {
        setHooks(data || []);
        setPagination({
          ...pagination,
          currentPage: page,
          total: res.data.total || 0,
        });
      } else {
        showError(message || t('加载失败'));
      }
    } catch (error) {
      showError(t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadHooks(1);
  }, []);

  const handleToggleEnabled = async (hook) => {
    try {
      const updatedHook = { ...hook, enabled: !hook.enabled };
      const res = await API.put(`/api/message-hooks/${hook.id}`, updatedHook);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('更新成功'));
        loadHooks(pagination.currentPage);
      } else {
        showError(message || t('更新失败'));
      }
    } catch (error) {
      showError(t('更新失败'));
    }
  };

  const handleDelete = async (id) => {
    try {
      const res = await API.delete(`/api/message-hooks/${id}`);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('删除成功'));
        loadHooks(pagination.currentPage);
      } else {
        showError(message || t('删除失败'));
      }
    } catch (error) {
      showError(t('删除失败'));
    }
  };

  const columns = [
    {
      title: t('名称'),
      dataIndex: 'name',
      key: 'name',
      render: (text) => <Text strong>{text}</Text>,
    },
    {
      title: t('类型'),
      dataIndex: 'type',
      key: 'type',
      render: (type) => (
        <Tag color={type === 1 ? 'blue' : 'green'}>
          {type === 1 ? 'Lua' : 'HTTP'}
        </Tag>
      ),
    },
    {
      title: t('优先级'),
      dataIndex: 'priority',
      key: 'priority',
    },
    {
      title: t('超时(ms)'),
      dataIndex: 'timeout',
      key: 'timeout',
    },
    {
      title: t('调用次数'),
      dataIndex: 'call_count',
      key: 'call_count',
    },
    {
      title: t('成功率'),
      dataIndex: 'success_count',
      key: 'success_rate',
      render: (_, record) => {
        if (record.call_count === 0) return '-';
        const rate = ((record.success_count / record.call_count) * 100).toFixed(1);
        return `${rate}%`;
      },
    },
    {
      title: t('平均耗时(ms)'),
      dataIndex: 'avg_duration',
      key: 'avg_duration',
      render: (duration) => duration ? duration.toFixed(2) : '-',
    },
    {
      title: t('状态'),
      dataIndex: 'enabled',
      key: 'enabled',
      render: (enabled, record) => (
        <Switch
          checked={enabled}
          onChange={() => handleToggleEnabled(record)}
        />
      ),
    },
    {
      title: t('操作'),
      key: 'action',
      render: (_, record) => (
        <Space>
          <Button
            theme='borderless'
            type='primary'
            size='small'
            icon={<IconEdit />}
            onClick={() => navigate(`/console/message-hooks/${record.id}/edit`)}
          />
          <Button
            theme='borderless'
            type='secondary'
            size='small'
            icon={<IconPlay />}
            onClick={() => navigate(`/console/message-hooks/test?hookId=${record.id}`)}
          />
          <Popconfirm
            title={t('确定删除此钩子吗？')}
            onConfirm={() => handleDelete(record.id)}
            okText={t('确定')}
            cancelText={t('取消')}
          >
            <Button
              theme='borderless'
              type='danger'
              size='small'
              icon={<IconDelete />}
            />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div style={{ marginBottom: 16 }}>
        <Button
          theme='solid'
          type='primary'
          icon={<IconPlus />}
          onClick={() => navigate('/console/message-hooks/create')}
        >
          {t('创建钩子')}
        </Button>
      </div>
      <Table
        columns={columns}
        dataSource={hooks}
        loading={loading}
        pagination={{
          currentPage: pagination.currentPage,
          pageSize: pagination.pageSize,
          total: pagination.total,
          onPageChange: (page) => loadHooks(page),
        }}
        rowKey='id'
      />
    </div>
  );
};

export default MessageHooksTable;
