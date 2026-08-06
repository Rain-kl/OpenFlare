/**
 * 内置源站错误页 HTML 模板目录。
 * 每个模板对应本目录下一个独立 `.ts` 模块（export default 完整 HTML 字符串）。
 * 新增模板：新增 `*.ts` 并在下方数组注册。
 *
 * 模板须使用占位符 {{status}} / {{host}}，禁止写死状态码。
 */
import baohausHtml from './baohaus';
import lineStyleHtml from './line-style';
import minimalistHtml from './minimalist';

export type OriginErrorPageTemplate = {
  /** 稳定 ID，与文件名一致 */
  id: string;
  /** 下拉展示名称 */
  name: string;
  /** 完整 HTML，含 {{status}} / {{host}} */
  html: string;
};

export const ORIGIN_ERROR_PAGE_TEMPLATES: OriginErrorPageTemplate[] = [
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

/** 默认预制模板（前端预览空 HTML 回退 & 加载模板默认选中） */
export const DEFAULT_ORIGIN_ERROR_PAGE_TEMPLATE_ID = 'minimalist';

export function getOriginErrorPageTemplate(
  id: string,
): OriginErrorPageTemplate | undefined {
  return ORIGIN_ERROR_PAGE_TEMPLATES.find((item) => item.id === id);
}
