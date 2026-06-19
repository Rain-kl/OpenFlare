"use client"

import {Search} from "lucide-react"

import {Button} from "@/components/ui/button"
import {Input} from "@/components/ui/input"
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue,} from "@/components/ui/select"
import type {AccessLogTab, SearchDraft} from "./access-log-utils"
import {PAGE_SIZE_OPTIONS} from "./access-log-utils"

interface AccessLogFiltersProps {
  tab: AccessLogTab
  draft: SearchDraft
  pageSize: number
  onDraftChange: (draft: SearchDraft) => void
  onPageSizeChange: (pageSize: number) => void
  onSearch: () => void
  onReset: () => void
}

export function AccessLogFilters({
  tab,
  draft,
  pageSize,
  onDraftChange,
  onPageSizeChange,
  onSearch,
  onReset,
}: AccessLogFiltersProps) {
  const showPath = tab === "list" || tab === "folds"

  return (
    <div className="space-y-3">
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <div className="space-y-1.5">
          <p className="text-xs font-medium text-muted-foreground">节点 ID</p>
          <div className="relative">
            <Search className="absolute left-2.5 top-2.5 size-3.5 text-muted-foreground" />
            <Input
              value={draft.nodeId}
              onChange={(e) =>
                onDraftChange({ ...draft, nodeId: e.target.value })
              }
              onKeyDown={(e) => {
                if (e.key === "Enter") onSearch()
              }}
              placeholder="按 node_id 搜索"
              className="pl-8 h-9 text-xs"
            />
          </div>
        </div>
        <div className="space-y-1.5">
          <p className="text-xs font-medium text-muted-foreground">来源 IP</p>
          <Input
            value={draft.remoteAddr}
            onChange={(e) =>
              onDraftChange({ ...draft, remoteAddr: e.target.value })
            }
            onKeyDown={(e) => {
              if (e.key === "Enter") onSearch()
            }}
            placeholder="按 IP 搜索"
            className="h-9 text-xs"
          />
        </div>
        <div className="space-y-1.5">
          <p className="text-xs font-medium text-muted-foreground">访问域名</p>
          <Input
            value={draft.host}
            onChange={(e) => onDraftChange({ ...draft, host: e.target.value })}
            onKeyDown={(e) => {
              if (e.key === "Enter") onSearch()
            }}
            placeholder="按域名搜索"
            className="h-9 text-xs"
          />
        </div>
        {showPath ? (
          <div className="space-y-1.5">
            <p className="text-xs font-medium text-muted-foreground">请求路径</p>
            <Input
              value={draft.path}
              onChange={(e) => onDraftChange({ ...draft, path: e.target.value })}
              onKeyDown={(e) => {
                if (e.key === "Enter") onSearch()
              }}
              placeholder="按路径搜索"
              className="h-9 text-xs"
            />
          </div>
        ) : null}
      </div>

      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div className="space-y-1.5 w-full sm:max-w-[180px]">
          <p className="text-xs font-medium text-muted-foreground">每页条数</p>
          <Select
            value={String(pageSize)}
            onValueChange={(value) => onPageSizeChange(Number.parseInt(value, 10))}
          >
            <SelectTrigger className="h-9 text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {PAGE_SIZE_OPTIONS.map((option) => (
                <SelectItem key={option} value={String(option)}>
                  {option} 条
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex gap-2">
          <Button size="sm" onClick={onSearch}>
            筛选
          </Button>
          <Button variant="outline" size="sm" onClick={onReset}>
            清空
          </Button>
        </div>
      </div>
    </div>
  )
}
