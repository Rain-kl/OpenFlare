import { I18nProvider } from 'fumadocs-ui/contexts/i18n';
import { DocsLayout } from 'fumadocs-ui/layouts/docs';
import { i18nProvider } from 'fumadocs-ui/i18n';
import { baseOptions, translations } from '@/lib/layout.shared';
import { source } from '@/lib/source';

export default function ChineseDocsLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <I18nProvider {...i18nProvider(translations, 'zh')}>
      <DocsLayout {...baseOptions('zh')} tree={source.getPageTree('zh')}>
        {children}
      </DocsLayout>
    </I18nProvider>
  );
}
