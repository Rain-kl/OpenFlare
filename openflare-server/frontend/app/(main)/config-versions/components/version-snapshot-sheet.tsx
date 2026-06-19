"use client"

import {useEffect, useState} from "react"
import {Loader2} from "lucide-react"
import {toast} from "sonner"

import {Badge} from "@/components/ui/badge"
import {Button} from "@/components/ui/button"
import {CodeBlock} from "@/components/ui/code-block"
import {ScrollArea} from "@/components/ui/scroll-area"
import {Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle,} from "@/components/ui/sheet"
import {type ConfigVersionDetail, ConfigVersionService, type ConfigVersionSummary,} from "@/lib/services/openflare"
import {formatDateTime} from "@/lib/utils"

interface VersionSnapshotSheetProps {
  version: ConfigVersionSummary | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function VersionSnapshotSheet({
  version,
  open,
  onOpenChange,
}: VersionSnapshotSheetProps) {
  const [detail, setDetail] = useState<ConfigVersionDetail | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!open || !version) {
      setDetail(null)
      return
    }

    let cancelled = false
    setLoading(true)

    ConfigVersionService.getById(version.id)
      .then((data) => {
        if (!cancelled) setDetail(data)
      })
      .catch((err) => {
        if (!cancelled) {
          toast.error("加载版本快照失败", {
            description: err instanceof Error ? err.message : "未知错误",
          })
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [open, version])

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="sm:max-w-2xl w-full p-0 flex flex-col gap-0">
        <SheetHeader className="px-5 py-4 border-b">
          <SheetTitle>
            {version ? `版本 ${version.version}` : "版本快照"}
          </SheetTitle>
          <SheetDescription>查看历史配置版本的完整快照内容。</SheetDescription>
        </SheetHeader>

        <ScrollArea className="flex-1 px-5 py-4">
          {loading ? (
            <div className="flex items-center justify-center gap-2 py-16 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" />
              正在加载快照...
            </div>
          ) : detail ? (
            <div className="space-y-5 pb-4">
              <div className="flex flex-wrap gap-2">
                {detail.is_active ? (
                  <Badge className="rounded-full text-[10px]">当前激活</Badge>
                ) : (
                  <Badge variant="outline" className="rounded-full text-[10px]">历史版本</Badge>
                )}
                <Badge variant="outline" className="rounded-full text-[10px] font-mono">
                  {detail.checksum}
                </Badge>
              </div>

              <div className="grid gap-3 sm:grid-cols-2 text-xs">
                <div>
                  <p className="text-muted-foreground">创建人</p>
                  <p className="font-medium">{detail.created_by || "系统"}</p>
                </div>
                <div>
                  <p className="text-muted-foreground">创建时间</p>
                  <p className="font-medium">{formatDateTime(detail.created_at)}</p>
                </div>
              </div>

              <div className="space-y-2">
                <p className="text-xs font-semibold">快照 JSON</p>
                <CodeBlock code={detail.snapshot_json} language="json" className="my-0 text-xs" />
              </div>

              <div className="space-y-2">
                <p className="text-xs font-semibold">主配置</p>
                <CodeBlock code={detail.main_config} language="nginx" className="my-0 text-xs" />
              </div>

              <div className="space-y-2">
                <p className="text-xs font-semibold">路由配置</p>
                <CodeBlock code={detail.rendered_config} language="nginx" className="my-0 text-xs" />
              </div>
            </div>
          ) : null}
        </ScrollArea>

        <SheetFooter className="px-5 py-4 border-t">
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            关闭
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
