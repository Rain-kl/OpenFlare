'use client';

import { useTranslations } from 'next-intl';

import { ResponsePageHtmlEditor } from '../../components/response-page-html-editor';
import { ResponsePageOption } from '../../components/response-page-option';
import {
  defaultSiteAuthHTML,
  previewSiteAuthHTML,
  SITE_AUTH_HTML_KEY,
  SITE_AUTH_HTML_MAX_BYTES,
} from '../../components/site-auth-page-html';

export default function SiteAuthPageEditPage() {
  const t = useTranslations('responses');

  return (
    <ResponsePageOption optionKey={SITE_AUTH_HTML_KEY}>
      {(html) => (
        <ResponsePageHtmlEditor
          optionKey={SITE_AUTH_HTML_KEY}
          savedHTML={html}
          defaultHTML={defaultSiteAuthHTML}
          maxBytes={SITE_AUTH_HTML_MAX_BYTES}
          title={t('siteAuthEdit')}
          backPath='/responses?tab=site-auth'
          backLabel={t('siteAuthBack')}
          help={t('siteAuthFormContract')}
          savedMessage={t('siteAuthSaved')}
          preview={previewSiteAuthHTML}
          previewTitle={t('siteAuthLivePreview')}
        />
      )}
    </ResponsePageOption>
  );
}
