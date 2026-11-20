import brandMark from './brand-mark.generated.json';

export function withBrandMark(html: string): string {
  return html.replaceAll('<!-- openflare-brand -->', brandMark);
}
