/**
 * Resolve user input into a full FQDN under a Zone root.
 *
 * - `@` → apex (zone root itself), e.g. `example.com`
 * - single label `name` → `name.example.com`
 * - `*` → `*.example.com`; other wildcards require a full FQDN
 * - full FQDN must equal the root or end with `.root`
 */
export type ZoneDomainInputError =
  | 'selectZone'
  | 'enterDomain'
  | 'noWildcard'
  | 'invalidFormat'
  | 'invalidSubdomain'
  | 'mustBelongToZone';

const dnsLabelPattern = /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/;

function isDnsName(value: string): boolean {
  return (
    value.length <= 253 &&
    value.split('.').every((label) => dnsLabelPattern.test(label))
  );
}

export function resolveZoneDomainInput(
  rawInput: string,
  zoneRoot: string,
): { domain: string; error?: ZoneDomainInputError } {
  const input = rawInput.trim().toLowerCase();
  const root = zoneRoot.trim().toLowerCase();

  if (!root) {
    return { domain: '', error: 'selectZone' };
  }
  if (!input) {
    return { domain: '', error: 'enterDomain' };
  }
  if (/\s/.test(input) || !isDnsName(root)) {
    return { domain: '', error: 'invalidFormat' };
  }

  if (input === '@') {
    return { domain: root };
  }

  // 星号只能作为最左侧完整标签；*.s3 不推断为 Zone 内的相对名称。
  const wildcard = input === '*' || input.startsWith('*.');
  if (input.includes('*') && (!wildcard || input.slice(1).includes('*'))) {
    return { domain: '', error: 'noWildcard' };
  }

  let domain = input;
  if (input === '*') {
    domain = `*.${root}`;
  } else if (!input.includes('.')) {
    if (!dnsLabelPattern.test(input)) {
      return { domain: '', error: 'invalidSubdomain' };
    }
    domain = `${input}.${root}`;
  }

  const hostname = wildcard ? domain.slice(2) : domain;
  if (domain.length > 253 || !isDnsName(hostname)) {
    return { domain: '', error: 'invalidFormat' };
  }
  if (hostname === root || hostname.endsWith(`.${root}`)) {
    return { domain };
  }

  return { domain: '', error: 'mustBelongToZone' };
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
