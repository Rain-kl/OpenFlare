export const UA_BROWSER_OPTIONS = [
  { value: 'Chrome', label: 'Chrome' },
  { value: 'Safari', label: 'Safari' },
  { value: 'Firefox', label: 'Firefox' },
  { value: 'Edge', label: 'Edge' },
  { value: 'Opera', label: 'Opera' },
  { value: 'Chromium', label: 'Chromium' },
  { value: 'WeChat', label: '微信' },
  { value: 'Postman', label: 'Postman' },
  { value: 'CLI', label: 'CLI' },
  { value: 'Bot', label: 'Bot' },
  { value: 'Unknown', label: 'Unknown' },
  { value: 'Other', label: 'Other' },
] as const;

export const UA_OS_OPTIONS = [
  { value: 'Android', label: 'Android' },
  { value: 'iOS', label: 'iOS' },
  { value: 'Windows', label: 'Windows' },
  { value: 'macOS', label: 'macOS' },
  { value: 'Chrome OS', label: 'Chrome OS' },
  { value: 'Linux', label: 'Linux' },
  { value: 'Bot', label: 'Bot' },
  { value: 'Unknown', label: 'Unknown' },
  { value: 'Other', label: 'Other' },
] as const;

export const UA_BROWSER_LABELS = new Set<string>(
  UA_BROWSER_OPTIONS.map((option) => option.value),
);
export const UA_OS_LABELS = new Set<string>(
  UA_OS_OPTIONS.map((option) => option.value),
);
