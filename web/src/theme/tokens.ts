// Bridges the active AntD theme into the plain-CSS `--dbx-*` custom properties
// used by src/styles/index.css, so hand-written CSS adapts to dark mode and to
// the active accent color without hardcoding values.
//
// Strategy: build the same ThemeConfig we hand to ConfigProvider, run it
// through AntD's `getDesignToken` to obtain the fully-computed token set, then
// map the relevant tokens onto our CSS variables. The provider writes these to
// document.documentElement plus a `data-theme` attribute, and index.css carries
// dark fallbacks under `[data-theme="dark"]`.

import { theme as antdTheme } from 'antd';
import type { ThemeConfig } from 'antd';
import type { PresetTheme, EffectiveScheme, SurfacePalette } from './types';
import { DBX_PRIMARY, DBX_RADIUS } from './storage';

export const FONT_FAMILY =
  '"Inter", -apple-system, "Segoe UI", Roboto, sans-serif';

/** Monospace stack for technical values (sizes, dates, object keys). */
export const MONO_FAMILY =
  '"JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, Consolas, monospace';

/**
 * Scheme-specific surface palette. We override a handful of AntD's structural
 * tokens so the whole component set (cards, tables, dropdowns, modals) inherits
 * a cohesive slate "console" palette rather than AntD's flatter defaults. Dark
 * mode in particular moves from near-black greys to layered slate (#0B1120 →
 * #141B2E → #1B2540) for a premium, depth-aware look.
 */
const SURFACES = {
  light: {
    bgLayout: '#F6F8FB',
    bgContainer: '#FFFFFF',
    bgElevated: '#FFFFFF',
    border: '#E2E8F0',
    borderSecondary: '#EAEEF4',
    surface2: '#F1F5F9',
    borderStrong: '#CBD5E1',
    shadowSm: '0 1px 2px rgba(15, 23, 42, 0.06)',
    shadow: '0 6px 18px rgba(15, 23, 42, 0.08)',
    shadowLg: '0 18px 44px rgba(15, 23, 42, 0.14)',
  },
  dark: {
    bgLayout: '#0B1120',
    bgContainer: '#141B2E',
    bgElevated: '#1B2540',
    border: '#2A344E',
    borderSecondary: '#202A42',
    surface2: '#1B2540',
    borderStrong: '#334155',
    shadowSm: '0 1px 2px rgba(0, 0, 0, 0.4)',
    shadow: '0 8px 24px rgba(0, 0, 0, 0.45)',
    shadowLg: '0 24px 60px rgba(0, 0, 0, 0.6)',
  },
} as const;

/**
 * Resolve the effective surface palette for a scheme, layering a preset's
 * optional overrides over the scheme defaults. Authored entirely in code, so no
 * untrusted values ever reach the rendered surface.
 */
function resolveSurfaces(
  scheme: EffectiveScheme,
  preset: PresetTheme | null,
): SurfacePalette {
  return { ...SURFACES[scheme], ...(preset?.surfaces ?? {}) };
}

/**
 * Build the AntD ThemeConfig for an effective scheme and optional custom theme.
 */
