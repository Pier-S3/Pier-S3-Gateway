// Decides how (and whether) a file may be previewed in-browser.
//
// SECURITY: the goal is to never let untrusted content execute in our origin.
// - HTML/XHTML/XML/SVG-as-markup are NEVER rendered as markup. Files that look
//   like HTML/XML are classified as 'text' and shown as escaped source.
// - SVG is classified as 'svg' and rendered via <img> only (image-loaded SVG
//   does not execute scripts).
// The content-type argument is an untrusted hint used only to pick a SAFE
// renderer; it can never select an unsafe one.

export type PreviewKind =
  | 'image'
  | 'svg'
  | 'video'
  | 'audio'
  | 'text'
  | 'pdf'
  | 'unsupported';

/** Max bytes we will read and render as text. Larger text files are not loaded. */
export const MAX_TEXT_BYTES = 512 * 1024;
/** Max object size we will auto-fetch for any preview (avoids huge in-memory loads). */
export const MAX_PREVIEW_BYTES = 25 * 1024 * 1024;

const EXT_KIND: Record<string, PreviewKind> = {
  // images
  png: 'image', jpg: 'image', jpeg: 'image', gif: 'image', webp: 'image',
  avif: 'image', bmp: 'image', ico: 'image',
  // svg -> rendered via <img> only
  svg: 'svg',
  // video
  mp4: 'video', webm: 'video', ogv: 'video', mov: 'video', m4v: 'video',
  // audio
  mp3: 'audio', wav: 'audio', oga: 'audio', ogg: 'audio', flac: 'audio',
  m4a: 'audio', aac: 'audio',
  // pdf
  pdf: 'pdf',
  // text / code / data (shown as escaped source - never parsed as markup)
  txt: 'text', md: 'text', markdown: 'text', json: 'text', yaml: 'text',
  yml: 'text', csv: 'text', tsv: 'text', log: 'text', xml: 'text', html: 'text',
  htm: 'text', css: 'text', js: 'text', mjs: 'text', cjs: 'text', ts: 'text',
  tsx: 'text', jsx: 'text', go: 'text', py: 'text', rb: 'text', rs: 'text',
  java: 'text', kt: 'text', c: 'text', h: 'text', cpp: 'text', hpp: 'text',
  sh: 'text', bash: 'text', zsh: 'text', sql: 'text', toml: 'text', ini: 'text',
  conf: 'text', cfg: 'text', env: 'text', properties: 'text', dockerfile: 'text',
  gitignore: 'text',
};

function extOf(key: string): string {
  const base = key.toLowerCase().replace(/\/+$/, '').split('/').pop() ?? '';
  const dot = base.lastIndexOf('.');
  return dot >= 0 ? base.slice(dot + 1) : base;
}

/**
 * Choose a preview renderer for an object. Extension wins; the (untrusted)
 * content-type is only a fallback and can only ever select a SAFE renderer.
 */
export function previewKindFor(key: string, contentType?: string): PreviewKind {
  const byExt = EXT_KIND[extOf(key)];
  if (byExt) return byExt;

  const ct = (contentType ?? '').toLowerCase().split(';')[0].trim();
  if (ct === 'image/svg+xml') return 'svg';
  if (ct.startsWith('image/')) return 'image';
  if (ct.startsWith('video/')) return 'video';
  if (ct.startsWith('audio/')) return 'audio';
  if (ct === 'application/pdf') return 'pdf';
  // text/* and JSON are shown as escaped source. text/html stays 'text' too -
  // it is rendered as source, never executed.
  if (ct.startsWith('text/') || ct === 'application/json') return 'text';

  return 'unsupported';
}
