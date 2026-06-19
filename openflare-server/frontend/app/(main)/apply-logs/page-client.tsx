"use client"

import {useCallback, useEffect, useMemo, useState} from "react"
import Link from "next/link"
import {useSearchParams} from "next/navigation"
import {ClipboardList, Eye, RefreshCw, Search} from "lucide-react"

import {EmptyStateWithBorder} from "@/components/layout/empty"
import {ErrorInline} from "@/components/layout/error"
import {LoadingStateWithBorder} from "@/components/layout/loading"
import {Badge} from "@/components/ui/badge"
import {Button} from "@/components/ui/button"
import {Input} from "@/components/ui/input"
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue,} from "@/components/ui/select"
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow,} from "@/components/ui/table"
import {type ApplyLogItem, ApplyLogService,} from "@/lib/services/openflare"
import {formatDateTime} from "@/lib/utils"

import {LogDetailSheet} from "./components/log-detail-sheet"

const PAGE_SIZE_OPTIONS = [20, 50, 100]

function truncateHash(value: string) {
  if (!value) return "—"
  return value.length > 12 ? `${value.slice(0, 12)}...` : value
}

function getResultBadge(result: string) {
  if (result === "success") {
    return (
      <Badge
        variant="outline"
        className="text-[10px] bg-emerald-500/10 border-emerald-500/20 text-emerald-600 rounded-full py-0 px-2"
      >
        <span className="size-1 bg-emerald-500 rounded-full mr-1.5 shrink-0" />
        成功
      </Badge>
    )
  }
  if (result === "warning") {
    return (
      <Badge
        variant="outline"
        className="text-[10px] bg-amber-500/10 border-amber-500/20 text-amber-600 rounded-full py-0 px-2"
      >
        <span className="size-1 bg-amber-500 rounded-full mr-1.5 shrink-0" />
        警告
      </Badge>
    )
  }
  return (
    <Badge
      variant="outline"
      className="text-[10px] bg-destructive/10 border-destructive/20 text-destructive rounded-full py-0 px-2"
    >
      <span className="size-1 bg-destructive rounded-full mr-1.5 shrink-0" />
      失败
    </Badge>
  )
}

