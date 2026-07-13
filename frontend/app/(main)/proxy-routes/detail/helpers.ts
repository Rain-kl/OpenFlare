import type { ProxyRouteConfigSection } from '@/lib/services/openflare';

export const proxyRouteFormIds: Record<ProxyRouteConfigSection, string> = {
  domains: 'proxy-route-domains-form',
  limits: 'proxy-route-limits-form',
  proxy: 'proxy-route-proxy-form',
  cache: 'proxy-route-cache-form',
  waf: 'proxy-route-waf-form',
  auth: 'proxy-route-auth-form',
};

export function submitProxyRouteSectionForm(section: ProxyRouteConfigSection) {
  const form = document.getElementById(proxyRouteFormIds[section]);
  if (form instanceof HTMLFormElement) {
    form.requestSubmit();
    return true;
  }
  return false;
}
