import { uiTranslations } from 'fumadocs-ui/i18n';
import { i18n } from './i18n';
import { source } from './source';

export const translations = i18n
  .translations()
  .extend(uiTranslations())
  .add({
    zh: { displayName: '简体中文' },
    en: { displayName: 'English' },
  });

export function baseOptions(locale: 'zh' | 'en', slug: string[]) {
  const isChinese = locale === 'zh';
  const otherPage = source.getPage(slug, isChinese ? 'en' : 'zh');
  const languageUrl = otherPage?.url ?? (isChinese ? '/en' : '/');

  return {
    nav: {
      title: 'OpenFlare',
      url: isChinese ? '/' : '/en',
    },
    githubUrl: 'https://github.com/Rain-kl/OpenFlare',
    links: [
      {
        text: isChinese ? '指南' : 'Guide',
        url: isChinese ? '/guide' : '/en/guide',
        active: 'nested-url' as const,
      },
      {
        text: isChinese ? '部署' : 'Deployment',
        url: isChinese ? '/deployment' : '/en/deployment',
        active: 'nested-url' as const,
      },
      {
        text: isChinese ? '参考' : 'Reference',
        url: isChinese ? '/reference' : '/en/reference',
        active: 'nested-url' as const,
      },
      {
        text: isChinese ? '设计' : 'Design',
        url: isChinese ? '/design' : '/en/design',
        active: 'nested-url' as const,
      },
      {
        text: isChinese ? '更新日志' : 'Changelog',
        url: isChinese ? '/changelog' : '/en/changelog',
        active: 'nested-url' as const,
      },
      {
        text: isChinese ? 'English' : '简体中文',
        url: languageUrl,
      },
    ],
  };
}
