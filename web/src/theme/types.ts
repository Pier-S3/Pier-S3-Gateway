// Theme system type definitions.
//
// The active selection is either a built-in mode ('system' | 'light' | 'dark')
// or a code-defined preset referenced by its id.

/** Built-in theme modes. */
export type ThemeMode = 'system' | 'light' | 'dark';

/** Base palette a preset extends (which AntD algorithm to use). */
export type ThemeBase = 'light' | 'dark';

/**
 * Surface/elevation palette for a scheme. The default light/dark palettes live
 * in theme/tokens.ts (SURFACES); a preset may override any subset of these to
 * ship a fully bespoke look (e.g. GitHub's slate canvas or Claude's warm
 * ivory/clay) rather than only re-tinting the accent. All values are
 * code-defined and reviewed - never user-supplied.
 */
export interface SurfacePalette {
  /** App background (body / content area). */
  bgLayout: string;
  /** Primary surface: cards, sider, header. */
  bgContainer: string;
  /** Elevated surface: popovers, modals, dropdowns. */
  bgElevated: string;
  /** Default hairline border. */
  border: string;
  /** Subtler secondary border. */
  borderSecondary: string;
  /** Secondary/recessed surface: table head, code blocks, row hover. */
  surface2: string;
  /** Stronger border for emphasis / hover. */
  borderStrong: string;
  /** Layered elevation shadows. */
  shadowSm: string;
  shadow: string;
  shadowLg: string;
  /** Optional primary text color (defaults to the AntD-computed value). */
  text?: string;
  /** Optional muted/secondary text color. */
  textMuted?: string;
}

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
  /**
   * Optional surface palette override. When present, the given keys replace the
   * scheme defaults so a preset can carry a complete bespoke palette (GitHub,
   * Claude, ...). Omitted keys fall back to the default SURFACES for `base`.
   */
  surfaces?: Partial<SurfacePalette>;
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
