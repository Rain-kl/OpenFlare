import type { Blockquote, Root, RootContent } from 'mdast';
import type { Plugin } from 'unified';

const alertTypes = {
  NOTE: { type: 'info', zh: '说明', en: 'Note' },
  TIP: { type: 'success', zh: '提示', en: 'Tip' },
  IMPORTANT: { type: 'idea', zh: '重要信息', en: 'Important' },
  WARNING: { type: 'warning', zh: '警告', en: 'Warning' },
  CAUTION: { type: 'error', zh: '注意', en: 'Caution' },
} as const;

type MdxCallout = {
  type: 'mdxJsxFlowElement';
  name: 'Callout';
  attributes: Array<{
    type: 'mdxJsxAttribute';
    name: 'type' | 'title';
    value: string;
  }>;
  children: RootContent[];
};

function alertNode(node: RootContent, path: string): MdxCallout | undefined {
  if (node.type !== 'blockquote') return;

  const quote = node as Blockquote;
  const marker = quote.children[0];
  if (marker?.type !== 'paragraph') return;

  const text = marker.children
    .map((child) => (child.type === 'text' ? child.value : ''))
    .join('')
    .trim();
  const match = /^\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]$/i.exec(text);
  if (!match) return;

  const kind = match[1].toUpperCase() as keyof typeof alertTypes;
  const alert = alertTypes[kind];
  const language = /(?:^|[\\/])en(?:[\\/]|$)/.test(path) ? 'en' : 'zh';

  return {
    type: 'mdxJsxFlowElement',
    name: 'Callout',
    attributes: [
      { type: 'mdxJsxAttribute', name: 'type', value: alert.type },
      { type: 'mdxJsxAttribute', name: 'title', value: alert[language] },
    ],
    children: quote.children.slice(1),
  };
}

export const remarkGithubAlerts: Plugin<[], Root> = () => (tree, file) => {
  const path = file.path ?? '';

  const visit = (children: RootContent[]) => {
    for (let index = 0; index < children.length; index += 1) {
      const child = children[index];
      if (child.type === 'blockquote') {
        const converted = alertNode(child, path);
        if (converted) {
          children[index] = converted as unknown as RootContent;
          visit(converted.children);
          continue;
        }
      }

      if ('children' in child) visit(child.children as RootContent[]);
    }
  };

  visit(tree.children);
};
