import { describe, expect, it } from 'vitest';
import { previewPoWPage, readPoWPage } from './pow-page';

const text = {
  title: '<unsafe>',
  status: '</script><script>bad()</script>',
  success: 'Success',
};

describe('PoW page preview', () => {
  it.each(['classic', 'shield'] as const)(
    'renders %s with both live widget styles',
    (template) => {
      for (const widget of ['progress', 'turnstile'] as const) {
        const html = previewPoWPage({ template, widget }, 'en', text);
        expect(html).toContain(`<openflare-pow appearance="${widget}">`);
        expect(html).toContain('class="brand-icon"');
        expect(html).toContain('data:image/svg+xml,%3Csvg');
        expect(html).not.toContain('<!-- openflare-brand -->');
        expect(html).toContain('customElements.define');
        expect(html).toContain('&lt;unsafe&gt;');
        expect(html).toContain('\\u003c/script>');
        expect(html).not.toContain('<script>bad()');
      }
    },
  );

  it('injects the runtime into custom fragments without a body element', () => {
    const html = previewPoWPage(
      {
        template: 'classic',
        widget: 'progress',
        html: '<main>{{widget}}</main>',
      },
      'en',
      text,
    );
    expect(html).toContain('<main><openflare-pow');
    expect(html).toContain('<script type="module">');
  });

  it.each([undefined, 'null', '{}', '{"template":"","widget":""}', 'invalid'])(
    'normalizes empty configuration %s',
    (raw) => {
      expect(readPoWPage(raw)).toEqual({
        template: 'shield',
        widget: 'progress',
        html: '',
      });
    },
  );
});
