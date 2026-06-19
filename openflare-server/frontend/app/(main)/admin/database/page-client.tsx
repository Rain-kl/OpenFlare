"use client"

import dynamic from "next/dynamic"
import * as React from "react"
import {useCallback, useEffect, useState} from "react"
import {toast} from "sonner"
import {
  Activity,
  Cpu,
  Database,
  Download,
  FileText,
  HardDrive,
  Layers,
  RefreshCw,
  Server,
  Terminal,
} from "lucide-react"

import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card"
import {Button} from "@/components/ui/button"
import {Skeleton} from "@/components/ui/skeleton"
import services from "@/lib/services"
import type {DBOverview} from "@/lib/services/db-manage"

const sectionFallback = (
  <div className="h-48 animate-pulse rounded-lg border border-border/40 bg-muted/20" />
)

const TableBrowser = dynamic(
  () => import("./components/table-browser").then((mod) => mod.TableBrowser),
  { loading: () => sectionFallback },
)

const CacheManager = dynamic(
  () => import("./components/cache-manager").then((mod) => mod.CacheManager),
  { loading: () => sectionFallback },
)

const SQLConsole = dynamic(
  () => import("./components/sql-console").then((mod) => mod.SQLConsole),
  { ssr: false },
)

/**
 * 格式化数字，每3位加逗号
 */
const formatNumber = (num: number | string) => {
  if (num === undefined || num === null) return "0"
  return num.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",")
}

