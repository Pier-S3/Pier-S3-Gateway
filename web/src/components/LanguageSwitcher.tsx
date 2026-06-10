import { Select, Space } from 'antd';
import { GlobalOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { LANGUAGE_OPTIONS, SUPPORTED_LANGUAGES } from '../i18n';

export default function LanguageSwitcher() {
  const { t, i18n } = useTranslation();
  const resolved = i18n.resolvedLanguage ?? '';
  const current = SUPPORTED_LANGUAGES.includes(resolved) ? resolved : 'en';

  return (
    <Select
      size="small"
      value={current}
      onChange={(value) => { void i18n.changeLanguage(value); }}
      options={LANGUAGE_OPTIONS}
      // Globe sits to the LEFT of the language name so the label reads as one
      // self-contained unit ("globe + English"), not a bare word floating
      // between the theme icon and a trailing globe.
      labelRender={({ label }) => (
        <Space size={6}>
          <GlobalOutlined />
          <span className="lang-label">{label}</span>
        </Space>
      )}
      popupMatchSelectWidth={false}
      variant="borderless"
      className="lang-select"
      aria-label={t('common.language')}
    />
  );
}
