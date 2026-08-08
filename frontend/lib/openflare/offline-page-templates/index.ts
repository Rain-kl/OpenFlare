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
    name: '极简白底',
    html: minimalistHtml,
  },
  {
    id: 'line-style',
    name: '线框拓扑',
    html: lineStyleHtml,
  },
  {
    id: 'baohaus',
    name: '包豪斯',
    html: baohausHtml,
  },
];

/** 默认预制模板 */
export const DEFAULT_OFFLINE_PAGE_TEMPLATE_ID = 'minimalist';

export const DEFAULT_OFFLINE_PAGE_HTML =
  getOfflinePageTemplate(DEFAULT_OFFLINE_PAGE_TEMPLATE_ID)?.html ?? '';

export function effectiveOfflinePageHTML(html: string): string {
  return html.trim() === '' ? DEFAULT_OFFLINE_PAGE_HTML : html;
}

export function getOfflinePageTemplate(
  id: string,
): OfflinePageTemplate | undefined {
  return OFFLINE_PAGE_TEMPLATES.find((item) => item.id === id);
}
