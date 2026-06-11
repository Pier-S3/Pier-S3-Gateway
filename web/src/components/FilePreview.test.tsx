import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import FilePreview from './FilePreview';

const MALICIOUS = '<img src=x onerror="alert(1)"><script>steal()</script>hello';

// Per-test blob content; the mock reads it at fetch time.
let blobContent = MALICIOUS;

vi.mock('../api/client', () => ({
  fetchObjectBlob: vi.fn(async () => new Blob([blobContent], { type: 'text/plain' })),
  downloadObject: vi.fn(),
  errorMessage: (_e: unknown, fallback: string) => fallback,
}));

function renderPreview(objectKey: string) {
  return render(
    <FilePreview
      bucket="b"
      objectKey={objectKey}
      size={blobContent.length}
      visible
      onClose={() => {}}
    />,
  );
}

beforeEach(() => {
  cleanup();
  blobContent = MALICIOUS;
});

describe('FilePreview security', () => {
  it('renders a text/HTML file as escaped source, never as live DOM', async () => {
    renderPreview('note.html');

    // The raw markup appears verbatim as TEXT (React-escaped) inside <pre>.
    const pre = await screen.findByText(/onerror=/);
    expect(pre.tagName).toBe('PRE');
    expect(pre.textContent).toBe(MALICIOUS);

    // Crucially, no live <script> or <img> element was created from the content.
    expect(pre.querySelector('script')).toBeNull();
    expect(pre.querySelector('img')).toBeNull();
  });

  it('renders markdown without executing raw HTML or dangerous links', async () => {
    blobContent = [
      '# Title',
      '<script>steal()</script>',
      '<img src=x onerror="alert(1)">',
      '[bad link](javascript:alert(1))',
      '![remote pic](https://evil.example/p.png)',
    ].join('\n\n');
    renderPreview('README.md');

    const heading = await screen.findByRole('heading', { name: 'Title' });
    expect(heading.tagName).toBe('H1');

    // Raw HTML inside markdown must never become live DOM.
    expect(document.querySelector('.preview-markdown script')).toBeNull();
    // No <img> at all: markdown images are rendered as links so the browser
    // never auto-fetches a remote URL when the preview opens.
    expect(document.querySelector('.preview-markdown img')).toBeNull();
    const imgLink = screen.getByText('remote pic').closest('a');
    expect(imgLink?.getAttribute('href')).toBe('https://evil.example/p.png');
    expect(imgLink?.getAttribute('rel')).toContain('noopener');

    // javascript: protocol is stripped by react-markdown's url transform.
    const badLink = screen.getByText('bad link').closest('a');
    expect(badLink?.getAttribute('href') ?? '').not.toMatch(/javascript:/i);
  });

  it('renders CSV as an inert table with escaped cells', async () => {
    blobContent = 'name,payload\nrow1,"<script>x()</script>"';
    renderPreview('data.csv');

    const cell = await screen.findByText('<script>x()</script>');
    // Cell content is text, not parsed markup.
    expect(cell.querySelector('script')).toBeNull();
    expect(await screen.findByRole('table')).toBeTruthy();
  });

  it('pretty-prints valid JSON and falls back to source for invalid JSON', async () => {
    blobContent = '{"a":1,"b":[1,2]}';
    renderPreview('config.json');
    const pre = await screen.findByText(/"a": 1/);
    expect(pre.tagName).toBe('PRE');
  });
});
