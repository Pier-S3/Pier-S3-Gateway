import '@testing-library/jest-dom/vitest';
import { afterEach, vi } from 'vitest';
import { cleanup } from '@testing-library/react';
// Initialize the global i18next instance so components using useTranslation
// render real (English/default) strings in tests instead of raw keys.
import '../i18n';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

// jsdom in this setup exposes a `localStorage` object without working methods,
// so provide a small in-memory Storage shim. The app (i18n detector, theme
// persistence) and tests both rely on a functional Web Storage API.
if (typeof window.localStorage?.setItem !== 'function') {
  const store = new Map<string, string>();
  const memoryStorage: Storage = {
    get length() {
      return store.size;
    },
    clear: () => store.clear(),
    getItem: (key: string) => (store.has(key) ? store.get(key)! : null),
    key: (index: number) => Array.from(store.keys())[index] ?? null,
    removeItem: (key: string) => {
      store.delete(key);
    },
    setItem: (key: string, value: string) => {
      store.set(key, String(value));
    },
  };
  Object.defineProperty(window, 'localStorage', {
    value: memoryStorage,
    configurable: true,
  });
}

// This jsdom build ships a Blob without the async read helpers (text/arrayBuffer).
// FilePreview reads text objects via blob.text(); provide a minimal polyfill so
// preview rendering works under test, matching real browsers.
if (typeof Blob !== 'undefined' && typeof Blob.prototype.text !== 'function') {
  Blob.prototype.text = function text(this: Blob): Promise<string> {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(String(reader.result ?? ''));
      reader.onerror = () => reject(reader.error);
      reader.readAsText(this);
    });
  };
}

// antd's responsive components rely on matchMedia, which jsdom does not provide.
if (!window.matchMedia) {
  window.matchMedia = (query: string): MediaQueryList => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  });
}
