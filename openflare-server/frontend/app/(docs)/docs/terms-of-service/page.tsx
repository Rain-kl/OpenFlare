import {LegalPageLayout} from "@/components/common/docs/legal-page-layout"
import {TERMS_LAST_UPDATED, termsSections} from "@/components/common/docs/terms"

export default function TermsOfServicePage() {
  return (
    <LegalPageLayout
      title="服务协议 (Terms)"
      lastUpdated={TERMS_LAST_UPDATED}
      sections={termsSections}
      description={
        <span>
          为了保障您的合法权益，请您仔细阅读以下条款。
        </span>
      }
    />
  )
}
