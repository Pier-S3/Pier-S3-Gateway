import React, { useState } from 'react';
import { Table, Button, Modal, message, Input, Skeleton, Alert, Dropdown, Empty, Tooltip } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  FolderFilled,
  FileOutlined,
  DownloadOutlined,
  DeleteOutlined,
  InfoCircleOutlined,
  MoreOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { ObjectInfo } from '../store/browser';
import { apiClient, buildObjectPath, downloadObject, errorMessage } from '../api/client';
import ObjectMeta from './ObjectMeta';
import FilePreview from './FilePreview';

const { Search } = Input;

interface Props {
  bucket: string;
  prefix: string;
  objects: ObjectInfo[];
  prefixes: string[];
  loading: boolean;
  canWrite: boolean;
  truncated: boolean;
  onNavigatePrefix: (prefix: string) => void;
  onLoadMore: () => void;
  onRefresh: () => void;
}

interface Row {
  key: string;
  name: string;
  size: number;
  lastModified: string;
  isPrefix: boolean;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let i = 0;
  let val = bytes;
  while (val >= 1024 && i < units.length - 1) {
    val /= 1024;
    i++;
  }
  return `${val.toFixed(1)} ${units[i]}`;
}

function fileName(key: string): string {
  const parts = key.replace(/\/$/, '').split('/');
  return parts[parts.length - 1] || key;
}

