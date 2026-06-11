import { describe, it, expect, beforeEach } from 'vitest';
import {
  SIDER_WIDTH_STORAGE_KEY,
  DEFAULT_SIDER_WIDTH,
  MIN_SIDER_WIDTH,
  MAX_SIDER_WIDTH,
  clampSiderWidth,
  loadSiderWidth,
  saveSiderWidth,
} from './siderWidth';

describe('clampSiderWidth', () => {
  it('passes through values inside the range, rounded', () => {
    expect(clampSiderWidth(240)).toBe(240);
    expect(clampSiderWidth(300.6)).toBe(301);
  });

  it('clamps values below the minimum', () => {
    expect(clampSiderWidth(10)).toBe(MIN_SIDER_WIDTH);
  });

  it('clamps values above the maximum', () => {
    expect(clampSiderWidth(9000)).toBe(MAX_SIDER_WIDTH);
  });

  it('falls back to the default for non-finite input', () => {
    expect(clampSiderWidth(NaN)).toBe(DEFAULT_SIDER_WIDTH);
    expect(clampSiderWidth(Infinity)).toBe(DEFAULT_SIDER_WIDTH);
  });
});

describe('loadSiderWidth / saveSiderWidth', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('returns the default when nothing is stored', () => {
    expect(loadSiderWidth()).toBe(DEFAULT_SIDER_WIDTH);
  });

  it('round-trips a saved width', () => {
    saveSiderWidth(320);
    expect(loadSiderWidth()).toBe(320);
  });

  it('clamps a stored out-of-range value on load', () => {
    localStorage.setItem(SIDER_WIDTH_STORAGE_KEY, '9999');
    expect(loadSiderWidth()).toBe(MAX_SIDER_WIDTH);
  });

  it('falls back to the default on corrupt storage', () => {
    localStorage.setItem(SIDER_WIDTH_STORAGE_KEY, 'not-a-number');
    expect(loadSiderWidth()).toBe(DEFAULT_SIDER_WIDTH);
  });

  it('clamps on save', () => {
    saveSiderWidth(1);
    expect(localStorage.getItem(SIDER_WIDTH_STORAGE_KEY)).toBe(String(MIN_SIDER_WIDTH));
  });
});
