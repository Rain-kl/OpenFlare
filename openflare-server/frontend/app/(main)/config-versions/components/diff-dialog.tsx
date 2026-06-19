"use client"

import {Loader2} from "lucide-react"

import {Badge} from "@/components/ui/badge"
import {Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle,} from "@/components/ui/dialog"
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow,} from "@/components/ui/table"
import type {ConfigDiffResult} from "@/lib/services/openflare"

interface DiffDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  diff: ConfigDiffResult | null
  loading: boolean
  error: string | null
}

function DiffChipList({ title, items }: { title: string; items: string[] }) {
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-2">
        <p className="text-xs font-semibold">{title}</p>
        <Badge variant="outline" className="text-[10px] rounded-full">
          {items.length} 项
        </Badge>
      </div>
      {items.length > 0 ? (
        <div className="flex flex-wrap gap-1.5">
          {items.map((item) => (
            <Badge key={item} variant="secondary" className="text-[10px] font-normal">
              {item}
            </Badge>
          ))}
        </div>
      ) : (
        <p className="text-xs text-muted-foreground">无变更</p>
      )}
    </div>
  )
}

function renderOptionValue(value: string) {
  return value === "" ? "空" : value
}

export function DiffDialog({
  open,
  onOpenChange,
  diff,
  loading,
  error,
}: DiffDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-3xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>配置差异</DialogTitle>
          <DialogDescription>
            对比当前待发布配置与已激活版本之间的差异。
          </DialogDescription>
        </DialogHeader>

        {loading ? (
          <div className="flex items-center justify-center gap-2 py-12 text-sm text-muted-foreground">
            <Loader2 className="size-4 animate-spin" />
            正在加载差异数据...
          </div>
        ) : error ? (
          <p className="text-sm text-destructive">{error}</p>
        ) : diff ? (
          <div className="space-y-5">
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <div className="rounded-lg border border-dashed p-3">
                <p className="text-[10px] uppercase tracking-wider text-muted-foreground">激活版本</p>
                <p className="mt-1 text-sm font-medium">{diff.active_version || "无"}</p>
              </div>
              <div className="rounded-lg border border-dashed p-3">
                <p className="text-[10px] uppercase tracking-wider text-muted-foreground">新增域名</p>
                <p className="mt-1 text-lg font-semibold text-emerald-600">{diff.added_domains.length}</p>
              </div>
              <div className="rounded-lg border border-dashed p-3">
                <p className="text-[10px] uppercase tracking-wider text-muted-foreground">删除域名</p>
                <p className="mt-1 text-lg font-semibold text-amber-600">{diff.removed_domains.length}</p>
              </div>
              <div className="rounded-lg border border-dashed p-3">
                <p className="text-[10px] uppercase tracking-wider text-muted-foreground">修改域名</p>
                <p className="mt-1 text-lg font-semibold text-blue-600">{diff.modified_domains.length}</p>
              </div>
            </div>

            <div className="grid gap-4 md:grid-cols-3">
              <DiffChipList title="新增域名" items={diff.added_domains} />
              <DiffChipList title="删除域名" items={diff.removed_domains} />
              <DiffChipList title="修改域名" items={diff.modified_domains} />
            </div>

            <div className="flex flex-wrap gap-2">
              <Badge variant={diff.main_config_changed ? "destructive" : "outline"}>
                主配置 {diff.main_config_changed ? "已变化" : "无变化"}
              </Badge>
              <Badge variant={diff.waf_config_changed ? "destructive" : "outline"}>
                WAF {diff.waf_config_changed ? "已变化" : "无变化"}
              </Badge>
              <Badge variant="outline">
                网站数 {diff.active_website_count} → {diff.current_website_count}
              </Badge>
            </div>

            {diff.changed_option_details.length > 0 ? (
              <div className="border border-dashed rounded-lg overflow-hidden">
                <Table>
                  <TableHeader className="bg-muted/40">
                    <TableRow className="border-dashed hover:bg-transparent">
                      <TableHead className="text-xs font-semibold">参数</TableHead>
                      <TableHead className="text-xs font-semibold">激活值</TableHead>
                      <TableHead className="text-xs font-semibold">待发布值</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {diff.changed_option_details.map((item) => (
                      <TableRow key={item.key} className="border-dashed">
                        <TableCell className="text-xs font-medium">{item.key}</TableCell>
                        <TableCell className="text-xs font-mono text-muted-foreground">
                          {renderOptionValue(item.previous_value)}
                        </TableCell>
                        <TableCell className="text-xs font-mono text-muted-foreground">
                          {renderOptionValue(item.current_value)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            ) : (
              <p className="text-xs text-muted-foreground">当前无 OpenResty 参数变化。</p>
            )}
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  )
}
