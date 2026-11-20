import { defineI18n } from 'fumadocs-core/i18n';

export const i18n = defineI18n({
  defaultLanguage: 'zh',
  languages: ['zh', 'en'],
  parser: 'dir',
  hideLocale: 'default-locale',
  fallbackLanguage: null,
});
