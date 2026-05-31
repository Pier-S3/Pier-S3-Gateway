import React, { useEffect, useRef, useState } from 'react';
import { Modal, Button, Spin, Empty, Typography, message } from 'antd';
import { DownloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { fetchObjectBlob, downloadObject, errorMessage } from '../api/client';
import {
  previewKindFor,
  MAX_TEXT_BYTES,
  MAX_PREVIEW_BYTES,
  type PreviewKind,
} from './previewKind';

const { Text } = Typography;

interface Props {
  bucket: string;
  objectKey: string;
  /** Object size in bytes, if known (used to skip oversized previews). */
  size?: number;
  visible: boolean;
  onClose: () => void;
}

type Status = 'loading' | 'ready' | 'too-large' | 'unsupported' | 'error';

function baseName(key: string): string {
  const parts = key.replace(/\/+$/, '').split('/');
  return parts[parts.length - 1] || key;
}

export default function FilePreview({ bucket, objectKey, size, visible, onClose }: Props) {
  const { t } = useTranslation();
  const kind: PreviewKind = previewKindFor(objectKey);
  const name = baseName(objectKey);

  const [status, setStatus] = useState<Status>('loading');
  const [url, setUrl] = useState<string | null>(null);
  const [textContent, setTextContent] = useState<string>('');
  // Track the active object URL so we always revoke exactly what we created.
  const urlRef = useRef<string | null>(null);

  useEffect(() => {
    if (!visible) return;
    let cancelled = false;

    const revoke = () => {
      if (urlRef.current) {
        URL.revokeObjectURL(urlRef.current);
        urlRef.current = null;
      }
    };

    const run = async () => {
      setStatus('loading');
      setUrl(null);
      setTextContent('');
      revoke();

      if (kind === 'unsupported') {
        if (!cancelled) setStatus('unsupported');
        return;
      }
      if (typeof size === 'number' && size > MAX_PREVIEW_BYTES) {
        if (!cancelled) setStatus('too-large');
        return;
      }

      try {
        const blob = await fetchObjectBlob(bucket, objectKey);
        if (cancelled) return;

        // Enforce the size cap by actual bytes too, in case the object size was
        // unknown (0/undefined) before fetching.
        if (blob.size > MAX_PREVIEW_BYTES) {
          setStatus('too-large');
          return;
        }

        if (kind === 'text') {
          if (blob.size > MAX_TEXT_BYTES) {
            setStatus('too-large');
            return;
          }
          // Read as text and render via React children, which escapes it. The
          // bytes are never interpreted as markup, so .html/.xml/.svg files are
          // shown as inert source rather than executed.
          const text = await blob.text();
          if (cancelled) return;
          setTextContent(text);
          setStatus('ready');
          return;
        }

        // Media / pdf: render from an object URL with safe elements only.
        if (cancelled) return;
        const objectUrl = URL.createObjectURL(blob);
        urlRef.current = objectUrl;
        setUrl(objectUrl);
        setStatus('ready');
      } catch (err) {
        if (!cancelled) {
          setStatus('error');
          message.error(errorMessage(err, t('preview.failed')));
        }
      }
    };

    void run();
    return () => {
      cancelled = true;
      revoke();
    };
  }, [visible, bucket, objectKey, kind, size, t]);

  const triggerDownload = async () => {
    try {
      await downloadObject(bucket, objectKey, name);
    } catch (err) {
      message.error(errorMessage(err, t('errors.downloadFailed')));
    }
  };

  const fallback = (description: string) => (
    <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={description}>
      <Text type="secondary">{t('preview.downloadToView')}</Text>
    </Empty>
  );

  const renderBody = () => {
    if (status === 'loading') {
      return <div className="preview-center"><Spin /></div>;
    }
    if (status === 'unsupported') return fallback(t('preview.unsupported'));
    if (status === 'too-large') return fallback(t('preview.tooLarge'));
    if (status === 'error') return fallback(t('preview.failed'));

    switch (kind) {
      case 'image':
      case 'svg':
        // An <img> never runs scripts embedded inside an SVG it loads.
        return <img className="preview-media" src={url ?? ''} alt={name} />;
      case 'video':
        return <video className="preview-media" src={url ?? ''} controls />;
      case 'audio':
        return <audio className="preview-audio" src={url ?? ''} controls />;
      case 'pdf':
        // Fully sandboxed iframe (no allow-scripts): the document cannot run
        // scripts in our origin. If a browser refuses to render it sandboxed,
        // the user can still download.
        return (
          <iframe
            className="preview-pdf"
            title={name}
            src={url ?? ''}
            sandbox=""
          />
        );
      case 'text':
        // Plain text rendered as React children (escaped) - never as markup.
        return <pre className="preview-text">{textContent}</pre>;
      default:
        return fallback(t('preview.unsupported'));
    }
  };

  return (
    <Modal
      title={`${t('preview.title')} - ${name}`}
      open={visible}
      onCancel={onClose}
      width={820}
      style={{ maxWidth: 'calc(100vw - 32px)' }}
      footer={[
        <Button key="close" onClick={onClose}>{t('common.cancel')}</Button>,
        <Button
          key="download"
          type="primary"
          icon={<DownloadOutlined />}
          onClick={() => { void triggerDownload(); }}
        >
          {t('common.download')}
        </Button>,
      ]}
    >
      <div className="preview-body">{renderBody()}</div>
    </Modal>
  );
}
