import { defineConfig, defineDocs } from 'fumadocs-mdx/config';
import { pageSchema } from 'fumadocs-core/source/schema';
import { z } from 'zod';

const sections = ['guide', 'deployment', 'reference', 'design', 'changelog'];
const docsFiles = sections.flatMap((section) => [
  `${section}/**/*.md`,
  `en/${section}/**/*.md`,
]);
const metaFiles = [
  'meta.json',
  'en/meta.json',
  ...sections.flatMap((section) => [
    `${section}/meta.json`,
    `en/${section}/meta.json`,
  ]),
];

export const docs = defineDocs({
  dir: '.',
  docs: {
    files: docsFiles,
    schema: ({ path, source }) => {
      const markdown = source
        .replace(/^---\s*\r?\n[\s\S]*?\r?\n---\s*(?:\r?\n|$)/, '')
        .replace(/```[\s\S]*?```|~~~[\s\S]*?~~~/g, '');
      const title = markdown.match(/^#\s+(.+?)\s*#*\s*$/m)?.[1]?.trim();
      const fallback = path
        .split('/')
        .at(-1)
        ?.replace(/\.mdx?$/, '')
        .replace(/[-_]/g, ' ');

      return pageSchema.extend({
        title: z.string().default(title || fallback || 'Documentation'),
      });
    },
  },
  meta: { files: metaFiles },
});

export default defineConfig();
