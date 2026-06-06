import type { ManagedDomainItem } from '@/features/managed-domains/types';

export function isWildcardManagedDomain(domain: string) {
  return domain.startsWith('*.');
}

export function buildRouteDomain(
  managedDomain: string | undefined,
  subdomainLabel: string,
) {
  if (!managedDomain) {
    return '';
  }

  if (!isWildcardManagedDomain(managedDomain)) {
    return managedDomain.toLowerCase();
  }

  const normalizedLabel = subdomainLabel.trim().toLowerCase();
  if (!normalizedLabel) {
    return '';
  }

  return `${normalizedLabel}.${managedDomain.slice(2).toLowerCase()}`;
}

export function findManagedDomainForRoute(
  routeDomain: string,
  managedDomains: ManagedDomainItem[],
) {
  const normalizedRouteDomain = routeDomain.trim().toLowerCase();
  const exactMatch = managedDomains.find(
    (item) => item.domain.toLowerCase() === normalizedRouteDomain,
  );

  if (exactMatch) {
    return {
      managedDomainId: exactMatch.domain,
      subdomainLabel: '',
    };
  }

  const wildcardMatch = managedDomains.find((item) => {
    if (!isWildcardManagedDomain(item.domain)) {
      return false;
    }

    const suffix = item.domain.slice(2).toLowerCase();
    const suffixWithDot = `.${suffix}`;
    if (!normalizedRouteDomain.endsWith(suffixWithDot)) {
      return false;
    }

    const label = normalizedRouteDomain.slice(
      0,
      normalizedRouteDomain.length - suffixWithDot.length,
    );

    return Boolean(label) && !label.includes('.');
  });

  if (!wildcardMatch) {
    return null;
  }

  return {
    managedDomainId: wildcardMatch.domain,
    subdomainLabel: normalizedRouteDomain.slice(
      0,
      normalizedRouteDomain.length - wildcardMatch.domain.slice(1).length,
    ),
  };
}