export function ApplyLogsPageClient() {
  const searchParams = useSearchParams()
  const initialNodeId = searchParams.get("node_id")?.trim() ?? ""

  const [nodeFilterInput, setNodeFilterInput] = useState(initialNodeId)
  const [nodeFilter, setNodeFilter] = useState(initialNodeId)
  const [pageNo, setPageNo] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  const [rows, setRows] = useState<ApplyLogItem[]>([])
  const [current, setCurrent] = useState(1)
  const [total, setTotal] = useState(0)
  const [totalPage, setTotalPage] = useState(0)

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [selectedLog, setSelectedLog] = useState<ApplyLogItem | null>(null)
  const [detailOpen, setDetailOpen] = useState(false)

  const summary = useMemo(() => {
    const nodeIds = new Set(rows.map((item) => item.node_id))
    return [
      { label: "总记录数", value: total },
      { label: "当前页", value: current },
      { label: "总页数", value: totalPage },
      { label: "当前页节点数", value: nodeIds.size },
    ]
  }, [rows, total, current, totalPage])

  const fetchLogs = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await ApplyLogService.list({
        node_id: nodeFilter || undefined,
        pageNo,
        pageSize,
      })
      setRows(data.rows)
      setCurrent(data.current)
      setTotal(data.total)
      setTotalPage(data.totalPage)
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载失败")
    } finally {
      setLoading(false)
    }
  }, [nodeFilter, pageNo, pageSize])

  useEffect(() => {
    void fetchLogs()
  }, [fetchLogs])

  useEffect(() => {
    const nodeId = searchParams.get("node_id")?.trim()
    if (!nodeId) return
    setNodeFilterInput(nodeId)
    setNodeFilter(nodeId)
    setPageNo(1)
  }, [searchParams])

  const handleSearch = () => {
    setPageNo(1)
    setNodeFilter(nodeFilterInput.trim())
  }

  const handleReset = () => {
    setNodeFilterInput("")
    setNodeFilter("")
    setPageNo(1)
  }

  return (
    <div className="py-6 px-1 space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2">
          <ClipboardList className="size-5 text-primary" />
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">应用日志</h1>
            <p className="text-sm text-muted-foreground">
              查看节点应用配置的成功、警告和失败记录，支持按 node_id 过滤。
            </p>
          </div>
        </div>

        <div className="flex flex-wrap gap-2">
          <Button variant="outline" size="sm" onClick={() => void fetchLogs()} disabled={loading}>
            <RefreshCw className={`size-3.5 mr-1 ${loading ? "animate-spin" : ""}`} />
            刷新
          </Button>
          <Button variant="outline" size="sm" asChild>
            <Link href="/nodes">返回节点</Link>
          </Button>
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {summary.map((item) => (
          <div
            key={item.label}
            className="rounded-lg border border-dashed px-4 py-3 bg-background"
          >
            <p className="text-[10px] uppercase tracking-wider text-muted-foreground">
              {item.label}
            </p>
            <p className="mt-1 text-lg font-semibold">{item.value}</p>
          </div>
        ))}
      </div>

      <div className="flex flex-col gap-3 xl:flex-row xl:items-end xl:justify-between">
        <div className="grid flex-1 gap-3 md:grid-cols-[minmax(0,1fr)_160px]">
          <div className="space-y-1.5">
            <p className="text-xs font-medium text-muted-foreground">Node ID</p>
            <div className="relative">
              <Search className="absolute left-2.5 top-2.5 size-3.5 text-muted-foreground" />
              <Input
                value={nodeFilterInput}
                onChange={(e) => setNodeFilterInput(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") handleSearch()
                }}
                placeholder="输入 node_id 过滤应用日志"
                className="pl-8 h-9 text-xs"
              />
            </div>
          </div>
          <div className="space-y-1.5">
            <p className="text-xs font-medium text-muted-foreground">每页条数</p>
            <Select
              value={String(pageSize)}
              onValueChange={(value) => {
                setPageSize(Number.parseInt(value, 10))
                setPageNo(1)
              }}
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
        </div>
        <div className="flex flex-wrap gap-2">
          <Button size="sm" onClick={handleSearch}>
            筛选
          </Button>
          <Button variant="outline" size="sm" onClick={handleReset}>
            清空
          </Button>
        </div>
      </div>

      {error ? <ErrorInline message={error} onRetry={() => void fetchLogs()} /> : null}

      <div className="border border-dashed shadow-none rounded-lg overflow-hidden bg-background">
        {loading ? (
          <LoadingStateWithBorder />
        ) : rows.length === 0 ? (
          <EmptyStateWithBorder
            title="暂无应用日志"
            description="当前筛选条件下没有可展示的应用记录。"
          />
        ) : (
          <Table>
            <TableHeader className="bg-muted/40">
              <TableRow className="border-dashed hover:bg-transparent">
                <TableHead className="text-xs font-semibold">Node ID</TableHead>
                <TableHead className="text-xs font-semibold">版本</TableHead>
                <TableHead className="text-xs font-semibold">结果</TableHead>
                <TableHead className="text-xs font-semibold">Checksum</TableHead>
                <TableHead className="text-xs font-semibold">时间</TableHead>
                <TableHead className="text-xs font-semibold">消息</TableHead>
                <TableHead className="text-xs font-semibold text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((log) => (
                <TableRow
                  key={log.id}
                  className="border-dashed hover:bg-muted/10 transition-colors align-top"
                >
                  <TableCell className="text-xs font-medium">{log.node_id}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">{log.version}</TableCell>
                  <TableCell>{getResultBadge(log.result)}</TableCell>
                  <TableCell
                    className="text-xs font-mono text-muted-foreground"
                    title={log.checksum}
                  >
                    {truncateHash(log.checksum)}
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {formatDateTime(log.created_at)}
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground max-w-56">
                    <div className="line-clamp-2 break-words">{log.message || "—"}</div>
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-7 text-xs"
                      onClick={() => {
                        setSelectedLog(log)
                        setDetailOpen(true)
                      }}
                    >
                      <Eye className="size-3 mr-1" />
                      详情
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>

      {!loading && rows.length > 0 ? (
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-xs text-muted-foreground">
            第 {current} / {Math.max(totalPage, 1)} 页，共 {total} 条记录。
          </p>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={current <= 1}
              onClick={() => setPageNo((prev) => Math.max(1, prev - 1))}
            >
              上一页
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={totalPage === 0 || current >= totalPage}
              onClick={() =>
                setPageNo((prev) =>
                  totalPage > 0 ? Math.min(totalPage, prev + 1) : prev,
                )
              }
            >
              下一页
            </Button>
          </div>
        </div>
      ) : null}

      <LogDetailSheet
        log={selectedLog}
        open={detailOpen}
        onOpenChange={setDetailOpen}
      />
    </div>
  )
}
