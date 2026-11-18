import { readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const root = new URL('../../', import.meta.url);
const templates = Object.fromEntries(
  ['classic', 'shield'].map((id) => [
    id,
    readFileSync(
      new URL(`internal/openflare/openresty/pow_templates/${id}.html`, root),
      'utf8',
    ),
  ]),
);
const widget = readFileSync(
  new URL('internal/edge/agent/nginx/pow_static/js/widget.mjs', root),
  'utf8',
);
const css = readFileSync(
  new URL('internal/edge/agent/nginx/pow_static/css/xess.css', root),
  'utf8',
);
const image = readFileSync(
  new URL('internal/edge/agent/nginx/pow_static/img/pensive.webp', root),
).toString('base64');
const output = `${JSON.stringify({ templates, widget, css, image }, null, 2)}\n`;
const target = fileURLToPath(
  new URL('frontend/lib/openflare/pow-assets.generated.json', root),
);
if (process.argv.includes('--check')) {
  if (readFileSync(target, 'utf8') !== output)
    throw new Error(
      'Run node scripts/generate-pow-assets.mjs to sync PoW assets.',
    );
} else {
  writeFileSync(target, output);
}

const siteAuthTemplate = readFileSync(
  new URL('internal/openflare/openresty/site_auth.html', root),
  'utf8',
);
const siteAuthOutput = `${JSON.stringify(siteAuthTemplate)}\n`;
const siteAuthTarget = new URL(
  'frontend/lib/openflare/site-auth-page.generated.json',
  root,
);
if (process.argv.includes('--check')) {
  if (readFileSync(siteAuthTarget, 'utf8') !== siteAuthOutput)
    throw new Error(
      'Run node scripts/generate-pow-assets.mjs to sync the site authentication page.',
    );
} else {
  writeFileSync(siteAuthTarget, siteAuthOutput);
}
