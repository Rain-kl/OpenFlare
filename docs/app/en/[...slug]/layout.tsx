import { I18nProvider } from 'fumadocs-ui/contexts/i18n';
import { i18nProvider } from 'fumadocs-ui/i18n';
import { DocsLayout } from 'fumadocs-ui/layouts/docs';
import { baseOptions, translations } from '@/lib/layout.shared';
import { source } from '@/lib/source';

export default async function EnglishDocsLayout({
  children,
  params,
}: Readonly<{
  children: React.ReactNode;
  params: Promise<{ slug: string[] }>;
}>) {
  const { slug } = await params;

  return (
    <I18nProvider {...i18nProvider(translations, 'en')}>
      <DocsLayout
        tree={source.getPageTree('en')}
        {...baseOptions('en', slug)}
      >
        {children}
      </DocsLayout>
    </I18nProvider>
  );
}
