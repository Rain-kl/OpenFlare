import { notFound } from 'next/navigation';
import { createRelativeLink } from 'fumadocs-ui/mdx';
import { DocsBody, DocsPage } from 'fumadocs-ui/page';
import { getMdxComponents } from '@/components/mdx';
import { source } from '@/lib/source';

export function generateStaticParams() {
  return source.getPages('en').map((page) => ({ slug: page.slugs }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string[] }>;
}) {
  const { slug } = await params;
  const page = source.getPage(slug, 'en');
  if (!page) return {};

  return {
    title: page.data.title,
    description: page.data.description,
  };
}

export default async function EnglishDocsPage({
  params,
}: {
  params: Promise<{ slug: string[] }>;
}) {
  const { slug } = await params;
  const page = source.getPage(slug, 'en');
  if (!page) notFound();

  const MDXContent = page.data.body;
  return (
    <DocsPage toc={page.data.toc} editOnGithub={{
      owner: 'Rain-kl',
      repo: 'OpenFlare',
      path: `docs/${page.path}`,
    }}>
      <DocsBody>
        <MDXContent components={getMdxComponents({ a: createRelativeLink(source, page) })} />
      </DocsBody>
    </DocsPage>
  );
}
