import type { DocsLayoutProps } from 'fumadocs-ui/layouts/docs';
import { uiTranslations } from 'fumadocs-ui/i18n';
import { i18n } from './i18n';

export const translations = i18n
  .translations()
  .extend(uiTranslations())
  .add({
    zh: {
      displayName: '简体中文',
      'Choose a language(language switcher)(aria-label)': '选择语言',
      'Search(search trigger)': '搜索文档',
      'Search(search dialog)': '搜索文档',
      'On this page(table of contents)': '本页目录',
    },
    en: { displayName: 'English' },
  });

export function baseOptions(locale: 'zh' | 'en'): Omit<DocsLayoutProps, 'tree'> {
  return {
    nav: {
      title: 'OpenFlare',
      url: locale === 'zh' ? '/' : '/en',
    },
    githubUrl: 'https://github.com/Rain-kl/OpenFlare',
    tabMode: 'top',
  };
}
