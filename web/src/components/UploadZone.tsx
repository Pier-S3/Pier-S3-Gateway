import { useState, useCallback } from "react";
import { Upload, message, Progress } from 'antd';
import type { UploadRequestOption } from 'rc-upload/lib/interface';
import { CloudUploadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { apiClient, buildObjectPath, errorMessage } from '../api/client';

const { Dragger } = Upload;

interface Props {
  bucket: string;
  prefix: string;
  onUploaded: () => void;
}

interface UploadItem {
  name: string;
  progress: number;
  status: 'uploading' | 'done' | 'error';
}

export default function UploadZone({ bucket, prefix, onUploaded }: Props) {
  const { t } = useTranslation();
  const [uploads, setUploads] = useState<UploadItem[]>([]);

  const customUpload = useCallback(async (options: UploadRequestOption) => {
    const { file, onSuccess, onError, onProgress } = options;
    const rcFile = file as File;
    const key = prefix + rcFile.name;

    setUploads((prev) => [...prev, { name: rcFile.name, progress: 0, status: 'uploading' }]);

    try {
      await apiClient.put(buildObjectPath(bucket, key), rcFile, {
        headers: { 'Content-Type': rcFile.type || 'application/octet-stream' },
        onUploadProgress: (e) => {
          const percent = Math.round((e.loaded * 100) / (e.total || 1));
          onProgress?.({ percent });
          setUploads((prev) =>
            prev.map((u) => (u.name === rcFile.name ? { ...u, progress: percent } : u)),
          );
        },
      });
      onSuccess?.(null);
      setUploads((prev) =>
        prev.map((u) => (u.name === rcFile.name ? { ...u, progress: 100, status: 'done' } : u)),
      );
      message.success(t('messages.uploaded', { name: rcFile.name }));
      onUploaded();
    } catch (err) {
      onError?.(err instanceof Error ? err : new Error('upload failed'));
      setUploads((prev) =>
        prev.map((u) => (u.name === rcFile.name ? { ...u, status: 'error' } : u)),
      );
      if (err && typeof err === 'object' && 'response' in err
        && (err as { response?: { status?: number } }).response?.status === 403) {
        message.error(t('errors.forbidden'));
      } else {
        message.error(errorMessage(err, t('errors.uploadFailed', { name: rcFile.name })));
      }
    }
  }, [bucket, prefix, onUploaded, t]);

  return (
    <div className="dbx-dropzone" style={{ marginBottom: 16 }}>
      {/* A single upload affordance: the drop area is itself click-to-select and
          drag-and-drop, so a separate button would just duplicate it. */}
      <Dragger
        multiple
        customRequest={(options) => { void customUpload(options); }}
        showUploadList={false}
      >
        <p className="ant-upload-drag-icon"><CloudUploadOutlined /></p>
        <p className="ant-upload-text">{t('browser.dropHint')}</p>
        <p className="ant-upload-hint">{t('browser.dropSubHint')}</p>
      </Dragger>
      {uploads.filter((u) => u.status === 'uploading').map((u) => (
        <div key={u.name} style={{ marginTop: 8 }}>
          <span>{u.name}</span>
          <Progress percent={u.progress} size="small" />
        </div>
      ))}
    </div>
  );
}
