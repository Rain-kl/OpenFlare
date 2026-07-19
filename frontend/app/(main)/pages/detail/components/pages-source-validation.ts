function hasWellFormedUnicode(value: string) {
  for (let index = 0; index < value.length; index += 1) {
    const code = value.charCodeAt(index);
    if (code >= 0xd800 && code <= 0xdbff) {
      const next = value.charCodeAt(index + 1);
      if (!(next >= 0xdc00 && next <= 0xdfff)) return false;
      index += 1;
    } else if (code >= 0xdc00 && code <= 0xdfff) {
      return false;
    }
  }
  return true;
}

function validGitHubSafeText(value: string) {
  return (
    value !== '' &&
    new TextEncoder().encode(value).byteLength <= 255 &&
    !/[\u0000-\u001f\u007f-\u009f\u061c\u200e\u200f\u2028-\u202e\u2066-\u2069]/u.test(
      value,
    ) &&
    hasWellFormedUnicode(value)
  );
}

export function validGitHubRepositoryURL(raw: string) {
  const match = /^https:\/\/([^/]+)\/([^/]+)\/([^/]+)$/u.exec(raw);
  if (!match) return false;

  const [, host, owner, rawRepository] = match;
  const repository = rawRepository.replace(/\.git$/u, '');
  return (
    host.toLowerCase() === 'github.com' &&
    /^[a-z0-9](?:[a-z0-9-]{0,37}[a-z0-9])?$/iu.test(owner) &&
    /^[a-z0-9._-]+$/iu.test(repository) &&
    repository.length <= 100 &&
    !['.', '..'].includes(repository)
  );
}

export function validGitHubAssetName(value: string) {
  return (
    validGitHubSafeText(value) &&
    value !== '.' &&
    value !== '..' &&
    !value.includes('/') &&
    !value.includes('\\')
  );
}

export function validGitHubReleaseTag(value: string) {
  const components = value.split('/');
  return (
    validGitHubSafeText(value) &&
    !value.endsWith('.') &&
    !value.includes('..') &&
    !value.includes('@{') &&
    ![' ', '~', '^', ':', '?', '*', '[', '\\'].some((character) =>
      value.includes(character),
    ) &&
    components.every(
      (component) =>
        component !== '' &&
        !component.startsWith('.') &&
        !component.endsWith('.lock'),
    )
  );
}
