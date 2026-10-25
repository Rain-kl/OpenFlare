import { runtimeText } from '@/i18n/runtime';
/**
 * 内置源站错误页 HTML 模板目录。
 * 每个模板对应本目录下一个独立 `.ts` 模块（export default 返回完整 HTML 的函数）。
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

/** 默认预制模板（前端预览空 HTML 回退 & 加载模板默认选中） */
export const DEFAULT_ORIGIN_ERROR_PAGE_TEMPLATE_ID = 'minimalist';

export function getOriginErrorPageTemplate(
  id: string,
): OriginErrorPageTemplate | undefined {
  return ORIGIN_ERROR_PAGE_TEMPLATES.find((item) => item.id === id);
}
