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
import type { PresetTheme, EffectiveScheme } from './types';
import { DBX_PRIMARY, DBX_RADIUS } from './storage';

export const FONT_FAMILY =
  '-apple-system, "Inter", "Segoe UI", Roboto, sans-serif';

/**
 * Build the AntD ThemeConfig for an effective scheme and optional custom theme.
 * Light default reproduces the original Dropbox token set exactly.
 */
export function buildThemeConfig(
  scheme: EffectiveScheme,
  preset: PresetTheme | null,
): ThemeConfig {
  const algorithm =
    scheme === 'dark' ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm;

  const primary = preset?.primaryColor ?? DBX_PRIMARY;
  const radius = preset?.borderRadius ?? DBX_RADIUS;

  const token: ThemeConfig['token'] = {
    colorPrimary: primary,
    borderRadius: radius,
    fontFamily: FONT_FAMILY,
  };

  // Preserve the original light theme's bespoke borders/layout background.
  // Dark mode relies on the dark algorithm's own derived values.
  if (scheme === 'light') {
    token.colorBorder = '#e7e7e7';
    token.colorBorderSecondary = '#e7e7e7';
    token.colorBgLayout = '#fafafa';
  }

  return { algorithm, token };
}

/** The CSS custom properties we expose to hand-written CSS. */
export interface DbxCssVars {
  '--dbx-blue': string;
  '--dbx-border': string;
  '--dbx-bg': string;
  '--dbx-text': string;
  '--dbx-muted': string;
  /** Surface for cards / sider / header (was hardcoded #fff). */
  '--dbx-surface': string;
  /** Subtle highlight for hovered/selected table rows (was #f3f7ff). */
  '--dbx-row-hover': string;
  /** Dropzone hover background (was #f7faff). */
  '--dbx-dropzone-hover': string;
}

/**
 * Compute the `--dbx-*` CSS variables from the active theme config. Most values
 * come straight from AntD's computed token set, so they track the algorithm
 * (light/dark) and the chosen accent automatically.
 */
export function computeCssVars(config: ThemeConfig): DbxCssVars {
  const t = antdTheme.getDesignToken(config);
  return {
    '--dbx-blue': t.colorPrimary,
    '--dbx-border': t.colorBorderSecondary,
    '--dbx-bg': t.colorBgLayout,
    '--dbx-text': t.colorText,
    '--dbx-muted': t.colorTextSecondary,
    '--dbx-surface': t.colorBgContainer,
    // Derived accent-tinted hover: AntD's "bg" shade of the primary palette.
    '--dbx-row-hover': t.controlItemBgHover,
    '--dbx-dropzone-hover': t.colorPrimaryBg,
  };
}
