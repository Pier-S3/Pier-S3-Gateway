// ThemeProvider owns the active theme selection, resolves the effective AntD
// ThemeConfig, persists the selection to localStorage, mirrors the resolved
// palette onto `--dbx-*` CSS variables plus a `data-theme` attribute, and
// renders the app's single ConfigProvider.
//
// Themes themselves are code-defined presets (theme/presets.ts) plus the
// built-in light/dark/system modes - there is no user-facing theme editor, so
// no untrusted theme data is ever applied. It reacts live to OS
// `prefers-color-scheme` changes whenever the selection is the `system` mode.

import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react';
import { ConfigProvider } from 'antd';
import type {
  PresetTheme,
  ThemeMode,
  ThemeSelection,
  EffectiveScheme,
} from './types';
import {
  getSystemScheme,
  loadSelection,
  resolveScheme,
  saveSelection,
} from './storage';
import { PRESET_THEMES } from './presets';
import { buildThemeConfig, computeCssVars } from './tokens';

interface ThemeContextValue {
  /** Persisted active selection (built-in mode or preset reference). */
  selection: ThemeSelection;
  /** Concrete light/dark surface currently applied. */
  effectiveScheme: EffectiveScheme;
  /** All code-defined presets. */
  presets: PresetTheme[];
  /** The active preset, if a preset is selected. */
  activePreset: PresetTheme | null;
  /** Select a built-in mode. */
  setMode: (mode: ThemeMode) => void;
  /** Select a preset by id. */
  selectPreset: (id: string) => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

/** Apply the `--dbx-*` variables and data-theme attribute to <html>. */
function applyCssVars(
  config: Parameters<typeof computeCssVars>[0],
  scheme: EffectiveScheme,
): void {
  if (typeof document === 'undefined') return;
  const root = document.documentElement;
  const vars = computeCssVars(config);
  (Object.keys(vars) as (keyof typeof vars)[]).forEach((key) => {
    root.style.setProperty(key, vars[key]);
  });
  root.setAttribute('data-theme', scheme);
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [selection, setSelection] = useState<ThemeSelection>(() => loadSelection());
  const [systemScheme, setSystemScheme] = useState<EffectiveScheme>(() => getSystemScheme());
  const presets = PRESET_THEMES;

  // Live-react to OS scheme changes (only meaningful while in `system` mode,
  // but cheap to keep current regardless).
  useEffect(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
      return;
    }
    const mql = window.matchMedia('(prefers-color-scheme: dark)');
    const onChange = (e: MediaQueryListEvent) => {
      setSystemScheme(e.matches ? 'dark' : 'light');
    };
    mql.addEventListener('change', onChange);
    return () => mql.removeEventListener('change', onChange);
  }, []);

  const activePreset = useMemo<PresetTheme | null>(() => {
    if (selection.kind !== 'preset') return null;
    return presets.find((p) => p.id === selection.id) ?? null;
  }, [selection, presets]);

  const effectiveScheme = useMemo<EffectiveScheme>(
    () => resolveScheme(selection, systemScheme, presets),
    [selection, systemScheme, presets],
  );

  const themeConfig = useMemo(
    () => buildThemeConfig(effectiveScheme, activePreset),
    [effectiveScheme, activePreset],
  );

  // Mirror the resolved palette into CSS variables + data-theme attribute.
  useEffect(() => {
    applyCssVars(themeConfig, effectiveScheme);
  }, [themeConfig, effectiveScheme]);

  const persistSelection = useCallback((next: ThemeSelection) => {
    setSelection(next);
    saveSelection(next);
  }, []);

  const setMode = useCallback(
    (mode: ThemeMode) => persistSelection({ kind: 'mode', mode }),
    [persistSelection],
  );

  const selectPreset = useCallback(
    (id: string) => persistSelection({ kind: 'preset', id }),
    [persistSelection],
  );

  const value = useMemo<ThemeContextValue>(
    () => ({
      selection,
      effectiveScheme,
      presets,
      activePreset,
      setMode,
      selectPreset,
    }),
    [selection, effectiveScheme, presets, activePreset, setMode, selectPreset],
  );

  return (
    <ThemeContext.Provider value={value}>
      <ConfigProvider theme={themeConfig}>{children}</ConfigProvider>
    </ThemeContext.Provider>
  );
}

/** Access the theme context. Throws if used outside a ThemeProvider. */
export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) {
    throw new Error('useTheme must be used within a ThemeProvider');
  }
  return ctx;
}
