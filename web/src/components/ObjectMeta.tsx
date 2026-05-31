import { useEffect, useState } from "react";
import { Modal, Descriptions, Skeleton, Empty } from 'antd';
import { useTranslation } from 'react-i18next';
import { apiClient, buildMetaPath } from '../api/client';

interface Props {
  bucket: string;
  objectKey: string;
  visible: boolean;
  onClose: () => void;
}

interface MetaData {
  key: string;
  content_type: string;
  content_length: number;
  etag: string;
  last_modified: string;
}

function formatBytes(bytes: number): string {
  if (!bytes) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let i = 0; let val = bytes;
  while (val >= 1024 && i < units.length - 1) { val /= 1024; i++; }
  return `${val.toFixed(1)} ${units[i]}`;
}

export default function ObjectMeta({ bucket, objectKey, visible, onClose }: Props) {
  const { t } = useTranslation();
  const [meta, setMeta] = useState<MetaData | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!visible) return;
    setLoading(true);
    apiClient.get<MetaData>(buildMetaPath(bucket, objectKey))
      .then((resp) => setMeta(resp.data))
      .catch(() => setMeta(null))
      .finally(() => setLoading(false));
  }, [bucket, objectKey, visible]);

  return (
    <Modal title={t('meta.title')} open={visible} onCancel={onClose} footer={null} width={600}>
      {loading ? <Skeleton active /> : meta ? (
        <Descriptions bordered column={1} size="small">
          <Descriptions.Item label={t('meta.key')}>{meta.key}</Descriptions.Item>
          <Descriptions.Item label={t('meta.contentType')}>{meta.content_type}</Descriptions.Item>
          <Descriptions.Item label={t('meta.size')}>{formatBytes(meta.content_length)}</Descriptions.Item>
          <Descriptions.Item label={t('meta.etag')}>{meta.etag}</Descriptions.Item>
          <Descriptions.Item label={t('meta.lastModified')}>{meta.last_modified ? new Date(meta.last_modified).toLocaleString() : t('common.dash')}</Descriptions.Item>
        </Descriptions>
      ) : <Empty description={t('errors.loadMeta')} />}
    </Modal>
  );
}
