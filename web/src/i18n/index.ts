import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';
import en from './en';
import ru from './ru';
import es from './es';
import it from './it';
import fr from './fr';

/**
 * Single registry of available UI languages.
 *
 * To add a language: create `./<code>.ts` mirroring `en.ts`, import it here,
 * and add one `{ code, label, bundle }` entry below. The supported-language
 * list, the i18next resources and the language-switcher options are all
 * derived from this array - nothing else needs to change.
 */
export interface LanguageDef {
  /** i18next language code, e.g. "en", "es", "fr". */
  code: string;
  /** Endonym shown in the switcher (sourced from the bundle's common.nativeName
   *  so each language carries its own name - no foreign literals live here). */
  label: string;
  /** Translation bundle for this language. */
  bundle: Record<string, unknown>;
}

/** Each language's own name lives inside its bundle (common.nativeName). */
function endonym(bundle: Record<string, unknown>): string {
  const common = bundle.common as { nativeName?: string } | undefined;
  return common?.nativeName ?? '';
}

// Registry: code -> bundle. To add a language, create ./<code>.ts and add a row.
const REGISTRY: { code: string; bundle: Record<string, unknown> }[] = [
  { code: 'en', bundle: en },
  { code: 'ru', bundle: ru },
  { code: 'es', bundle: es },
  { code: 'it', bundle: it },
  { code: 'fr', bundle: fr },
];

export const LANGUAGES: LanguageDef[] = REGISTRY.map(({ code, bundle }) => ({
  code,
  label: endonym(bundle),
  bundle,
}));

export const SUPPORTED_LANGUAGES: string[] = LANGUAGES.map((l) => l.code);

/** Options ready for an AntD Select / list control. */
export const LANGUAGE_OPTIONS = LANGUAGES.map(({ code, label }) => ({
  value: code,
  label,
}));

const resources = Object.fromEntries(
  LANGUAGES.map((l) => [l.code, { translation: l.bundle }]),
);

void i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    fallbackLng: 'en',
    supportedLngs: SUPPORTED_LANGUAGES,
    nonExplicitSupportedLngs: true,
    interpolation: { escapeValue: false },
    detection: {
      order: ['localStorage', 'navigator'],
      lookupLocalStorage: 'lang',
      caches: ['localStorage'],
    },
  });

export default i18n;
