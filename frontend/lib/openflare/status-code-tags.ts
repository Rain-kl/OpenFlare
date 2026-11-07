import { runtimeText } from '@/i18n/runtime';
/** Matches pkg/render/openresty StatusCodeMin / StatusCodeMax. */
export const STATUS_CODE_MIN = 400;
export const STATUS_CODE_MAX = 599;

export const DEFAULT_ORIGIN_ERROR_PAGE_STATUS_TAGS = ['500-599'] as const;

/**
 * Parse a single tag such as "502" or "500-599".
 * Bounds must fall within 400–599 inclusive (aligned with Go ParseStatusCodeTag).
 */
export function parseStatusCodeTag(tag: string): {
  lo: number;
  hi: number;
} {
  const trimmed = tag.trim();
  if (!trimmed) {
    throw new Error(runtimeText('status_code_tag_must_not_be_empty_21235ebf'));
  }

  let lo: number;
  let hi: number;

  const dash = trimmed.indexOf('-');
  if (dash >= 0) {
    lo = Number.parseInt(trimmed.slice(0, dash), 10);
    hi = Number.parseInt(trimmed.slice(dash + 1), 10);
    if (Number.isNaN(lo) || Number.isNaN(hi)) {
      throw new Error(
        runtimeText('invalid_status_code_range_value0_8983e96e', {
          value0: trimmed,
        }),
      );
    }
  } else {
    lo = Number.parseInt(trimmed, 10);
    if (Number.isNaN(lo)) {
      throw new Error(
        runtimeText('invalid_status_code_value0_0f206e48', { value0: trimmed }),
      );
    }
    hi = lo;
  }

  if (lo > hi) {
    throw new Error(
      runtimeText('status_code_range_endpoints_are_reversed_value0_d92c2784', {
        value0: trimmed,
      }),
    );
  }
  if (lo < STATUS_CODE_MIN || hi > STATUS_CODE_MAX) {
    throw new Error(
      runtimeText('status_code_must_be_between_value0_value1_c5b04822', {
        value0: STATUS_CODE_MIN,
        value1: STATUS_CODE_MAX,
        value2: trimmed,
      }),
    );
  }

  return { lo, hi };
}

/** Expand tags into a sorted unique list of integers. */
export function expandStatusCodeTags(tags: string[]): number[] {
  const set = new Set<number>();
  for (const tag of tags) {
    const { lo, hi } = parseStatusCodeTag(tag);
    for (let code = lo; code <= hi; code += 1) {
      set.add(code);
    }
  }
  return Array.from(set).sort((a, b) => a - b);
}

/** Soft validator for TagsInput: returns error message or null if ok. */
export function validateStatusCodeTagMessage(tag: string): string | null {
  try {
    parseStatusCodeTag(tag);
    return null;
  } catch (error) {
    return error instanceof Error
      ? error.message
      : runtimeText('invalid_status_code_tag_d16e85d7');
  }
}

/** Validate a full tag list before save (must be non-empty and expand cleanly). */
export function validateStatusCodeTags(tags: string[]): void {
  if (tags.length === 0) {
    throw new Error(runtimeText('add_at_least_one_status_code_tag_b81b2696'));
  }
  const codes = expandStatusCodeTags(tags);
  if (codes.length === 0) {
    throw new Error(
      runtimeText('the_expanded_status_code_list_is_empty_47245f34'),
    );
  }
}

export function parseStatusCodeTagsJSON(raw: string | undefined): string[] {
  if (!raw || !raw.trim()) {
    return [...DEFAULT_ORIGIN_ERROR_PAGE_STATUS_TAGS];
  }
  try {
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) {
      return [...DEFAULT_ORIGIN_ERROR_PAGE_STATUS_TAGS];
    }
    const tags = parsed
      .filter((item): item is string => typeof item === 'string')
      .map((item) => item.trim())
      .filter(Boolean);
    return tags.length > 0 ? tags : [...DEFAULT_ORIGIN_ERROR_PAGE_STATUS_TAGS];
  } catch {
    return [...DEFAULT_ORIGIN_ERROR_PAGE_STATUS_TAGS];
  }
}
