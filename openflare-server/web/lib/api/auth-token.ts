const openFlareTokenStorageKey = 'OpenFlare-Token';

export function getStoredOpenFlareToken() {
  if (typeof window === 'undefined') {
    return '';
  }
  return window.localStorage.getItem(openFlareTokenStorageKey) || '';
}

export function setStoredOpenFlareToken(token: string) {
  if (typeof window === 'undefined') {
    return;
  }
  window.localStorage.setItem(openFlareTokenStorageKey, token);
}

export function clearStoredOpenFlareToken() {
  if (typeof window === 'undefined') {
    return;
  }
  window.localStorage.removeItem(openFlareTokenStorageKey);
}
