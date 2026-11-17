import assets from './pow-assets.generated.json';
import type { PoWPageConfig } from '@/lib/services/openflare/pow';

export const POW_PAGE_KEY = 'pow_page';
export const POW_HTML_MAX_BYTES = 256 * 1024;
export const powTemplates = assets.templates;

export function readPoWPage(value?: string): PoWPageConfig {
  if (value) {
    try {
      const parsed: Partial<PoWPageConfig> | null = JSON.parse(value);
      return {
        template: parsed?.template === 'classic' ? 'classic' : 'shield',
        widget: parsed?.widget === 'turnstile' ? 'turnstile' : 'progress',
        html: typeof parsed?.html === 'string' ? parsed.html : '',
      };
    } catch {
      /* Use the built-in page when no valid configuration has been saved. */
    }
  }
  return { template: 'shield', widget: 'progress', html: '' };
}

export function previewPoWPage(
  config: PoWPageConfig,
  locale: string,
  text: Record<string, string>,
): string {
  const escape = (value: string) =>
    value.replace(
      /[&<>"']/g,
      (char) =>
        ({
          '&': '&amp;',
          '<': '&lt;',
          '>': '&gt;',
          '"': '&quot;',
          "'": '&#39;',
        })[char]!,
    );
  const values = { ...text, lang: locale, host: 'example.com' };
  const html = (
    config.html?.trim() ? config.html : powTemplates[config.template]
  )
    .replace(/\{\{(\w+)\}\}/g, (token, key: string) => {
      if (key === 'widget')
        return `<openflare-pow appearance="${config.widget}"></openflare-pow>`;
      return Object.hasOwn(values, key)
        ? escape(values[key as keyof typeof values])
        : token;
    })
    .replace(
      '<link rel="stylesheet" href="/.within.website/x/cmd/anubis/static/css/xess.css">',
      () => `<style>${assets.css}</style>`,
    )
    .replace(
      '/.within.website/x/cmd/anubis/static/img/pensive.webp',
      `data:image/webp;base64,${assets.image}`,
    );
  const source = assets.widget.replace(
    'export function updateState',
    'function updateState',
  );
  const message = JSON.stringify(text.status).replace(/</g, '\\u003c');
  const success = JSON.stringify(text.success).replace(/</g, '\\u003c');
  const runtime = `<script type="module">${source}
    let progress = 0;
    updateState({status:'working', progress, message:${message}});
    setInterval(() => { progress = (progress + 5) % 125;
      updateState({status:progress >= 100 ? 'success' : 'working', progress:Math.min(progress,100), message:progress >= 100 ? ${success} : ${message}});
    }, 200);
  </script>`;
  return /<\/body\s*>/i.test(html)
    ? html.replace(/<\/body\s*>/i, () => `${runtime}</body>`)
    : html + runtime;
}