export default function ObjectBrowser({
  bucket, prefix, objects, prefixes, loading, canWrite, truncated,
  onNavigatePrefix, onLoadMore, onRefresh,
}: Props) {
  const { t } = useTranslation();
  const [searchText, setSearchText] = useState('');
  const [metaKey, setMetaKey] = useState<string | null>(null);
  const [preview, setPreview] = useState<{ key: string; size: number } | null>(null);

  const handleDelete = (key: string) => {
    Modal.confirm({
      title: t('browser.deleteTitle'),
      content: t('browser.deleteConfirm', { name: fileName(key) }),
      okText: t('common.delete'),
      okType: 'danger',
      cancelText: t('common.cancel'),
      onOk: async () => {
        try {
          await apiClient.delete(buildObjectPath(bucket, key));
          message.success(t('messages.deleted'));
          onRefresh();
        } catch (err) {
          if (err && typeof err === 'object' && 'response' in err
            && (err as { response?: { status?: number } }).response?.status === 403) {
            message.error(t('errors.forbidden'));
          } else {
            message.error(errorMessage(err, t('errors.deleteFailed')));
          }
        }
      },
    });
  };

  const handleDownload = async (key: string) => {
    try {
      await downloadObject(bucket, key, fileName(key));
    } catch (err) {
      message.error(errorMessage(err, t('errors.downloadFailed')));
    }
  };

  // The primary, non-destructive affordance for a row. Folders navigate;
  // files open the (reversible, informational) metadata panel. A single click
  // must NEVER start a download - downloads are an explicit action only.
  const handleRowActivate = (record: Row) => {
    if (record.isPrefix) {
      onNavigatePrefix(record.key);
    } else {
      setPreview({ key: record.key, size: record.size });
    }
  };

  // Build table data: prefixes first, then objects.
  // Search only filters the in-memory page; while the listing is truncated we
  // surface a notice so users understand unloaded objects are not searched.
  const needle = searchText.toLowerCase();
  const prefixRows: Row[] = prefixes
    .filter((p) => !needle || p.toLowerCase().includes(needle))
    .map((p) => ({
      key: p,
      name: p.replace(prefix, '').replace(/\/$/, ''),
      size: -1,
      lastModified: '',
      isPrefix: true,
    }));

  const objectRows: Row[] = objects
    .filter((o) => !needle || o.key.toLowerCase().includes(needle))
    .map((o) => ({
      key: o.key,
      name: fileName(o.key),
      size: o.size_bytes,
      lastModified: o.last_modified ? new Date(o.last_modified).toLocaleString() : '',
      isPrefix: false,
    }));

  const dataSource: Row[] = [...prefixRows, ...objectRows];

  if (loading && dataSource.length === 0) return <Skeleton active paragraph={{ rows: 6 }} />;

  const rowActions = (record: Row) => {
    const items = [
      {
        key: 'download',
        icon: <DownloadOutlined />,
        label: t('common.download'),
        onClick: () => { void handleDownload(record.key); },
      },
      {
        key: 'info',
        icon: <InfoCircleOutlined />,
        label: t('common.info'),
        onClick: () => setMetaKey(record.key),
      },
      ...(canWrite
        ? [{
          key: 'delete',
          icon: <DeleteOutlined />,
          label: t('common.delete'),
          danger: true,
          onClick: () => handleDelete(record.key),
        }]
        : []),
    ];
    return items;
  };

  const columns: ColumnsType<Row> = [
    {
      title: t('common.name'),
      dataIndex: 'name',
      key: 'name',
      sorter: (a, b) => {
        if (a.isPrefix !== b.isPrefix) return a.isPrefix ? -1 : 1;
        return a.name.localeCompare(b.name);
      },
      defaultSortOrder: 'ascend',
      ellipsis: true,
      render: (name: string, record) => (
        <span className="file-name-cell">
          <span className={`file-icon-wrap ${record.isPrefix ? 'folder' : 'file'}`}>
            {record.isPrefix
              ? <FolderFilled />
              : <FileOutlined />}
          </span>
          <span>{name}</span>
        </span>
      ),
    },
    {
      title: t('common.lastModified'),
      dataIndex: 'lastModified',
      key: 'lastModified',
      sorter: (a, b) => a.lastModified.localeCompare(b.lastModified),
      width: 220,
      render: (value: string) => (
        <span className="col-mono">{value || t('common.dash')}</span>
      ),
    },
    {
      title: t('common.size'),
      dataIndex: 'size',
      key: 'size',
      sorter: (a, b) => a.size - b.size,
      render: (size: number, record) => (
        <span className="col-mono">{record.isPrefix ? t('common.dash') : formatBytes(size)}</span>
      ),
      width: 130,
    },
    {
      title: '',
      key: 'actions',
      width: 96,
      align: 'right',
      render: (_: unknown, record) => {
        if (record.isPrefix) return null;
        return (
          <div
            className="row-actions"
            onClick={(e) => e.stopPropagation()}
            onKeyDown={(e) => e.stopPropagation()}
          >
            <Tooltip title={t('common.download')}>
              <Button
                className="row-download-btn"
                type="text"
                size="small"
                icon={<DownloadOutlined />}
                onClick={(e) => {
                  e.stopPropagation();
                  void handleDownload(record.key);
                }}
                aria-label={t('common.downloadFile', { name: record.name })}
              />
            </Tooltip>
            <Dropdown menu={{ items: rowActions(record) }} trigger={['click']}>
              <Button
                type="text"
                size="small"
                icon={<MoreOutlined />}
                onClick={(e) => e.stopPropagation()}
                aria-label={t('common.moreActions', { name: record.name })}
              />
            </Dropdown>
          </div>
        );
      },
    },
  ];

  const isEmpty = !loading && dataSource.length === 0;

  return (
    <>
      <div style={{ display: 'flex', gap: 12, marginBottom: 16, flexWrap: 'wrap', alignItems: 'center' }}>
        <Search
          placeholder={t('common.search')}
          allowClear
          onChange={(e) => setSearchText(e.target.value)}
          style={{ maxWidth: 400, flex: 1 }}
        />
      </div>
      {truncated && searchText && (
        <Alert
          type="info"
          showIcon
          message={t('browser.searchTruncated')}
          style={{ marginBottom: 16 }}
        />
      )}
      {isEmpty ? (
        <Empty
          description={t('browser.emptyTitle')}
          style={{ marginTop: 48 }}
        >
          <span style={{ color: 'var(--dbx-muted)' }}>{t('browser.emptyHint')}</span>
        </Empty>
      ) : (
        <Table<Row>
          className="file-table"
          dataSource={dataSource}
          columns={columns}
          pagination={false}
          loading={loading}
          size="middle"
          scroll={{ x: 720 }}
          rowKey="key"
          rowClassName={(record) => (record.isPrefix ? 'row-folder' : 'row-file')}
          onRow={(record) => ({
            onClick: () => handleRowActivate(record),
            onKeyDown: (e: React.KeyboardEvent) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                handleRowActivate(record);
              }
            },
            tabIndex: 0,
            role: 'button',
            'aria-label': record.isPrefix
              ? t('common.openFolder', { name: record.name })
              : t('preview.open', { name: record.name }),
          })}
        />
      )}
      {truncated && (
        <Button onClick={onLoadMore} loading={loading} style={{ marginTop: 16 }}>
          {t('common.loadMore')}
        </Button>
      )}
      {metaKey && (
        <ObjectMeta
          bucket={bucket}
          objectKey={metaKey}
          visible={!!metaKey}
          onClose={() => setMetaKey(null)}
        />
      )}
      {preview && (
        <FilePreview
          bucket={bucket}
          objectKey={preview.key}
          size={preview.size}
          visible={!!preview}
          onClose={() => setPreview(null)}
        />
      )}
    </>
  );
}