export function buildThemeConfig(
  scheme: EffectiveScheme,
  preset: PresetTheme | null,
): ThemeConfig {
  const algorithm =
    scheme === 'dark' ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm;

  const primary = preset?.primaryColor ?? DBX_PRIMARY;
  const radius = preset?.borderRadius ?? DBX_RADIUS;
  const s = resolveSurfaces(scheme, preset);

  const token: ThemeConfig['token'] = {
    colorPrimary: primary,
    borderRadius: radius,
    fontFamily: FONT_FAMILY,
    fontSize: 14,
    colorBgLayout: s.bgLayout,
    colorBgContainer: s.bgContainer,
    colorBgElevated: s.bgElevated,
    colorBorder: s.border,
    colorBorderSecondary: s.borderSecondary,
    // Slightly more generous control height for a modern, tappable feel.
    controlHeight: 36,
    // Unify popup elevation (dropdowns, selects, popovers) with the app's own
    // shadow language instead of AntD's flatter default - the difference is most
    // visible in dark mode, where the default shadow barely reads.
    boxShadowSecondary: s.shadow,
  };

  // A preset may pin its own text colors (e.g. Claude's warm ink) so the whole
  // component set tracks the bespoke palette rather than the algorithm default.
  if (s.text) {
    token.colorText = s.text;
  }
  if (s.textMuted) {
    token.colorTextSecondary = s.textMuted;
    token.colorTextTertiary = s.textMuted;
  }

  // Per-component refinements that don't have a single global token.
  const components: ThemeConfig['components'] = {
    Layout: {
      headerBg: s.bgContainer,
      siderBg: s.bgContainer,
      bodyBg: s.bgLayout,
    },
    Menu: {
      itemBorderRadius: radius,
      itemHeight: 40,
      itemMarginInline: 8,
      activeBarWidth: 0,
    },
    Card: { borderRadiusLG: Math.max(radius, 12) },
    Table: { headerBg: s.surface2, rowHoverBg: s.surface2 },
    Button: { fontWeight: 500, primaryShadow: 'none' },
  };

  return { algorithm, token, components };
}

/** The CSS custom properties we expose to hand-written CSS. */
export interface DbxCssVars {
  '--dbx-blue': string;
  '--dbx-blue-hover': string;
  '--dbx-blue-soft': string;
  '--dbx-border': string;
  '--dbx-border-strong': string;
  '--dbx-bg': string;
  '--dbx-text': string;
  '--dbx-muted': string;
  /** Surface for cards / sider / header (was hardcoded #fff). */
  '--dbx-surface': string;
  /** A secondary, slightly raised/recessed surface (table head, code blocks). */
  '--dbx-surface-2': string;
  /** Surface for popovers / modals (elevated). */
  '--dbx-elevated': string;
  /** Subtle highlight for hovered/selected table rows (was #f3f7ff). */
  '--dbx-row-hover': string;
  /** Dropzone hover background (was #f7faff). */
  '--dbx-dropzone-hover': string;
  /** Layered elevation shadows. */
  '--dbx-shadow-sm': string;
  '--dbx-shadow': string;
  '--dbx-shadow-lg': string;
  /** Brand accent gradient (logo tile, login, primary surfaces). */
  '--dbx-gradient': string;
  /** Monospace font stack for technical values. */
  '--dbx-mono': string;
}

/**
 * Compute the `--dbx-*` CSS variables from the active theme config. Most values
 * come straight from AntD's computed token set, so they track the algorithm
 * (light/dark) and the chosen accent automatically; the rest come from the
 * scheme-specific SURFACES palette above.
 */
export function computeCssVars(
  config: ThemeConfig,
  scheme: EffectiveScheme,
  preset: PresetTheme | null = null,
): DbxCssVars {
  const t = antdTheme.getDesignToken(config);
  const s = resolveSurfaces(scheme, preset);
  return {
    '--dbx-blue': t.colorPrimary,
    '--dbx-blue-hover': t.colorPrimaryHover,
    '--dbx-blue-soft': t.colorPrimaryBg,
    '--dbx-border': t.colorBorderSecondary,
    '--dbx-border-strong': s.borderStrong,
    '--dbx-bg': t.colorBgLayout,
    '--dbx-text': s.text ?? t.colorText,
    '--dbx-muted': s.textMuted ?? t.colorTextSecondary,
    '--dbx-surface': t.colorBgContainer,
    '--dbx-surface-2': s.surface2,
    '--dbx-elevated': t.colorBgElevated,
    // Derived accent-tinted hover: AntD's "bg" shade of the primary palette.
    '--dbx-row-hover': s.surface2,
    '--dbx-dropzone-hover': t.colorPrimaryBg,
    '--dbx-shadow-sm': s.shadowSm,
    '--dbx-shadow': s.shadow,
    '--dbx-shadow-lg': s.shadowLg,
    // Glossy brand gradient that adapts to the active accent.
    '--dbx-gradient': `linear-gradient(135deg, ${t.colorPrimaryHover} 0%, ${t.colorPrimary} 55%, ${t.colorPrimaryActive} 100%)`,
    '--dbx-mono': MONO_FAMILY,
  };
}
