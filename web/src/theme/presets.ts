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
  { id: 'ocean', name: 'Ocean', base: 'light', primaryColor: '#0EA5E9', borderRadius: 10 },
  { id: 'midnight', name: 'Midnight', base: 'dark', primaryColor: '#6366F1', borderRadius: 10 },
  { id: 'emerald', name: 'Emerald', base: 'dark', primaryColor: '#10B981', borderRadius: 10 },

  // GitHub - "Primer" palette. Light uses the canvas/border greys and the
  // accent blue (#0969DA); dark uses the #0D1117 slate canvas and #2F81F7 link
  // accent. 6px radius matches GitHub's tighter corner language.
  {
    id: 'github-light',
    name: 'GitHub Light',
    base: 'light',
    primaryColor: '#0969DA',
    borderRadius: 6,
    surfaces: {
      bgLayout: '#F6F8FA',
      bgContainer: '#FFFFFF',
      bgElevated: '#FFFFFF',
      border: '#D0D7DE',
      borderSecondary: '#D8DEE4',
      surface2: '#F6F8FA',
      borderStrong: '#AFB8C1',
      text: '#1F2328',
      textMuted: '#656D76',
    },
  },
  {
    id: 'github-dark',
    name: 'GitHub Dark',
    base: 'dark',
    primaryColor: '#2F81F7',
    borderRadius: 6,
    surfaces: {
      bgLayout: '#010409',
      bgContainer: '#0D1117',
      bgElevated: '#161B22',
      border: '#30363D',
      borderSecondary: '#21262D',
      surface2: '#161B22',
      borderStrong: '#3D444D',
      text: '#E6EDF3',
      textMuted: '#8B949E',
    },
  },

  // Claude / Anthropic - warm "ivory & clay" palette. Light is the signature
  // cream canvas with clay (#C96442) accent and warm ink; dark is Claude
  // Desktop's warm near-black panels (#262624) with a brighter clay (#D97757).
  {
    id: 'claude-light',
    name: 'Claude Light',
    base: 'light',
    primaryColor: '#C96442',
    borderRadius: 8,
    surfaces: {
      bgLayout: '#F0EEE6',
      bgContainer: '#FAF9F5',
      bgElevated: '#FFFFFF',
      border: '#DAD3C2',
      borderSecondary: '#E7E1D5',
      surface2: '#F0EEE6',
      borderStrong: '#C9BFA8',
      text: '#29261F',
      textMuted: '#6B6456',
    },
  },
  {
    id: 'claude-dark',
    name: 'Claude Dark',
    base: 'dark',
    primaryColor: '#D97757',
    borderRadius: 8,
    surfaces: {
      bgLayout: '#1F1E1D',
      bgContainer: '#262624',
      bgElevated: '#30302E',
      border: '#3E3E3A',
      borderSecondary: '#333330',
      surface2: '#2A2A28',
      borderStrong: '#4A4A45',
      text: '#ECEAE2',
      textMuted: '#A6A095',
    },
  },

  // Popular editor palettes. Authentic source colors (Dracula, Nord, Solarized,
  // Rosé Pine, Gruvbox), each carrying a full surface set.
  {
    id: 'dracula',
    name: 'Dracula',
    base: 'dark',
    primaryColor: '#BD93F9',
    borderRadius: 8,
    surfaces: {
      bgLayout: '#21222C',
      bgContainer: '#282A36',
      bgElevated: '#343746',
      border: '#44475A',
      borderSecondary: '#343746',
      surface2: '#343746',
      borderStrong: '#6272A4',
      text: '#F8F8F2',
      textMuted: '#9AA0B5',
    },
  },
  {
    id: 'nord',
    name: 'Nord',
    base: 'dark',
    primaryColor: '#88C0D0',
    borderRadius: 8,
    surfaces: {
      bgLayout: '#2E3440',
      bgContainer: '#3B4252',
      bgElevated: '#434C5E',
      border: '#4C566A',
      borderSecondary: '#434C5E',
      surface2: '#434C5E',
      borderStrong: '#4C566A',
      text: '#ECEFF4',
      textMuted: '#AEB6C7',
    },
  },
  {
    id: 'solarized-light',
    name: 'Solarized Light',
    base: 'light',
    primaryColor: '#268BD2',
    borderRadius: 6,
    surfaces: {
      bgLayout: '#EEE8D5',
      bgContainer: '#FDF6E3',
      bgElevated: '#FDF6E3',
      border: '#E3DCC6',
      borderSecondary: '#EEE8D5',
      surface2: '#EEE8D5',
      borderStrong: '#CFC8B0',
      text: '#657B83',
      textMuted: '#93A1A1',
    },
  },
  {
    id: 'rose-pine',
    name: 'Rosé Pine',
    base: 'dark',
    primaryColor: '#C4A7E7',
    borderRadius: 10,
    surfaces: {
      bgLayout: '#191724',
      bgContainer: '#1F1D2E',
      bgElevated: '#26233A',
      border: '#26233A',
      borderSecondary: '#211F30',
      surface2: '#26233A',
      borderStrong: '#403D52',
      text: '#E0DEF4',
      textMuted: '#908CAA',
    },
  },
  {
    id: 'gruvbox',
    name: 'Gruvbox',
    base: 'dark',
    primaryColor: '#FE8019',
    borderRadius: 6,
    surfaces: {
      bgLayout: '#1D2021',
      bgContainer: '#282828',
      bgElevated: '#3C3836',
      border: '#504945',
      borderSecondary: '#3C3836',
      surface2: '#3C3836',
      borderStrong: '#665C54',
      text: '#EBDBB2',
      textMuted: '#A89984',
    },
  },
];
