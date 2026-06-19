"use client"

import {useCallback, useEffect, useMemo, useState} from "react"
import {useMutation, useQuery, useQueryClient} from "@tanstack/react-query"
import {Area, AreaChart, CartesianGrid, XAxis, YAxis} from "recharts"
import {RefreshCw, ScrollText, Trash2} from "lucide-react"
import {toast} from "sonner"

import {EmptyStateWithBorder} from "@/components/layout/empty"
import {ErrorInline} from "@/components/layout/error"
import {LoadingStateWithBorder} from "@/components/layout/loading"
import {Badge} from "@/components/ui/badge"
import {Button} from "@/components/ui/button"
import {ChartConfig, ChartContainer, ChartTooltip, ChartTooltipContent,} from "@/components/ui/chart"
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue,} from "@/components/ui/select"
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow,} from "@/components/ui/table"
import {Tabs, TabsContent, TabsList, TabsTrigger} from "@/components/ui/tabs"
import {AccessLogService} from "@/lib/services/openflare"
import {formatDateTime} from "@/lib/utils"

import {AccessLogFilters} from "./components/access-log-filters"
import {
  type AccessLogTab,
  DETAIL_SORT_OPTIONS,
  FOLD_SORT_OPTIONS,
  formatCompactNumber,
  IP_SORT_OPTIONS,
  parseSortValue,
  type SearchDraft,
} from "./components/access-log-utils"
import {CleanupDialog} from "./components/cleanup-dialog"

const trendChartConfig = {
  requests: { label: "请求数", color: "hsl(var(--primary))" },
} satisfies ChartConfig

const emptyDraft: SearchDraft = {
  nodeId: "",
  remoteAddr: "",
  host: "",
  path: "",
}

function PaginationBar({
  page,
  hasMore,
  loading,
  onPrev,
  onNext,
}: {
  page: number
  hasMore: boolean
  loading: boolean
  onPrev: () => void
  onNext: () => void
}) {
  return (
    <div className="flex items-center justify-between px-4 py-3 border-t border-dashed">
      <p className="text-xs text-muted-foreground">当前第 {page + 1} 页</p>
      <div className="flex gap-2">
        <Button variant="outline" size="sm" disabled={loading || page <= 0} onClick={onPrev}>
          上一页
        </Button>
        <Button variant="outline" size="sm" disabled={loading || !hasMore} onClick={onNext}>
          下一页
        </Button>
      </div>
    </div>
  )
}

