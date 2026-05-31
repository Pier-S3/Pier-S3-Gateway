import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import ObjectBrowser from './ObjectBrowser';
import { ObjectInfo } from '../store/browser';

// Mock the api client so a download is observable and the metadata fetch the
// Info modal performs does not hit the network.
const downloadObjectMock = vi.fn(
  (_bucket: string, _key: string, _name: string): Promise<void> => Promise.resolve(),
);
vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof import('../api/client')>('../api/client');
  return {
    ...actual,
    downloadObject: (bucket: string, key: string, name: string) =>
      downloadObjectMock(bucket, key, name),
    apiClient: { get: vi.fn(() => new Promise(() => {})), delete: vi.fn() },
  };
});

const objects: ObjectInfo[] = [
  { key: 'report.pdf', size_bytes: 2048, last_modified: '', etag: 'e', is_prefix: false },
];

function renderBrowser(canWrite = true) {
  const onNavigatePrefix = vi.fn();
  render(
    <ObjectBrowser
      bucket="my-bucket"
      prefix=""
      objects={objects}
      prefixes={['photos/']}
      loading={false}
      canWrite={canWrite}
      truncated={false}
      onNavigatePrefix={onNavigatePrefix}
      onLoadMore={vi.fn()}
      onRefresh={vi.fn()}
    />,
  );
  return { onNavigatePrefix };
}

function fileRow() {
  return screen.getByText('report.pdf').closest('tr') as HTMLElement;
}

describe('ObjectBrowser misclick safety', () => {
  beforeEach(() => {
    downloadObjectMock.mockClear();
  });

  it('does NOT download when a file row is clicked', () => {
    renderBrowser();
    fireEvent.click(fileRow());
    expect(downloadObjectMock).not.toHaveBeenCalled();
  });

  it('opens the (non-destructive) preview panel when a file row is clicked', () => {
    renderBrowser();
    fireEvent.click(fileRow());
    // A file-row click opens the read-only FilePreview modal, whose title is
    // "Preview - <name>". It must never start a download.
    expect(screen.getByText('Preview - report.pdf')).toBeInTheDocument();
    expect(downloadObjectMock).not.toHaveBeenCalled();
  });

  it('opens the preview panel via keyboard (Enter) without downloading', () => {
    renderBrowser();
    fireEvent.keyDown(fileRow(), { key: 'Enter' });
    expect(screen.getByText('Preview - report.pdf')).toBeInTheDocument();
    expect(downloadObjectMock).not.toHaveBeenCalled();
  });

  it('downloads only via the explicit hover Download button', () => {
    renderBrowser();
    const btn = screen.getByLabelText('Download report.pdf');
    fireEvent.click(btn);
    expect(downloadObjectMock).toHaveBeenCalledTimes(1);
    expect(downloadObjectMock).toHaveBeenCalledWith('my-bucket', 'report.pdf', 'report.pdf');
  });

  it('navigates into a folder when a prefix row is clicked', () => {
    const { onNavigatePrefix } = renderBrowser();
    const folderRow = screen.getByText('photos').closest('tr') as HTMLElement;
    fireEvent.click(folderRow);
    expect(onNavigatePrefix).toHaveBeenCalledWith('photos/');
    expect(downloadObjectMock).not.toHaveBeenCalled();
  });

  it('keeps Download available in the per-row ... menu', async () => {
    renderBrowser();
    const moreBtn = screen.getByLabelText('More actions for report.pdf');
    fireEvent.click(moreBtn);
    const menuItem = await screen.findByText('Download');
    fireEvent.click(menuItem);
    expect(downloadObjectMock).toHaveBeenCalledTimes(1);
  });

  it('confirms before deleting (destructive action keeps its dialog)', async () => {
    renderBrowser(true);
    const moreBtn = screen.getByLabelText('More actions for report.pdf');
    fireEvent.click(moreBtn);
    const deleteItem = await screen.findByText('Delete');
    fireEvent.click(deleteItem);
    // Modal.confirm renders a confirmation dialog with our localized title and
    // body before any deletion happens.
    expect(await screen.findByText('Delete file "report.pdf"?')).toBeInTheDocument();
    expect(screen.getAllByText('Delete confirmation').length).toBeGreaterThan(0);
  });
});
