import defaultSiteAuthHTML from '@/lib/openflare/site-auth-page.generated.json';

export const SITE_AUTH_HTML_KEY = 'site_auth_html';
export const SITE_AUTH_HTML_MAX_BYTES = 256 * 1024;

export { defaultSiteAuthHTML };

export function previewSiteAuthHTML(html: string) {
  return (html.trim() ? html : defaultSiteAuthHTML).replaceAll(
    '{{login_error}}',
    '',
  );
}
