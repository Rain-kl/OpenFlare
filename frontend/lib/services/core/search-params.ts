/**
 * Serialize query params for Gin-compatible repeated array keys.
 * Axios default uses hosts[]= which Gin QueryArray("hosts") ignores.
 */
export function serializeSearchParams(
  params?: Record<string, unknown>,
): string {
  if (!params) return '';
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === '') continue;
    if (Array.isArray(value)) {
      for (const item of value) {
        if (item === undefined || item === null || item === '') continue;
        search.append(key, String(item));
      }
      continue;
    }
    search.append(key, String(value));
  }
  return search.toString();
}
