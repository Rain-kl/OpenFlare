export const OPTIONS_QUERY_KEY = ['openflare', 'options'] as const;

export const KEY_SW_ENABLED = 'sw_offline_enabled';
export const KEY_SW_HTML = 'sw_offline_html';
export const KEY_SW_DOMAINS = 'sw_offline_domains';

export type ContactPageFields = {
  enabled: boolean;
  html: string;
  domains: string[];
};

export const defaultContactPageFields: ContactPageFields = {
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

export function mapOptionsToContactFields(
  optionMap: Record<string, string>,
): ContactPageFields {
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
