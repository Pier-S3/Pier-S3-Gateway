import { describe, it, expect, beforeEach } from 'vitest';
import {
  SELECTION_STORAGE_KEY,
  DEFAULT_SELECTION,
  loadSelection,
  saveSelection,
  resolveScheme,
} from './storage';
import type { PresetTheme } from './types';

describe('theme storage', () => {
  beforeEach(() => {
    localStorage.removeItem(SELECTION_STORAGE_KEY);
  });

  it('returns the system default when nothing is stored', () => {
    expect(loadSelection()).toEqual(DEFAULT_SELECTION);
  });

  it('persists and reloads a mode selection', () => {
    saveSelection({ kind: 'mode', mode: 'dark' });
    expect(localStorage.getItem(SELECTION_STORAGE_KEY)).toContain('dark');
    expect(loadSelection()).toEqual({ kind: 'mode', mode: 'dark' });
  });

  it('persists and reloads a preset selection', () => {
    saveSelection({ kind: 'preset', id: 'ocean' });
    expect(loadSelection()).toEqual({ kind: 'preset', id: 'ocean' });
  });

  it('falls back to default on corrupt selection JSON', () => {
    localStorage.setItem(SELECTION_STORAGE_KEY, '{not json');
    expect(loadSelection()).toEqual(DEFAULT_SELECTION);
  });

  it('rejects an invalid selection shape', () => {
    localStorage.setItem(SELECTION_STORAGE_KEY, JSON.stringify({ kind: 'preset' }));
    expect(loadSelection()).toEqual(DEFAULT_SELECTION);
  });
});

describe('resolveScheme', () => {
  const ocean: PresetTheme = {
    id: 'ocean',
    name: 'Ocean',
    base: 'dark',
    primaryColor: '#00aaff',
    borderRadius: 12,
  };

  it('follows the OS scheme in system mode', () => {
    expect(resolveScheme({ kind: 'mode', mode: 'system' }, 'dark', [])).toBe('dark');
    expect(resolveScheme({ kind: 'mode', mode: 'system' }, 'light', [])).toBe('light');
  });

  it('honors explicit light/dark modes regardless of OS', () => {
    expect(resolveScheme({ kind: 'mode', mode: 'light' }, 'dark', [])).toBe('light');
    expect(resolveScheme({ kind: 'mode', mode: 'dark' }, 'light', [])).toBe('dark');
  });

  it('uses the base of the referenced preset', () => {
    expect(resolveScheme({ kind: 'preset', id: 'ocean' }, 'light', [ocean])).toBe('dark');
  });

  it('falls back to the OS scheme when the preset is missing', () => {
    expect(resolveScheme({ kind: 'preset', id: 'gone' }, 'light', [ocean])).toBe('light');
  });
});
