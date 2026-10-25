import { runtimeText } from '@/i18n/runtime';
/**
 * 内置离线页 HTML 模板目录。
 */
import baohausHtml from './baohaus';
import lineStyleHtml from './line-style';
import minimalistHtml from './minimalist';

export type OfflinePageTemplate = {
  /** 稳定 ID */
  id: string;
  /** 下拉展示名称 */
  name: string;
  /** 完整 HTML */
  html: string;
};

export const OFFLINE_PAGE_TEMPLATES: OfflinePageTemplate[] = [
  {
    id: 'minimalist',
    get name() {
      return runtimeText('minimal_white_5636e8a9');
    },
    get html() {
      return minimalistHtml();
    },
  },
  {
    id: 'line-style',
    get name() {
      return runtimeText('line_topology_22ad450c');
    },
    get html() {
      return lineStyleHtml();
    },
  },
  {
    id: 'baohaus',
    get name() {
      return runtimeText('bauhaus_5cc69781');
    },
    get html() {
      return baohausHtml();
    },
  },
];

/** 默认预制模板 */
export const DEFAULT_OFFLINE_PAGE_TEMPLATE_ID = 'minimalist';

export function effectiveOfflinePageHTML(html: string): string {
  return html.trim() === ''
    ? (getOfflinePageTemplate(DEFAULT_OFFLINE_PAGE_TEMPLATE_ID)?.html ?? '')
    : html;
}

export function getOfflinePageTemplate(
  id: string,
): OfflinePageTemplate | undefined {
  return OFFLINE_PAGE_TEMPLATES.find((item) => item.id === id);
}
