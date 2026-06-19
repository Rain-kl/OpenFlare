import type {ProxyRouteConfigSection, ProxyRouteItem} from '@/lib/services/openflare';

export type DomainListRow = {
  domain: string;
  certificateId: string;
};

export const proxyRouteFormIds: Record<ProxyRouteConfigSection, string> = {
  domains: 'proxy-route-domains-form',
  limits: 'proxy-route-limits-form',
  proxy: 'proxy-route-proxy-form',
  cache: 'proxy-route-cache-form',
  waf: 'proxy-route-waf-form',
  auth: 'proxy-route-auth-form',
};

function ensureRows(rows: DomainListRow[]) {
  return rows.length > 0 ? rows : [{ domain: '', certificateId: '' }];
}

export function buildDomainRowsFromRoute(
  domains: string[],
  domainCertIDs: number[],
  certIDs: number[],
): DomainListRow[] {
  if (domains.length === 0) {
    return ensureRows([]);
  }

  if (domainCertIDs.length === domains.length) {
    return domains.map((domain, index) => ({
      domain,
      certificateId: domainCertIDs[index] ? String(domainCertIDs[index]) : '',
    }));
  }

  if (certIDs.length === 0) {
    return domains.map((domain) => ({ domain, certificateId: '' }));
  }

  if (certIDs.length === 1) {
    return domains.map((domain) => ({
      domain,
      certificateId: String(certIDs[0]),
    }));
  }

  return domains.map((domain, index) => ({
    domain,
    certificateId: certIDs[index] ? String(certIDs[index]) : '',
  }));
}

export function normalizeSelectedCertificateIDs(rows: DomainListRow[]) {
  return Array.from(
    new Set(
      rows
        .filter((item) => item.domain.trim() !== '')
        .map((item) => Number(item.certificateId))
        .filter((item) => Number.isFinite(item) && item > 0),
    ),
  );
}

export function buildDomainCertificateIDs(rows: DomainListRow[]) {
  return rows
    .filter((item) => item.domain.trim() !== '')
    .map((item) => {
      const certificateID = Number(item.certificateId);
      return Number.isFinite(certificateID) && certificateID > 0 ? certificateID : 0;
    });
}

export function buildDomainRows(route: ProxyRouteItem) {
  const selectedCertIDs =
    route.cert_ids.length > 0
      ? route.cert_ids
      : route.cert_id
        ? [route.cert_id]
        : [];

  return buildDomainRowsFromRoute(route.domains, route.domain_cert_ids, selectedCertIDs);
}

export function submitProxyRouteSectionForm(section: ProxyRouteConfigSection) {
  const form = document.getElementById(proxyRouteFormIds[section]);
  if (form instanceof HTMLFormElement) {
    form.requestSubmit();
    return true;
  }
  return false;
}