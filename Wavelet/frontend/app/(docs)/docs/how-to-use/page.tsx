import {LegalPageLayout} from "@/components/common/docs/legal-page-layout";
import {howToUseSections} from "@/components/common/docs/how-to-use";
import {DOCS_LAST_UPDATED} from "@/components/common/docs/api";

export default function HowToUsePage() {
  return (
    <LegalPageLayout
      title="平台使用指南"
      lastUpdated={DOCS_LAST_UPDATED}
      sections={howToUseSections}
      description={
        <p className="text-muted-foreground text-sm leading-relaxed">
          欢迎使用 OpenFlare 管理平台。本文档将引导您快速了解平台的核心组件、角色权限设计及二次开发流程，帮助您快速上手开发。
        </p>
      }
    />
  );
}
