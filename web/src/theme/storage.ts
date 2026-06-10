// Pure helpers for persisting and resolving theme state.
//
// Kept free of React so the resolution logic can be unit-tested directly and
// reused by the provider. All functions are defensive: corrupt or missing
// localStorage values fall back to safe defaults rather than throwing.

import type { PresetTheme, ThemeSelection, EffectiveScheme } from './types';

/** localStorage key holding the active selection (JSON ThemeSelection). */
export const SELECTION_STORAGE_KEY = 'pier.theme.selection';

/** Default selection when nothing is stored. */
export const DEFAULT_SELECTION: ThemeSelection = { kind: 'mode', mode: 'system' };

/** Default brand token values (modern indigo accent, softened corners). */
export const DBX_PRIMARY = '#4F46E5';
export const DBX_RADIUS = 10;

function safeParse<T>(raw: string | null): T | null {
  if (!raw) return null;
  try {
    return JSON.parse(raw) as T;
  } catch {
    return null;
  }
}

function isValidSelection(value: unknown): value is ThemeSelection {
  if (!value || typeof value !== 'object') return false;
  const v = value as Record<string, unknown>;
  if (v.kind === 'mode') {
    return v.mode === 'system' || v.mode === 'light' || v.mode === 'dark';
  }
  if (v.kind === 'preset') {
    return typeof v.id === 'string' && v.id.length > 0;
  }
  return false;
}

/** Read the persisted selection, falling back to the default. */
export function loadSelection(): ThemeSelection {
  const parsed = safeParse<unknown>(
    typeof localStorage !== 'undefined'
      ? localStorage.getItem(SELECTION_STORAGE_KEY)
      : null,
  );
  return isValidSelection(parsed) ? parsed : DEFAULT_SELECTION;
}

/** Persist the active selection. */
export function saveSelection(selection: ThemeSelection): void {
  if (typeof localStorage === 'undefined') return;
  localStorage.setItem(SELECTION_STORAGE_KEY, JSON.stringify(selection));
}

/** Read the OS color-scheme preference. */
export function getSystemScheme(): EffectiveScheme {
  if (
    typeof window !== 'undefined' &&
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-color-scheme: dark)').matches
  ) {
    return 'dark';
  }
  return 'light';
}

/**
 * Resolve a selection (plus current OS scheme and the known presets) to the
 * concrete light/dark surface that should be applied. A preset selection whose
 * preset no longer exists falls back to the system scheme.
 */
export function resolveScheme(
  selection: ThemeSelection,
  systemScheme: EffectiveScheme,
  presets: PresetTheme[],
): EffectiveScheme {
  if (selection.kind === 'mode') {
    if (selection.mode === 'system') return systemScheme;
    return selection.mode;
  }
  const preset = presets.find((p) => p.id === selection.id);
  return preset ? preset.base : systemScheme;
}
