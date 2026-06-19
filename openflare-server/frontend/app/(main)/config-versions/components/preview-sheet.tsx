"use client"

import {Loader2} from "lucide-react"

import {Badge} from "@/components/ui/badge"
import {Button} from "@/components/ui/button"
import {CodeBlock} from "@/components/ui/code-block"
import {ScrollArea} from "@/components/ui/scroll-area"
import {Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle,} from "@/components/ui/sheet"
import type {ConfigPreviewResult} from "@/lib/services/openflare"

interface PreviewSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  preview: ConfigPreviewResult | null
  loading: boolean
  error: string | null
  publishing: boolean
  canPublish: boolean
  onPublish: () => void
}

export function PreviewSheet({
  open,
  onOpenChange,
  preview,
  loading,
  error,
  publishing,
  canPublish,
  onPublish,
}: PreviewSheetProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="sm:max-w-2xl w-full p-0 flex flex-col gap-0">
        <SheetHeader className="px-5 py-4 border-b">
          <SheetTitle>发布预览</SheetTitle>
          <SheetDescription>
            查看待发布配置的渲染结果与支持文件。
          </SheetDescription>
        </SheetHeader>

        <ScrollArea className="flex-1 px-5 py-4">
          {loading ? (
            <div className="flex items-center justify-center gap-2 py-16 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" />
              正在加载预览...
            </div>
          ) : error ? (
            <p className="text-sm text-destructive">{error}</p>
          ) : preview ? (
            <div className="space-y-5 pb-4">
              <div className="flex flex-wrap gap-2">
                <Badge variant="outline" className="rounded-full text-[10px]">
                  规则 {preview.route_count} 条
                </Badge>
                <Badge variant="outline" className="rounded-full text-[10px]">
                  网站 {preview.website_count} 个
                </Badge>
                <Badge variant="outline" className="rounded-full text-[10px] font-mono">
                  {preview.checksum.slice(0, 16)}...
                </Badge>
              </div>

              <div className="space-y-2">
                <p className="text-xs font-semibold">主配置</p>
                <CodeBlock code={preview.main_config} language="nginx" className="my-0 text-xs" />
              </div>

              <div className="space-y-2">
                <p className="text-xs font-semibold">路由配置</p>
                <CodeBlock code={preview.rendered_config} language="nginx" className="my-0 text-xs" />
              </div>

              {preview.support_files.length > 0 ? (
                <div className="space-y-2">
                  <p className="text-xs font-semibold">支持文件 ({preview.support_files.length})</p>
                  {preview.support_files.map((file) => (
                    <details
                      key={file.path}
                      className="rounded-lg border border-dashed px-3 py-2"
                    >
                      <summary className="cursor-pointer text-xs font-medium">{file.path}</summary>
                      <CodeBlock code={file.content} language="text" className="my-2 text-xs" />
                    </details>
                  ))}
                </div>
              ) : (
                <p className="text-xs text-muted-foreground">当前发布不需要额外支持文件。</p>
              )}

              {!canPublish ? (
                <p className="text-xs text-muted-foreground">
                  当前规则与已激活版本一致，无法重复发布。
                </p>
              ) : null}
            </div>
          ) : null}
        </ScrollArea>

        <SheetFooter className="px-5 py-4 border-t">
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={publishing}>
            关闭
          </Button>
          <Button onClick={onPublish} disabled={!canPublish || publishing || loading}>
            {publishing ? (
              <>
                <Loader2 className="size-4 animate-spin mr-1" />
                发布中...
              </>
            ) : (
              "确认发布"
            )}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
