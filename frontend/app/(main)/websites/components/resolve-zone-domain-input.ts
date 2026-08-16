/**
 * Resolve user input into a full FQDN under a Zone root.
 *
 * - `@` → apex (zone root itself), e.g. `example.com`
 * - single label `name` → `name.example.com`
 * - full FQDN must equal the root or end with `.root`
 */
export type ZoneDomainInputError =
  | 'selectZone'
  | 'enterDomain'
  | 'noWildcard'
  | 'invalidFormat'
  | 'invalidSubdomain'
  | 'mustBelongToZone';

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
  if (input.includes('*')) {
    return { domain: '', error: 'noWildcard' };
  }
  if (
    input.includes('://') ||
    input.includes('/') ||
    input.includes('?') ||
    input.includes('#')
  ) {
    return { domain: '', error: 'invalidFormat' };
  }

  if (input === '@') {
    return { domain: root };
  }

  // Short label: no dots → prefix under zone root
  if (!input.includes('.')) {
    if (!/^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/i.test(input)) {
      return { domain: '', error: 'invalidSubdomain' };
    }
    return { domain: `${input}.${root}` };
  }

  // Full FQDN
  if (input === root || input.endsWith(`.${root}`)) {
    return { domain: input };
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
