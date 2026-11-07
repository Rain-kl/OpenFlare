import english from '@/messages/fragments/builtin-metadata.en.json';
import chinese from '@/messages/fragments/builtin-metadata.zh-CN.json';
import russian from '@/messages/fragments/builtin-metadata.ru.json';

function localizedDefault(
  value: string,
  en: string | undefined,
  zh: string | undefined,
  ru: string | undefined,
  locale: string,
): string {
  // Only known defaults are translated. Custom and unknown text stays intact.
  if (!en || !zh || !ru || ![en, zh, ru].includes(value)) return value;
  return locale === 'zh-CN' ? zh : locale === 'ru' ? ru : en;
}

export function localizeConfigDescription(
  key: string,
  value: string,
  locale: string,
): string {
  const en = english.builtinMetadata.configDescriptions as Record<
    string,
    string
  >;
  const zh = chinese.builtinMetadata.configDescriptions as Record<
    string,
    string
  >;
  const ru = russian.builtinMetadata.configDescriptions as typeof en;
  return localizedDefault(value, en[key], zh[key], ru[key], locale);
}

export function localizeTemplateMetadata(
  key: string,
  isSystem: boolean,
  field: 'name' | 'description',
  value: string,
  locale: string,
): string {
  if (!isSystem) return value;
  const en = english.builtinMetadata.templates as Record<
    string,
    { name: string; description: string }
  >;
  const zh = chinese.builtinMetadata.templates as typeof en;
  const ru = russian.builtinMetadata.templates as typeof en;
  return localizedDefault(
    value,
    en[key]?.[field],
    zh[key]?.[field],
    ru[key]?.[field],
    locale,
  );
}
