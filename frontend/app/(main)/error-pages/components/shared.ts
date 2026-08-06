import { ORIGIN_ERROR_PAGE_HTML_MAX_BYTES } from '@/lib/openflare/default-origin-error-page-html';
import {
  DEFAULT_ORIGIN_ERROR_PAGE_STATUS_TAGS,
  parseStatusCodeTagsJSON,
  validateStatusCodeTags,
} from '@/lib/openflare/status-code-tags';

export const OPTIONS_QUERY_KEY = ['openflare', 'options'] as const;

export const KEY_ENABLED = 'origin_error_page_enabled';
export const KEY_STATUS_CODES = 'origin_error_page_status_codes';
export const KEY_HTML = 'origin_error_page_html';
export const KEY_GET_ONLY = 'origin_error_page_get_only';

export type ErrorPageFields = {
  enabled: boolean;
  getOnly: boolean;
  statusCodes: string[];
  html: string;
};

export const defaultErrorPageFields: ErrorPageFields = {
  enabled: true,
  getOnly: false,
  statusCodes: [...DEFAULT_ORIGIN_ERROR_PAGE_STATUS_TAGS],
  html: '',
};

export function optionsToMap(options: Array<{ key: string; value: string }>) {
  return options.reduce<Record<string, string>>((acc, option) => {
    acc[option.key] = option.value;
    return acc;
  }, {});
}

export function mapOptionsToFields(
  optionMap: Record<string, string>,
): ErrorPageFields {
  const enabledRaw = optionMap[KEY_ENABLED];
  const getOnlyRaw = optionMap[KEY_GET_ONLY];
  return {
    enabled: enabledRaw === undefined ? true : enabledRaw === 'true',
    getOnly: getOnlyRaw === undefined ? false : getOnlyRaw === 'true',
    statusCodes: parseStatusCodeTagsJSON(optionMap[KEY_STATUS_CODES]),
    html: optionMap[KEY_HTML] ?? '',
  };
}

export function validateErrorPageFields(fields: ErrorPageFields) {
  validateStatusCodeTags(fields.statusCodes);
  validateErrorPageHTML(fields.html);
}

export function validateErrorPageHTML(html: string) {
  const htmlBytes = new TextEncoder().encode(html).length;
  if (htmlBytes > ORIGIN_ERROR_PAGE_HTML_MAX_BYTES) {
    throw new Error(
      `HTML 超过最大长度限制（${ORIGIN_ERROR_PAGE_HTML_MAX_BYTES} 字节）`,
    );
  }
}

export async function invalidateErrorPageQueries(queryClient: {
  invalidateQueries: (opts: {
    queryKey: readonly unknown[];
  }) => Promise<unknown>;
}) {
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: OPTIONS_QUERY_KEY }),
    queryClient.invalidateQueries({
      queryKey: ['openflare', 'config-preview'],
    }),
    queryClient.invalidateQueries({
      queryKey: ['openflare', 'config-versions'],
    }),
  ]);
}
