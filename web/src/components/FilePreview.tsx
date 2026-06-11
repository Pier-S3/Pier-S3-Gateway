import React, { useEffect, useRef, useState } from 'react';
import { Modal, Button, Spin, Empty, Typography, message, Segmented, Table, Alert } from 'antd';
import { DownloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { fetchObjectBlob, downloadObject, errorMessage } from '../api/client';
import {
  previewKindFor,
  isTextualKind,
  MAX_TEXT_BYTES,
  MAX_PREVIEW_BYTES,
  type PreviewKind,
} from './previewKind';
import { parseCsv, delimiterFor } from './csv';

const { Text } = Typography;

// SECURITY: react-markdown is used WITHOUT rehype-raw, so raw HTML inside
// markdown is never parsed into live DOM, and its default urlTransform strips
// dangerous link protocols (javascript: etc.). On top of that we open links in
// a new tab without an opener, and we do NOT auto-load images: an <img> would
// make the browser fetch an arbitrary remote URL the moment the preview opens
// (tracking/exfiltration vector), so images render as plain links instead.
const markdownComponents = {
  a: ({ ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => (
    <a {...props} target="_blank" rel="noopener noreferrer" />
  ),
  img: ({ src, alt }: React.ImgHTMLAttributes<HTMLImageElement>) => (
    <a href={typeof src === 'string' ? src : undefined} target="_blank" rel="noopener noreferrer">
      {alt || String(src ?? '')}
    </a>
  ),
};

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
  // Rendered vs raw-source toggle for markdown/csv/json previews.
  const [viewMode, setViewMode] = useState<'rendered' | 'source'>('rendered');
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
      setViewMode('rendered');
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

        if (isTextualKind(kind)) {
          if (blob.size > MAX_TEXT_BYTES) {
            setStatus('too-large');
            return;
          }
          // Read as text; every textual renderer below emits the bytes through
          // React children (escaped) or inert components. The content is never
          // interpreted as markup in our origin, so .html/.xml/.svg files are
          // shown as source and markdown cannot smuggle live HTML.
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

  const renderMarkdown = () => (
    <div className="preview-markdown">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={markdownComponents}>
        {textContent}
      </ReactMarkdown>
    </div>
  );

  const renderCsv = () => {
    const table = parseCsv(textContent, delimiterFor(objectKey));
    const columns = table.header.map((title, i) => ({
      title,
      dataIndex: i,
      key: i,
      ellipsis: true,
    }));
    const rows = table.rows.map((cells, i) => ({ key: i, ...cells }));
    return (
      <div className="preview-table">
        {table.truncated && (
          <Alert type="info" showIcon message={t('preview.tableTruncated')} style={{ marginBottom: 12 }} />
        )}
        <Table
          columns={columns}
          dataSource={rows}
          size="small"
          pagination={{ pageSize: 50, hideOnSinglePage: true, showSizeChanger: false }}
          scroll={{ x: 'max-content' }}
        />
      </div>
    );
  };

  const renderJson = () => {
    try {
      const pretty = JSON.stringify(JSON.parse(textContent), null, 2);
      return <pre className="preview-text">{pretty}</pre>;
    } catch {
      // Not valid JSON after all - fall back to the escaped raw source.
      return <pre className="preview-text">{textContent}</pre>;
    }
  };

  const renderBody = () => {
    if (status === 'loading') {
      return <div className="preview-center"><Spin /></div>;
    }
    if (status === 'unsupported') return fallback(t('preview.unsupported'));
    if (status === 'too-large') return fallback(t('preview.tooLarge'));
    if (status === 'error') return fallback(t('preview.failed'));

    // Rendered/source toggle for the structured textual kinds.
    if (viewMode === 'source' && (kind === 'markdown' || kind === 'csv' || kind === 'json')) {
      return <pre className="preview-text">{textContent}</pre>;
    }

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
      case 'markdown':
        return renderMarkdown();
      case 'csv':
        return renderCsv();
      case 'json':
        return renderJson();
      case 'text':
        // Plain text rendered as React children (escaped) - never as markup.
        return <pre className="preview-text">{textContent}</pre>;
      default:
        return fallback(t('preview.unsupported'));
    }
  };

  // Label of the "rendered" side of the toggle, per kind.
  const renderedLabel =
    kind === 'csv' ? t('preview.modeTable')
    : kind === 'json' ? t('preview.modeFormatted')
    : t('preview.modeRendered');

  const showModeToggle = status === 'ready' && (kind === 'markdown' || kind === 'csv' || kind === 'json');

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
      {showModeToggle && (
        <div className="preview-toolbar">
          <Segmented
            size="small"
            value={viewMode}
            onChange={(v) => setViewMode(v as 'rendered' | 'source')}
            options={[
              { label: renderedLabel, value: 'rendered' },
              { label: t('preview.modeSource'), value: 'source' },
            ]}
          />
        </div>
      )}
      <div className="preview-body">{renderBody()}</div>
    </Modal>
  );
}
