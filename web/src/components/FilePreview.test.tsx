import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import FilePreview from './FilePreview';

const MALICIOUS = '<img src=x onerror="alert(1)"><script>steal()</script>hello';

vi.mock('../api/client', () => ({
  fetchObjectBlob: vi.fn(async () => new Blob([MALICIOUS], { type: 'text/plain' })),
  downloadObject: vi.fn(),
  errorMessage: (_e: unknown, fallback: string) => fallback,
}));

describe('FilePreview security', () => {
  it('renders a text/HTML file as escaped source, never as live DOM', async () => {
    render(
      <FilePreview
        bucket="b"
        objectKey="note.html"
        size={MALICIOUS.length}
        visible
        onClose={() => {}}
      />,
    );

    // The raw markup appears verbatim as TEXT (React-escaped) inside <pre>.
    const pre = await screen.findByText(/onerror=/);
    expect(pre.tagName).toBe('PRE');
    expect(pre.textContent).toBe(MALICIOUS);

    // Crucially, no live <script> or <img> element was created from the content.
    expect(pre.querySelector('script')).toBeNull();
    expect(pre.querySelector('img')).toBeNull();
  });
});
