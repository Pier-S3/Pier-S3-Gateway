// Header control for switching themes. A dropdown listing System / Light / Dark
// plus the code-defined presets. Selection only - themes are authored in code
// (theme/presets.ts), there is no in-app theme editor. Keyboard-accessible;
// the trigger carries an aria-label sourced from i18n.

import { useMemo } from "react";
import { Button, Dropdown, Space } from 'antd';
import type { MenuProps } from 'antd';
import {
  BgColorsOutlined,
  DesktopOutlined,
  SunOutlined,
  MoonOutlined,
  CheckOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useTheme } from './ThemeProvider';

export default function ThemeSwitcher() {
  const { t } = useTranslation();
  const { selection, presets, setMode, selectPreset } = useTheme();

  const activeKey = useMemo(
    () => (selection.kind === 'mode' ? `mode:${selection.mode}` : `preset:${selection.id}`),
    [selection],
  );

  const check = (key: string) =>
    activeKey === key ? (
      <CheckOutlined />
    ) : (
      <span style={{ width: 14, display: 'inline-block' }} />
    );

  const items: MenuProps['items'] = [
    { key: 'mode:system', icon: <DesktopOutlined />, label: <Space>{check('mode:system')}{t('theme.system')}</Space> },
    { key: 'mode:light', icon: <SunOutlined />, label: <Space>{check('mode:light')}{t('theme.light')}</Space> },
    { key: 'mode:dark', icon: <MoonOutlined />, label: <Space>{check('mode:dark')}{t('theme.dark')}</Space> },
  ];

  if (presets.length > 0) {
    items.push({ type: 'divider' });
    presets.forEach((preset) => {
      items.push({
        key: `preset:${preset.id}`,
        icon: <BgColorsOutlined style={{ color: preset.primaryColor }} />,
        // preset.name is a code-defined brand name - shown verbatim.
        label: <Space>{check(`preset:${preset.id}`)}<span>{preset.name}</span></Space>,
      });
    });
  }

  const onClick: MenuProps['onClick'] = ({ key }) => {
    if (key.startsWith('mode:')) {
      setMode(key.slice('mode:'.length) as 'system' | 'light' | 'dark');
    } else if (key.startsWith('preset:')) {
      selectPreset(key.slice('preset:'.length));
    }
  };

  return (
    <Dropdown
      menu={{ items, onClick, selectable: true, selectedKeys: [activeKey] }}
      trigger={['click']}
      placement="bottomRight"
    >
      <Button type="text" icon={<BgColorsOutlined />} aria-label={t('theme.label')} />
    </Dropdown>
  );
}
