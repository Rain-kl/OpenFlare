import { readFileSync, writeFileSync } from 'node:fs';

const root = new URL('../../', import.meta.url);
const brandMark = readFileSync(
  new URL(
    'internal/openflare/edgeconfig/pow_static/img/openflare-mark.svg',
    root,
  ),
  'utf8',
).trim();

function readBrandedTemplate(path) {
  return readFileSync(new URL(path, root), 'utf8').replaceAll(
    '<!-- openflare-brand -->',
    brandMark,
  );
}

function syncAsset(filename, value) {
  const target = new URL(`frontend/lib/openflare/${filename}`, root);
  const output = `${JSON.stringify(value, null, 2)}\n`;
  if (process.argv.includes('--check')) {
    if (readFileSync(target, 'utf8') !== output)
      throw new Error(
        `Run node scripts/generate-pow-assets.mjs to sync ${filename}.`,
      );
  } else {
    writeFileSync(target, output);
  }
}

const templates = Object.fromEntries(
  ['classic', 'shield'].map((id) => [
    id,
    readBrandedTemplate(
      `internal/openflare/edgeconfig/pow_templates/${id}.html`,
    ),
  ]),
);
const widget = readFileSync(
  new URL('internal/openflare/edgeconfig/pow_static/js/widget.mjs', root),
  'utf8',
);
const css = readFileSync(
  new URL('internal/openflare/edgeconfig/pow_static/css/xess.css', root),
  'utf8',
);
const image = readFileSync(
  new URL('internal/openflare/edgeconfig/pow_static/img/pensive.webp', root),
).toString('base64');
syncAsset('pow-assets.generated.json', { templates, widget, css, image });
syncAsset(
  'site-auth-page.generated.json',
  readBrandedTemplate('internal/openflare/edgeconfig/site_auth.html'),
);
syncAsset('brand-mark.generated.json', brandMark);
