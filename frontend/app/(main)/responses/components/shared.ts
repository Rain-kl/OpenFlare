import {
  DEFAULT_ORIGIN_ERROR_PAGE_STATUS_TAGS,
  parseStatusCodeTagsJSON,
} from '@/lib/openflare/status-code-tags';

export const OPTIONS_QUERY_KEY = ['openflare', 'options'] as const;

export const KEY_ENABLED = 'origin_error_page_enabled';
export const KEY_STATUS_CODES = 'origin_error_page_status_codes';
export const KEY_HTML = 'origin_error_page_html';
export const KEY_GET_ONLY = 'origin_error_page_get_only';

export const KEY_SW_ENABLED = 'sw_offline_enabled';
export const KEY_SW_HTML = 'sw_offline_html';
export const KEY_SW_DOMAINS = 'sw_offline_domains';

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

export type OfflinePageFields = {
  enabled: boolean;
  html: string;
  domains: string[];
};

export const defaultOfflinePageFields: OfflinePageFields = {
  enabled: false,
  html: '',
  domains: [],
};

export function optionsToMap(options: Array<{ key: string; value: string }>) {
  return options.reduce<Record<string, string>>((acc, option) => {
    acc[option.key] = option.value;
    return acc;
  }, {});
}

function parseDomains(raw: string | undefined): string[] {
  if (!raw) return [];
  try {
    const parsed: unknown = JSON.parse(raw);
    return Array.isArray(parsed)
      ? parsed.filter((item): item is string => typeof item === 'string')
      : [];
  } catch {
    return [];
  }
}

export function mapOptionsToErrorFields(
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

export function mapOptionsToOfflineFields(
  optionMap: Record<string, string>,
): OfflinePageFields {
  return {
    enabled: optionMap[KEY_SW_ENABLED] === 'true',
    html: optionMap[KEY_SW_HTML] ?? '',
    domains: parseDomains(optionMap[KEY_SW_DOMAINS]),
  };
}

export async function invalidateResponseQueries(queryClient: {
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
