import { notFound } from 'next/navigation';
import defaultMdxComponents, { createRelativeLink } from 'fumadocs-ui/mdx';
import { DocsBody, DocsPage, DocsTitle } from 'fumadocs-ui/page';
import { source } from '@/lib/source';

export function generateStaticParams() {
  return source.getPages('zh').map((page) => ({ slug: page.slugs }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string[] }>;
}) {
  const { slug } = await params;
  const page = source.getPage(slug, 'zh');
  if (!page) return {};

  return {
    title: page.data.title,
    description: page.data.description,
  };
}

export default async function ChineseDocsPage({
  params,
}: {
  params: Promise<{ slug: string[] }>;
}) {
  const { slug } = await params;
  const page = source.getPage(slug, 'zh');
  if (!page) notFound();

  const MDXContent = page.data.body;
  return (
    <DocsPage toc={page.data.toc} editOnGithub={{
      owner: 'Rain-kl',
      repo: 'OpenFlare',
      path: `docs/${page.path}`,
    }}>
      <DocsTitle>{page.data.title}</DocsTitle>
      <DocsBody>
        <MDXContent
          components={{
            ...defaultMdxComponents,
            a: createRelativeLink(source, page),
          }}
        />
      </DocsBody>
    </DocsPage>
  );
}
