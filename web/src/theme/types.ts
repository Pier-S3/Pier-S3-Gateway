// Theme system type definitions.
//
// The active selection is either a built-in mode ('system' | 'light' | 'dark')
// or a code-defined preset referenced by its id.

/** Built-in theme modes. */
export type ThemeMode = 'system' | 'light' | 'dark';

/** Base palette a preset extends (which AntD algorithm to use). */
export type ThemeBase = 'light' | 'dark';

/**
 * A code-defined theme preset (see theme/presets.ts). Presets are authored in
 * code - not crafted by end users - so their values are trusted and reviewed.
 */
export interface PresetTheme {
  /** Stable id used in the persisted selection and the menu. */
  id: string;
  /** Display name - shown verbatim (brand-style, not translated). */
  name: string;
  /** Which built-in algorithm this preset builds on. */
  base: ThemeBase;
  /** Primary / accent color (hex). */
  primaryColor: string;
  /** Corner radius in px. */
  borderRadius: number;
}

/**
 * The persisted selection: a built-in mode, or a reference to a code preset.
 * Kept as a discriminated string so it round-trips cleanly through
 * localStorage and is easy to compare.
 */
export type ThemeSelection =
  | { kind: 'mode'; mode: ThemeMode }
  | { kind: 'preset'; id: string };

/** The light/dark surface actually in effect (after resolving `system`). */
export type EffectiveScheme = 'light' | 'dark';
