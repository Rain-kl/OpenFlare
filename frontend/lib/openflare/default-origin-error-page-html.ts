import {
  DEFAULT_ORIGIN_ERROR_PAGE_TEMPLATE_ID,
  getOriginErrorPageTemplate,
} from '@/lib/openflare/origin-error-page-templates';

/**
 * 预览/回退用的默认 HTML：与内置模板目录中的 minimalist 对齐。
 * 边缘侧空配置时的默认页仍由 Go DefaultOriginErrorPageHTML 提供。
 */
export const DEFAULT_ORIGIN_ERROR_PAGE_HTML =
  getOriginErrorPageTemplate(DEFAULT_ORIGIN_ERROR_PAGE_TEMPLATE_ID)?.html ?? '';

/** Max HTML size in bytes (aligned with backend 256 KiB). */
export const ORIGIN_ERROR_PAGE_HTML_MAX_BYTES = 256 * 1024;

export function effectiveOriginErrorPageHTML(html: string): string {
  return html.trim() === '' ? DEFAULT_ORIGIN_ERROR_PAGE_HTML : html;
}

export function previewOriginErrorPageHTML(
  html: string,
  status = '502',
  host = 'example.com',
): string {
  return effectiveOriginErrorPageHTML(html)
    .replaceAll('{{status}}', status)
    .replaceAll('{{host}}', host);
}