export function DatabasePageClient() {
  // 核心状态
  const [overview, setOverview] = useState<DBOverview | null>(null)
  const [tables, setTables] = useState<string[]>([])
  const [loadingOverview, setLoadingOverview] = useState<boolean>(true)
  const [loadingTables, setLoadingTables] = useState<boolean>(true)
  const [exporting, setExporting] = useState<boolean>(false)

  // 协调子组件刷新与视图切换状态
  const [showConsole, setShowConsole] = useState<boolean>(false)
  const [refreshTrigger, setRefreshTrigger] = useState<number>(0)

  // 1. 获取运行概览
  const fetchOverview = useCallback(async (isSilent = false) => {
    if (!isSilent) setLoadingOverview(true)
    try {
      const data = await services.dbManage.getOverview()
      setOverview(data)
    } catch (err) {
      toast.error("获取数据库概览失败", {
        description: err instanceof Error ? err.message : "未知错误",
      })
    } finally {
      setLoadingOverview(false)
    }
  }, [])

  // 2. 获取表列表
  const fetchTables = useCallback(async () => {
    setLoadingTables(true)
    try {
      const data = await services.dbManage.listTables()
      setTables(data)
    } catch (err) {
      toast.error("获取数据库数据表列表失败", {
        description: err instanceof Error ? err.message : "未知错误",
      })
    } finally {
      setLoadingTables(false)
    }
  }, [])

  // 3. 协调刷新
  const handleRefreshAll = () => {
    fetchOverview()
    fetchTables()
    setRefreshTrigger((prev) => prev + 1)
  }

  // 4. 导出数据库备份
  const handleExport = async () => {
    setExporting(true)
    try {
      const { blob, filename } = await services.dbManage.exportDatabase()
      const url = URL.createObjectURL(blob)
      const a = document.createElement("a")
      a.href = url
      a.download = filename
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(url)
      toast.success("数据库导出成功", { description: `已下载归档文件: ${filename}` })
    } catch (err) {
      toast.error("数据库导出失败", {
        description: err instanceof Error ? err.message : "导出异常",
      })
    } finally {
      setExporting(false)
    }
  }

  // 初始化加载
  useEffect(() => {
    fetchOverview()
    fetchTables()
  }, [fetchOverview, fetchTables])

  // 渲染概览卡片骨架
  const renderOverviewSkeleton = () => (
    <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
      {Array.from({ length: 6 }).map((_, i) => (
        <Card key={i} className="border-border/40 bg-card/50">
          <CardHeader className="pb-2">
            <Skeleton className="h-4 w-16" />
          </CardHeader>
          <CardContent>
            <Skeleton className="h-6 w-24" />
          </CardContent>
        </Card>
      ))}
    </div>
  )

  // 渲染 SQL 查询控台全屏视图
  if (showConsole) {
    return (
      <SQLConsole
        dbType={overview?.type}
        onClose={() => setShowConsole(false)}
      />
    )
  }

  // 正常页面排版布局
  return (
    <div className="py-6 px-1 space-y-6 w-full">
      {/* 顶部控制与标题 */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between pb-4 gap-4">
        <div className="flex items-center gap-2">
          <Database className="size-5 text-primary" />
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">数据管理</h1>
          </div>
        </div>
        <Button
          size="sm"
          variant="secondary"
          className="h-8 gap-1.5 text-xs self-start sm:self-auto"
          onClick={handleRefreshAll}
          disabled={loadingOverview}
        >
          <RefreshCw className={`size-3 ${loadingOverview ? "animate-spin" : ""}`} />
          刷新数据
        </Button>
      </div>

      {/* 1. 概览指标卡片组 */}
      {loadingOverview ? (
        renderOverviewSkeleton()
      ) : (
        overview && (
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
            {/* 卡片1: 数据库类型 */}
            <Card className="shadow-sm border-border/40 bg-card/50 backdrop-blur-sm hover:border-primary/20 transition-all duration-300">
              <CardHeader className="flex flex-row items-center justify-between pb-2 space-y-0">
                <CardDescription className="text-[10px] font-medium">数据库类型</CardDescription>
                <Server className="size-3.5 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-sm font-bold uppercase">{overview.type}</div>
              </CardContent>
            </Card>

            {/* 卡片2: 数据库版本 */}
            <Card className="shadow-sm border-border/40 bg-card/50 backdrop-blur-sm hover:border-primary/20 transition-all duration-300 col-span-1 md:col-span-2 lg:col-span-1">
              <CardHeader className="flex flex-row items-center justify-between pb-2 space-y-0">
                <CardDescription className="text-[10px] font-medium">版本信息</CardDescription>
                <Cpu className="size-3.5 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-xs font-semibold truncate" title={overview.version}>
                  {overview.version.split(" ").slice(0, 2).join(" ")}
                </div>
              </CardContent>
            </Card>

            {/* 卡片3: 数据库名称 */}
            <Card className="shadow-sm border-border/40 bg-card/50 backdrop-blur-sm hover:border-primary/20 transition-all duration-300">
              <CardHeader className="flex flex-row items-center justify-between pb-2 space-y-0">
                <CardDescription className="text-[10px] font-medium">名称/路径</CardDescription>
                <Database className="size-3.5 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-xs font-semibold truncate" title={overview.name}>
                  {overview.name.substring(overview.name.lastIndexOf("/") + 1)}
                </div>
              </CardContent>
            </Card>

            {/* 卡片4: 数据库大小 */}
            <Card className="shadow-sm border-border/40 bg-card/50 backdrop-blur-sm hover:border-primary/20 transition-all duration-300">
              <CardHeader className="flex flex-row items-center justify-between pb-2 space-y-0">
                <CardDescription className="text-[10px] font-medium">数据库大小</CardDescription>
                <HardDrive className="size-3.5 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-sm font-bold">{overview.size}</div>
              </CardContent>
            </Card>

            {/* 卡片5: 表数量 */}
            <Card className="shadow-sm border-border/40 bg-card/50 backdrop-blur-sm hover:border-primary/20 transition-all duration-300">
              <CardHeader className="flex flex-row items-center justify-between pb-2 space-y-0">
                <CardDescription className="text-[10px] font-medium">物理数据表</CardDescription>
                <Layers className="size-3.5 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-sm font-bold">{formatNumber(overview.table_count)}</div>
              </CardContent>
            </Card>

            {/* 卡片6: 连接数 */}
            <Card className="shadow-sm border-border/40 bg-card/50 backdrop-blur-sm hover:border-primary/20 transition-all duration-300">
              <CardHeader className="flex flex-row items-center justify-between pb-2 space-y-0">
                <CardDescription className="text-[10px] font-medium">活跃连接数</CardDescription>
                <Activity className="size-3.5 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-sm font-bold">{formatNumber(overview.connections)}</div>
              </CardContent>
            </Card>
          </div>
        )
      )}

      {/* 2. 数据表浏览器区块 */}
      <TableBrowser
        tables={tables}
        loadingTables={loadingTables}
        refreshTrigger={refreshTrigger}
      />

      {/* 3. 缓存管理区块 */}
      <CacheManager
        refreshTrigger={refreshTrigger}
      />

      {/* 4. 底部功能卡片区 */}
      <Card className="border-border/40 bg-card/50 backdrop-blur-sm shadow-sm">
        <CardHeader className="pb-3 border-b border-dashed">
          <CardTitle className="text-sm font-semibold">功能区</CardTitle>
          <CardDescription className="text-[11px]">数据库导出及自定义高级 SQL 执行终端</CardDescription>
        </CardHeader>
        <CardContent className="pt-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {/* 功能一: 数据库导出 */}
            <div className="flex items-center justify-between p-4 border border-dashed rounded-lg hover:bg-muted/20 transition-colors duration-150">
              <div className="space-y-1 pr-4">
                <p className="text-xs font-semibold flex items-center gap-1.5">
                  <Download className="size-4 text-primary" />
                  数据库备份导出
                </p>
                <p className="text-[10px] text-muted-foreground">
                  直接导出并下载物理数据库镜像文件（SQLite 导出 .db，PostgreSQL 导出为打包的 .sql 文本文件）
                </p>
              </div>
              <Button
                size="sm"
                variant="outline"
                className="h-8 text-xs gap-1"
                onClick={handleExport}
                disabled={exporting}
              >
                <RefreshCw className={`size-3 ${exporting ? "animate-spin" : ""}`} />
                {exporting ? "正在准备..." : "开始导出"}
              </Button>
            </div>

            {/* 功能二: SQL 查询终端 */}
            <div className="flex items-center justify-between p-4 border border-dashed rounded-lg hover:bg-muted/20 transition-colors duration-150">
              <div className="space-y-1 pr-4">
                <p className="text-xs font-semibold flex items-center gap-1.5">
                  <Terminal className="size-4 text-primary" />
                  SQL 查询控台
                </p>
                <p className="text-[10px] text-muted-foreground">
                  打开在线 SQL 控制终端，执行原生的 SQL 进行数据筛选、更新、调试或性能优化
                </p>
              </div>
              <Button
                size="sm"
                className="h-8 text-xs gap-1 bg-primary text-primary-foreground hover:bg-primary/95"
                onClick={() => {
                  setShowConsole(true)
                }}
              >
                <FileText className="size-3.5" />
                进入控台
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}