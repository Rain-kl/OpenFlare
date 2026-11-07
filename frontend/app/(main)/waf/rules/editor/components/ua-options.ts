import { runtimeText } from '@/i18n/runtime';
export const UA_BROWSER_OPTIONS = [
  { value: 'Chrome', label: 'Chrome' },
  { value: 'Safari', label: 'Safari' },
  { value: 'Firefox', label: 'Firefox' },
  { value: 'Edge', label: 'Edge' },
  { value: 'Opera', label: 'Opera' },
  { value: 'Chromium', label: 'Chromium' },
  {
    value: 'WeChat',
    get label() {
      return runtimeText('wechat_81b430b6');
    },
  },
  { value: 'Postman', label: 'Postman' },
  { value: 'CLI', label: 'CLI' },
  {
    value: 'Bot',
    get label() {
      return runtimeText('ui_botLabel');
    },
  },
  {
    value: 'Unknown',
    get label() {
      return runtimeText('ui_unknownLabel');
    },
  },
  {
    value: 'Other',
    get label() {
      return runtimeText('ui_otherLabel');
    },
  },
] as const;

export const UA_OS_OPTIONS = [
  { value: 'Android', label: 'Android' },
  { value: 'iOS', label: 'iOS' },
  { value: 'Windows', label: 'Windows' },
  { value: 'macOS', label: 'macOS' },
  { value: 'Chrome OS', label: 'Chrome OS' },
  { value: 'Linux', label: 'Linux' },
  {
    value: 'Bot',
    get label() {
      return runtimeText('ui_botLabel');
    },
  },
  {
    value: 'Unknown',
    get label() {
      return runtimeText('ui_unknownLabel');
    },
  },
  {
    value: 'Other',
    get label() {
      return runtimeText('ui_otherLabel');
    },
  },
] as const;

export const UA_BROWSER_LABELS = new Set<string>(
  UA_BROWSER_OPTIONS.map((option) => option.value),
);
export const UA_OS_LABELS = new Set<string>(
  UA_OS_OPTIONS.map((option) => option.value),
);
