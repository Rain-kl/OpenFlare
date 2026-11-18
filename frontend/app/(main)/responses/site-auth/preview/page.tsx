'use client';

import { useTranslations } from 'next-intl';

import { ResponsePageFullPreview } from '../../components/response-page-full-preview';

import {
  previewSiteAuthHTML,
  SITE_AUTH_HTML_KEY,
} from '../../components/site-auth-page-html';
import { ResponsePageOption } from '../../components/response-page-option';

export default function SiteAuthPagePreviewPage() {
  const t = useTranslations('responses');

  return (
    <ResponsePageOption optionKey={SITE_AUTH_HTML_KEY}>
      {(html) => (
        <ResponsePageFullPreview
          title={t('siteAuthPreview')}
          backPath='/responses?tab=site-auth'
          backLabel={t('siteAuthBack')}
          editPath='/responses/site-auth/edit'
          html={previewSiteAuthHTML(html)}
        />
      )}
    </ResponsePageOption>
  );
}
