// Pure helpers for the resizable sidebar width.
//
// Kept free of React so the clamp/persistence logic can be unit-tested
// directly. All functions are defensive: corrupt or missing localStorage
// values fall back to the default width rather than throwing.

/** localStorage key holding the user-chosen sidebar width (pixels). */
export const SIDER_WIDTH_STORAGE_KEY = 'pier.sider.width';

/** Default sidebar width when nothing is stored. */
export const DEFAULT_SIDER_WIDTH = 240;

/** Resize bounds: narrow enough to save space, wide enough for long names. */
export const MIN_SIDER_WIDTH = 180;
export const MAX_SIDER_WIDTH = 480;

/** Clamp a candidate width into the allowed range. */
export function clampSiderWidth(width: number): number {
  if (!Number.isFinite(width)) return DEFAULT_SIDER_WIDTH;
  return Math.min(MAX_SIDER_WIDTH, Math.max(MIN_SIDER_WIDTH, Math.round(width)));
}

/** Read the persisted width, falling back to the default. */
export function loadSiderWidth(): number {
  if (typeof localStorage === 'undefined') return DEFAULT_SIDER_WIDTH;
  const raw = localStorage.getItem(SIDER_WIDTH_STORAGE_KEY);
  if (!raw) return DEFAULT_SIDER_WIDTH;
  const parsed = Number(raw);
  return Number.isFinite(parsed) ? clampSiderWidth(parsed) : DEFAULT_SIDER_WIDTH;
}

/** Persist the chosen width (already clamped by the caller). */
export function saveSiderWidth(width: number): void {
  if (typeof localStorage === 'undefined') return;
  localStorage.setItem(SIDER_WIDTH_STORAGE_KEY, String(clampSiderWidth(width)));
}
