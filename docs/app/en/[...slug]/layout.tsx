import { I18nProvider } from 'fumadocs-ui/contexts/i18n';
import { i18nProvider } from 'fumadocs-ui/i18n';
import { DocsLayout } from 'fumadocs-ui/layouts/docs';
import { baseOptions, translations } from '@/lib/layout.shared';
import { source } from '@/lib/source';

export default function EnglishDocsLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <I18nProvider {...i18nProvider(translations, 'en')}>
      <DocsLayout {...baseOptions('en')} tree={source.getPageTree('en')}>
        {children}
      </DocsLayout>
    </I18nProvider>
  );
}
