import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { ThemeProvider, useTheme } from './ThemeProvider';
import { SELECTION_STORAGE_KEY } from './storage';
import { PRESET_THEMES } from './presets';

function Probe() {
  const { effectiveScheme, setMode, selectPreset, presets } = useTheme();
  const darkPreset = presets.find((p) => p.base === 'dark');
  return (
    <div>
      <span data-testid="scheme">{effectiveScheme}</span>
      <span data-testid="count">{presets.length}</span>
      <button onClick={() => setMode('dark')}>dark</button>
      <button onClick={() => darkPreset && selectPreset(darkPreset.id)}>preset</button>
    </div>
  );
}

describe('ThemeProvider', () => {
  beforeEach(() => {
    localStorage.removeItem(SELECTION_STORAGE_KEY);
    document.documentElement.removeAttribute('data-theme');
  });

  it('defaults to the Claude Light preset (light surface)', () => {
    render(
      <ThemeProvider>
        <Probe />
      </ThemeProvider>,
    );
    expect(screen.getByTestId('scheme').textContent).toBe('light');
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
  });

  it('exposes the code-defined presets', () => {
    render(
      <ThemeProvider>
        <Probe />
      </ThemeProvider>,
    );
    expect(screen.getByTestId('count').textContent).toBe(String(PRESET_THEMES.length));
  });

  it('persists the selected mode and applies the dark surface', () => {
    render(
      <ThemeProvider>
        <Probe />
      </ThemeProvider>,
    );
    act(() => {
      fireEvent.click(screen.getByText('dark'));
    });
    expect(screen.getByTestId('scheme').textContent).toBe('dark');
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
    expect(localStorage.getItem(SELECTION_STORAGE_KEY)).toContain('dark');
  });

  it('selecting a dark-based preset applies the dark surface and persists', () => {
    render(
      <ThemeProvider>
        <Probe />
      </ThemeProvider>,
    );
    act(() => {
      fireEvent.click(screen.getByText('preset'));
    });
    expect(screen.getByTestId('scheme').textContent).toBe('dark');
    expect(localStorage.getItem(SELECTION_STORAGE_KEY)).toContain('preset');
  });
});
