import { describe, it, expect } from 'vitest';
import { previewKindFor } from './previewKind';

describe('previewKindFor (by extension)', () => {
  it('classifies images', () => {
    expect(previewKindFor('a.png')).toBe('image');
    expect(previewKindFor('a.JPG')).toBe('image');
    expect(previewKindFor('dir/sub/photo.webp')).toBe('image');
  });

  it('classifies svg as svg (rendered via <img>, never inline)', () => {
    expect(previewKindFor('icon.svg')).toBe('svg');
  });

  it('classifies video and audio', () => {
    expect(previewKindFor('clip.mp4')).toBe('video');
    expect(previewKindFor('song.mp3')).toBe('audio');
  });

  it('classifies pdf', () => {
    expect(previewKindFor('doc.pdf')).toBe('pdf');
  });

  it('classifies text/code/data as text', () => {
    expect(previewKindFor('a.txt')).toBe('text');
    expect(previewKindFor('a.md')).toBe('text');
    expect(previewKindFor('a.json')).toBe('text');
    expect(previewKindFor('main.go')).toBe('text');
  });

  it('treats HTML/XML files as TEXT (shown as source, never executed)', () => {
    expect(previewKindFor('page.html')).toBe('text');
    expect(previewKindFor('data.xml')).toBe('text');
  });

  it('returns unsupported for unknown or extensionless names', () => {
    expect(previewKindFor('archive.bin')).toBe('unsupported');
    expect(previewKindFor('README')).toBe('unsupported');
    expect(previewKindFor('a.b.tar.gz')).toBe('unsupported');
  });
});

describe('previewKindFor (content-type fallback)', () => {
  it('uses content-type only when the extension is unknown', () => {
    expect(previewKindFor('blob', 'image/png')).toBe('image');
    expect(previewKindFor('blob', 'video/mp4')).toBe('video');
    expect(previewKindFor('blob', 'audio/mpeg')).toBe('audio');
    expect(previewKindFor('blob', 'application/pdf')).toBe('pdf');
    expect(previewKindFor('blob', 'text/plain; charset=utf-8')).toBe('text');
    expect(previewKindFor('blob', 'application/json')).toBe('text');
  });

  it('maps SVG content-type to the safe <img> renderer', () => {
    expect(previewKindFor('blob', 'image/svg+xml')).toBe('svg');
  });

  it('never picks an active renderer from content-type: html stays text', () => {
    expect(previewKindFor('blob', 'text/html')).toBe('text');
  });

  it('extension wins over a conflicting content-type', () => {
    expect(previewKindFor('a.png', 'text/html')).toBe('image');
  });

  it('returns unsupported for opaque/unknown content-types', () => {
    expect(previewKindFor('blob', 'application/octet-stream')).toBe('unsupported');
    expect(previewKindFor('blob')).toBe('unsupported');
  });
});
