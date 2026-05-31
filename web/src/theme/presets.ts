// Code-defined theme presets.
//
// To add a theme: append an entry here. It appears in the header theme menu
// automatically. Names are shown verbatim (not translated).
//
// Themes are intentionally authored in code rather than crafted by end users
// in the UI: a user-editable theme would mean accepting arbitrary
// user-supplied values (colors, and potentially more over time) into the
// rendered surface, which is an avoidable injection / abuse surface. Keeping
// presets in code means every theme is reviewed and trusted.

import type { PresetTheme } from './types';

export const PRESET_THEMES: PresetTheme[] = [
  { id: 'ocean', name: 'Ocean', base: 'light', primaryColor: '#0091FF', borderRadius: 10 },
  { id: 'graphite', name: 'Graphite', base: 'dark', primaryColor: '#10B981', borderRadius: 8 },
  { id: 'grape', name: 'Grape', base: 'dark', primaryColor: '#8B5CF6', borderRadius: 12 },
];