export default function AccessLogsPage() {
  const queryClient = useQueryClient()
  const [tab, setTab] = useState<AccessLogTab>("list")
  const [draft, setDraft] = useState<SearchDraft>(emptyDraft)
  const [filters, setFilters] = useState<SearchDraft>(emptyDraft)
  const [pageSize, setPageSize] = useState(20)
  const [page, setPage] = useState(0)
  const [detailSort, setDetailSort] = useState("logged_at:desc")
  const [foldSort, setFoldSort] = useState("bucket_started_at:desc")
  const [ipSort, setIpSort] = useState("total_requests:desc")
  const [foldMinutes, setFoldMinutes] = useState<3 | 5>(3)
  const [trendIp, setTrendIp] = useState("")
  const [appliedTrendIp, setAppliedTrendIp] = useState("")
  const [cleanupOpen, setCleanupOpen] = useState(false)

  const detailSortState = parseSortValue(detailSort)
  const foldSortState = parseSortValue(foldSort)
  const ipSortState = parseSortValue(ipSort)

  const listQuery = useQuery({
    queryKey: ["openflare", "access-logs", "list", filters, page, pageSize, detailSort],
    queryFn: () =>
      AccessLogService.list({
        node_id: filters.nodeId || undefined,
        remote_addr: filters.remoteAddr || undefined,
        host: filters.host || undefined,
        path: filters.path || undefined,
        p: page,
        page_size: pageSize,
        sort_by: detailSortState.sortBy,
        sort_order: detailSortState.sortOrder,
      }),
    enabled: tab === "list",
  })

  const foldsQuery = useQuery({
    queryKey: ["openflare", "access-logs", "folds", filters, page, pageSize, foldSort, foldMinutes],
    queryFn: () =>
      AccessLogService.listFolds({
        node_id: filters.nodeId || undefined,
        remote_addr: filters.remoteAddr || undefined,
        host: filters.host || undefined,
        path: filters.path || undefined,
        p: page,
        page_size: pageSize,
        sort_by: foldSortState.sortBy,
        sort_order: foldSortState.sortOrder,
        fold_minutes: foldMinutes,
      }),
    enabled: tab === "folds",
  })

  const ipSummaryQuery = useQuery({
    queryKey: ["openflare", "access-logs", "ip-summary", filters, page, pageSize, ipSort],
    queryFn: () =>
      AccessLogService.listIPSummaries({
        node_id: filters.nodeId || undefined,
        remote_addr: filters.remoteAddr || undefined,
        host: filters.host || undefined,
        p: page,
        page_size: pageSize,
        sort_by: ipSortState.sortBy,
        sort_order: ipSortState.sortOrder,
      }),
    enabled: tab === "ip-summary",
  })

  const ipTrendQuery = useQuery({
    queryKey: ["openflare", "access-logs", "ip-trend", filters, appliedTrendIp],
    queryFn: () =>
      AccessLogService.getIPTrend({
        node_id: filters.nodeId || undefined,
        remote_addr: appliedTrendIp,
        host: filters.host || undefined,
        hours: 24,
        bucket_minutes: 30,
      }),
    enabled: tab === "ip-trend" && appliedTrendIp !== "",
  })

  const cleanupMutation = useMutation({
    mutationFn: (retentionDays: number) =>
      AccessLogService.cleanup({ retention_days: retentionDays }),
    onSuccess: async (result) => {
      toast.success(`已清理 ${result.deleted_count} 条日志`)
      setCleanupOpen(false)
      await queryClient.invalidateQueries({ queryKey: ["openflare", "access-logs"] })
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : "清理失败")
    },
  })

  const activeSummary = useMemo(() => {
    if (tab === "list" && listQuery.data) {
      return {
        totalRecord: listQuery.data.total_record,
        totalIp: listQuery.data.total_ip,
      }
    }
    if (tab === "folds" && foldsQuery.data) {
      return {
        totalRecord: foldsQuery.data.total_record,
        totalIp: foldsQuery.data.total_ip,
      }
    }
    if (tab === "ip-summary" && ipSummaryQuery.data) {
      return {
        totalRecord: 0,
        totalIp: ipSummaryQuery.data.total_ip,
      }
    }
    return { totalRecord: 0, totalIp: 0 }
  }, [tab, listQuery.data, foldsQuery.data, ipSummaryQuery.data])

  const trendChartData = useMemo(() => {
    return (ipTrendQuery.data?.points ?? []).map((point) => ({
      label: formatDateTime(point.bucket_started_at).slice(5),
      requests: point.request_count,
    }))
  }, [ipTrendQuery.data?.points])

  const handleSearch = useCallback(() => {
    setFilters({
      nodeId: draft.nodeId.trim(),
      remoteAddr: draft.remoteAddr.trim(),
      host: draft.host.trim(),
      path: draft.path.trim(),
    })
    setPage(0)
  }, [draft])

  const handleReset = () => {
    setDraft(emptyDraft)
    setFilters(emptyDraft)
    setPage(0)
    setTrendIp("")
    setAppliedTrendIp("")
  }

  useEffect(() => {
    setPage(0)
  }, [tab, pageSize])

  const refreshActive = () => {
    if (tab === "list") void listQuery.refetch()
    if (tab === "folds") void foldsQuery.refetch()
    if (tab === "ip-summary") void ipSummaryQuery.refetch()
    if (tab === "ip-trend") void ipTrendQuery.refetch()
  }

  const isFetching =
    listQuery.isFetching ||
    foldsQuery.isFetching ||
    ipSummaryQuery.isFetching ||
    ipTrendQuery.isFetching

  return (
    <div className="py-6 px-1 space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2">
          <ScrollText className="size-5 text-primary" />
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">访问日志</h1>
            <p className="text-sm text-muted-foreground">
              按节点、IP、域名与路径检索，支持时间折叠、IP 汇总与趋势分析。
            </p>
          </div>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={refreshActive} disabled={isFetching}>
            <RefreshCw className={`size-3.5 mr-1 ${isFetching ? "animate-spin" : ""}`} />
            刷新
          </Button>
          <Button variant="destructive" size="sm" onClick={() => setCleanupOpen(true)}>
            <Trash2 className="size-3.5 mr-1" />
            清理日志
          </Button>
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-3">
        {[
          { label: "访问记录", value: formatCompactNumber(activeSummary.totalRecord) },
          { label: "来源 IP", value: formatCompactNumber(activeSummary.totalIp) },
          {
            label: "当前视图",
            value:
              tab === "list"
                ? "明细日志"
                : tab === "folds"
                  ? "时间折叠"
                  : tab === "ip-summary"
                    ? "IP 汇总"
                    : "IP 趋势",
          },
        ].map((item) => (
          <div key={item.label} className="rounded-lg border border-dashed px-4 py-3">
            <p className="text-[10px] uppercase tracking-wider text-muted-foreground">
              {item.label}
            </p>
            <p className="mt-1 text-lg font-semibold">{item.value}</p>
          </div>
        ))}
      </div>

      <Tabs value={tab} onValueChange={(value) => setTab(value as AccessLogTab)}>
        <TabsList className="grid w-full grid-cols-2 lg:grid-cols-4">
          <TabsTrigger value="list">明细列表</TabsTrigger>
          <TabsTrigger value="folds">时间折叠</TabsTrigger>
          <TabsTrigger value="ip-summary">IP 汇总</TabsTrigger>
          <TabsTrigger value="ip-trend">IP 趋势</TabsTrigger>
        </TabsList>

        <div className="mt-4 rounded-lg border border-dashed bg-background p-4">
          <AccessLogFilters
            tab={tab}
            draft={draft}
            pageSize={pageSize}
            onDraftChange={setDraft}
            onPageSizeChange={setPageSize}
            onSearch={handleSearch}
            onReset={handleReset}
          />
        </div>

        <TabsContent value="list" className="mt-4">
          <div className="rounded-lg border border-dashed overflow-hidden bg-background">
            <div className="flex items-center justify-between px-4 py-3 border-b border-dashed">
              <p className="text-sm font-medium">明细日志</p>
              <Select value={detailSort} onValueChange={setDetailSort}>
                <SelectTrigger className="h-8 w-44 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {DETAIL_SORT_OPTIONS.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {listQuery.isError ? (
              <div className="p-4">
                <ErrorInline
                  message={listQuery.error instanceof Error ? listQuery.error.message : "加载失败"}
                  onRetry={() => void listQuery.refetch()}
                />
              </div>
            ) : listQuery.isLoading ? (
              <LoadingStateWithBorder />
            ) : (listQuery.data?.items ?? []).length === 0 ? (
              <EmptyStateWithBorder title="暂无访问日志" />
            ) : (
              <Table>
                <TableHeader className="bg-muted/40">
                  <TableRow className="border-dashed hover:bg-transparent">
                    <TableHead className="text-xs">时间</TableHead>
                    <TableHead className="text-xs">节点</TableHead>
                    <TableHead className="text-xs">IP</TableHead>
                    <TableHead className="text-xs">域名</TableHead>
                    <TableHead className="text-xs">路径</TableHead>
                    <TableHead className="text-xs">状态码</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(listQuery.data?.items ?? []).map((item) => (
                    <TableRow key={item.id} className="border-dashed">
                      <TableCell className="text-xs">{formatDateTime(item.logged_at)}</TableCell>
                      <TableCell className="text-xs">{item.node_name || item.node_id}</TableCell>
                      <TableCell className="text-xs font-mono">{item.remote_addr}</TableCell>
                      <TableCell className="text-xs">{item.host}</TableCell>
                      <TableCell className="text-xs max-w-48 truncate">{item.path}</TableCell>
                      <TableCell>
                        <Badge variant="outline" className="text-[10px]">
                          {item.status_code}
                        </Badge>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
            <PaginationBar
              page={page}
              hasMore={listQuery.data?.has_more ?? false}
              loading={listQuery.isFetching}
              onPrev={() => setPage((p) => Math.max(0, p - 1))}
              onNext={() => setPage((p) => p + 1)}
            />
          </div>
        </TabsContent>

        <TabsContent value="folds" className="mt-4">
          <div className="rounded-lg border border-dashed overflow-hidden bg-background">
            <div className="flex flex-wrap items-center justify-between gap-2 px-4 py-3 border-b border-dashed">
              <div className="flex items-center gap-2">
                <p className="text-sm font-medium">时间折叠</p>
                <Select
                  value={String(foldMinutes)}
                  onValueChange={(value) => setFoldMinutes(Number(value) as 3 | 5)}
                >
                  <SelectTrigger className="h-8 w-36 text-xs">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="3">3 分钟桶</SelectItem>
                    <SelectItem value="5">5 分钟桶</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <Select value={foldSort} onValueChange={setFoldSort}>
                <SelectTrigger className="h-8 w-44 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {FOLD_SORT_OPTIONS.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {foldsQuery.isError ? (
              <div className="p-4">
                <ErrorInline
                  message={
                    foldsQuery.error instanceof Error
                      ? foldsQuery.error.message
                      : "加载失败"
                  }
                  onRetry={() => void foldsQuery.refetch()}
                />
              </div>
            ) : foldsQuery.isLoading ? (
              <LoadingStateWithBorder />
            ) : (foldsQuery.data?.items ?? []).length === 0 ? (
              <EmptyStateWithBorder title="暂无折叠数据" />
            ) : (
              <Table>
                <TableHeader className="bg-muted/40">
                  <TableRow className="border-dashed hover:bg-transparent">
                    <TableHead className="text-xs">时间桶</TableHead>
                    <TableHead className="text-xs">请求数</TableHead>
                    <TableHead className="text-xs">独立 IP</TableHead>
                    <TableHead className="text-xs">独立域名</TableHead>
                    <TableHead className="text-xs">2xx</TableHead>
                    <TableHead className="text-xs">4xx</TableHead>
                    <TableHead className="text-xs">5xx</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(foldsQuery.data?.items ?? []).map((item) => (
                    <TableRow key={item.bucket_started_at} className="border-dashed">
                      <TableCell className="text-xs">
                        {formatDateTime(item.bucket_started_at)}
                      </TableCell>
                      <TableCell className="text-xs">{item.request_count}</TableCell>
                      <TableCell className="text-xs">{item.unique_ip_count}</TableCell>
                      <TableCell className="text-xs">{item.unique_host_count}</TableCell>
                      <TableCell className="text-xs">{item.success_count}</TableCell>
                      <TableCell className="text-xs">{item.client_error_count}</TableCell>
                      <TableCell className="text-xs">{item.server_error_count}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
            <PaginationBar
              page={page}
              hasMore={foldsQuery.data?.has_more ?? false}
              loading={foldsQuery.isFetching}
              onPrev={() => setPage((p) => Math.max(0, p - 1))}
              onNext={() => setPage((p) => p + 1)}
            />
          </div>
        </TabsContent>

        <TabsContent value="ip-summary" className="mt-4">
          <div className="rounded-lg border border-dashed overflow-hidden bg-background">
            <div className="flex items-center justify-between px-4 py-3 border-b border-dashed">
              <p className="text-sm font-medium">IP 汇总</p>
              <Select value={ipSort} onValueChange={setIpSort}>
                <SelectTrigger className="h-8 w-52 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {IP_SORT_OPTIONS.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {ipSummaryQuery.isError ? (
              <div className="p-4">
                <ErrorInline
                  message={
                    ipSummaryQuery.error instanceof Error
                      ? ipSummaryQuery.error.message
                      : "加载失败"
                  }
                  onRetry={() => void ipSummaryQuery.refetch()}
                />
              </div>
            ) : ipSummaryQuery.isLoading ? (
              <LoadingStateWithBorder />
            ) : (ipSummaryQuery.data?.items ?? []).length === 0 ? (
              <EmptyStateWithBorder title="暂无 IP 汇总数据" />
            ) : (
              <Table>
                <TableHeader className="bg-muted/40">
                  <TableRow className="border-dashed hover:bg-transparent">
                    <TableHead className="text-xs">IP</TableHead>
                    <TableHead className="text-xs">总请求数</TableHead>
                    <TableHead className="text-xs">近 3 小时</TableHead>
                    <TableHead className="text-xs">最后访问</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(ipSummaryQuery.data?.items ?? []).map((item) => (
                    <TableRow key={item.remote_addr} className="border-dashed">
                      <TableCell className="text-xs font-mono">{item.remote_addr}</TableCell>
                      <TableCell className="text-xs">{item.total_requests}</TableCell>
                      <TableCell className="text-xs">{item.recent_requests}</TableCell>
                      <TableCell className="text-xs">
                        {formatDateTime(item.last_seen_at)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
            <PaginationBar
              page={page}
              hasMore={ipSummaryQuery.data?.has_more ?? false}
              loading={ipSummaryQuery.isFetching}
              onPrev={() => setPage((p) => Math.max(0, p - 1))}
              onNext={() => setPage((p) => p + 1)}
            />
          </div>
        </TabsContent>

        <TabsContent value="ip-trend" className="mt-4 space-y-4">
          <div className="rounded-lg border border-dashed bg-background p-4 flex flex-col gap-3 sm:flex-row sm:items-end">
            <div className="space-y-1.5 flex-1">
              <p className="text-xs font-medium text-muted-foreground">趋势 IP</p>
              <input
                className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 text-xs"
                value={trendIp}
                onChange={(e) => setTrendIp(e.target.value)}
                placeholder="输入要分析趋势的 IP 地址"
              />
            </div>
            <Button
              size="sm"
              onClick={() => setAppliedTrendIp(trendIp.trim())}
              disabled={!trendIp.trim()}
            >
              查看趋势
            </Button>
          </div>

          <div className="rounded-lg border border-dashed overflow-hidden bg-background p-4">
            {!appliedTrendIp ? (
              <EmptyStateWithBorder description="请输入 IP 地址后查看 24 小时访问趋势。" />
            ) : ipTrendQuery.isLoading ? (
              <LoadingStateWithBorder />
            ) : ipTrendQuery.isError ? (
              <ErrorInline
                message={
                  ipTrendQuery.error instanceof Error
                    ? ipTrendQuery.error.message
                    : "加载失败"
                }
                onRetry={() => void ipTrendQuery.refetch()}
              />
            ) : trendChartData.length === 0 ? (
              <EmptyStateWithBorder description="该 IP 在选定时间范围内没有访问记录。" />
            ) : (
              <div className="space-y-3">
                <p className="text-sm font-medium">
                  {appliedTrendIp} · 近 {ipTrendQuery.data?.hours ?? 24} 小时
                </p>
                <ChartContainer config={trendChartConfig} className="h-64 w-full">
                  <AreaChart data={trendChartData}>
                    <CartesianGrid vertical={false} />
                    <XAxis dataKey="label" tickLine={false} axisLine={false} fontSize={10} />
                    <YAxis tickLine={false} axisLine={false} fontSize={10} width={40} />
                    <ChartTooltip content={<ChartTooltipContent />} />
                    <Area
                      type="monotone"
                      dataKey="requests"
                      stroke="var(--color-requests)"
                      fill="var(--color-requests)"
                      fillOpacity={0.2}
                    />
                  </AreaChart>
                </ChartContainer>
              </div>
            )}
          </div>
        </TabsContent>
      </Tabs>

      <CleanupDialog
        open={cleanupOpen}
        onOpenChange={setCleanupOpen}
        onConfirm={(days) => cleanupMutation.mutate(days)}
        loading={cleanupMutation.isPending}
      />
    </div>
  )
}
