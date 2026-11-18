'use client';

import { useTranslations } from 'next-intl';

import {
  POW_HTML_MAX_BYTES,
  POW_PAGE_KEY,
  powTemplates,
  readPoWPage,
} from '@/lib/openflare/pow-page';

import { ResponsePageHtmlEditor } from '../../components/response-page-html-editor';
import { ResponsePageOption } from '../../components/response-page-option';
import { usePoWPagePreview } from '../../components/use-pow-page-preview';

export default function PoWPageEditPage() {
  const t = useTranslations('pow');
  const preview = usePoWPagePreview();

  return (
    <ResponsePageOption optionKey={POW_PAGE_KEY}>
      {(value) => {
        const config = readPoWPage(value);
        return (
          <ResponsePageHtmlEditor
            optionKey={POW_PAGE_KEY}
            savedHTML={config.html ?? ''}
            template={{
              defaultId: config.template,
              options: (['shield', 'classic'] as const).map((id) => ({
                id,
                label: t(id),
                html: powTemplates[id],
              })),
            }}
            maxBytes={POW_HTML_MAX_BYTES}
            title={t('editPage')}
            backPath='/responses?tab=pow'
            backLabel={t('backToPage')}
            help={t('customHelp', { widget: '{{widget}}', host: '{{host}}' })}
            savedMessage={t('pageSaved')}
            preview={(html) => preview({ ...config, html })}
            previewTitle={t('pagePreview')}
            previewSandbox='allow-scripts'
            previewHint={t('previewHelp')}
            mergeHTML={(latest, html) =>
              JSON.stringify({ ...readPoWPage(latest), html })
            }
          />
        );
      }}
    </ResponsePageOption>
  );
}
