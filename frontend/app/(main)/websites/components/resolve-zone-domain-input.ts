/**
 * Resolve user input into a full FQDN under a Zone root.
 *
 * - `@` → apex (zone root itself), e.g. `example.com`
 * - single label `name` → `name.example.com`
 * - full FQDN must equal the root or end with `.root`
 */
export function resolveZoneDomainInput(
  rawInput: string,
  zoneRoot: string,
): { domain: string; error?: string } {
  const input = rawInput.trim().toLowerCase();
  const root = zoneRoot.trim().toLowerCase();

  if (!root) {
    return { domain: '', error: '请先选择 Zone' };
  }
  if (!input) {
    return { domain: '', error: '请输入域名' };
  }
  if (input.includes('*')) {
    return { domain: '', error: 'Zone 域名不支持通配符' };
  }
  if (
    input.includes('://') ||
    input.includes('/') ||
    input.includes('?') ||
    input.includes('#')
  ) {
    return { domain: '', error: '域名格式不合法' };
  }

  if (input === '@') {
    return { domain: root };
  }

  // Short label: no dots → prefix under zone root
  if (!input.includes('.')) {
    if (!/^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/i.test(input)) {
      return { domain: '', error: '子域名称格式不合法' };
    }
    return { domain: `${input}.${root}` };
  }

  // Full FQDN
  if (input === root || input.endsWith(`.${root}`)) {
    return { domain: input };
  }

  return { domain: '', error: `域名必须属于 Zone ${root}` };
}

/** Live preview string for the input helper text. */
export function previewZoneDomainInput(
  rawInput: string,
  zoneRoot: string,
): string {
  const resolved = resolveZoneDomainInput(rawInput, zoneRoot);
  if (resolved.error || !resolved.domain) {
    return '';
  }
  return resolved.domain;
}
