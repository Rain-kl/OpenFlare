"use client"

import * as React from "react"
import {useCallback, useEffect, useRef, useState} from "react"
import {toast} from "sonner"
import {Activity, Cpu, Database, HardDrive, Layers, RefreshCw} from "lucide-react"
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card"
import {Button} from "@/components/ui/button"
import {Switch} from "@/components/ui/switch"
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue,} from "@/components/ui/select"
import {Badge} from "@/components/ui/badge"
import {Progress} from "@/components/ui/progress"
import {Skeleton} from "@/components/ui/skeleton"
import type {SystemStatus} from "@/lib/services/admin"
import services from "@/lib/services"

/**
 * 格式化数字，每3位加逗号
 */
const formatNumber = (num: number | string) => {
  if (num === undefined || num === null) return "0"
  return num.toString().replace(/\B(?=(\d{3})+(?!\B))/g, ",")
}

/**
 * 运行时系统状态展示与管理组件
 */
export function SystemStatusManager() {
  const [status, setStatus] = useState<SystemStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [wavelet, setWavelet] = useState(false)
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [intervalTime, setIntervalTime] = useState("5000") // 默认5秒

  const intervalRef = useRef<NodeJS.Timeout | null>(null)
  const prevStatusRef = useRef<SystemStatus | null>(null)
  const [changedFields, setChangedFields] = useState<Record<string, boolean>>({})

  // 获取状态数据
  const fetchStatus = useCallback(async (isSilent = false) => {
    if (!isSilent) setWavelet(true)
    try {
      const data = await services.adminStatus.getSystemStatus()

      // 检测变化的字段用于微动画效果
      if (prevStatusRef.current) {
        const changes: Record<string, boolean> = {}
        Object.keys(data).forEach((key) => {
          const k = key as keyof SystemStatus
          if (prevStatusRef.current && prevStatusRef.current[k] !== data[k]) {
            changes[k] = true
          }
        })
        setChangedFields(changes)
        // 1秒后清除变化状态
        setTimeout(() => {
          setChangedFields({})
        }, 1000)
      }

      setStatus(data)
      prevStatusRef.current = data
    } catch (err) {
      toast.error("获取系统状态失败", {
        description: err instanceof Error ? err.message : "请求时发生未知错误"
      })
    } finally {
      setLoading(false)
      setWavelet(false)
    }
  }, [])

  // 轮询逻辑
  useEffect(() => {
    // 首次加载
    fetchStatus()
  }, [fetchStatus])

  useEffect(() => {
    if (intervalRef.current) {
      clearInterval(intervalRef.current)
    }

    if (autoRefresh) {
      const time = parseInt(intervalTime, 10)
      intervalRef.current = setInterval(() => {
        fetchStatus(true)
      }, time)
    }

    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
      }
    }
  }, [autoRefresh, intervalTime, fetchStatus])

  // 计算内存分配百分比 (Alloc / Sys)
  const getMemoryUsagePercent = () => {
    if (!status) return 0

    const parseSizeToBytes = (sizeStr: string): number => {
      const parts = sizeStr.split(" ")
      if (parts.length < 2) return 0
      const value = parseFloat(parts[0])
      const unit = parts[1].toLowerCase()

      if (unit.startsWith("gib")) return value * 1024 * 1024 * 1024
      if (unit.startsWith("mib")) return value * 1024 * 1024
      if (unit.startsWith("kib")) return value * 1024
      return value
    }

    const alloc = parseSizeToBytes(status.alloc)
    const sys = parseSizeToBytes(status.sys)
    if (sys === 0) return 0
    return Math.min(Math.round((alloc / sys) * 100), 100)
  }

  // 闪烁动画类
  const getPulseClass = (field: string) => {
    return changedFields[field] ? "animate-pulse text-primary font-bold transition-all duration-300 scale-105 inline-block" : "transition-all duration-300"
  }

  const RenderMetricRow = ({ label, value, field }: { label: string; value: string | number; field?: string }) => (
    <div className="flex items-center justify-between py-2 border-b border-dashed border-border/40 last:border-b-0 hover:bg-muted/30 px-2 rounded-sm transition-colors duration-150">
      <span className="text-xs font-medium text-muted-foreground">{label}</span>
      <span className={`text-xs font-mono font-medium ${field ? getPulseClass(field) : ""}`}>
        {typeof value === "number" ? formatNumber(value) : value}
      </span>
    </div>
  )

  if (loading) {
    return (
      <div className="space-y-6 p-1">
        <div className="flex items-center justify-between border-b border-border/40 pb-4">
          <div className="space-y-1">
            <Skeleton className="h-6 w-32" />
            <Skeleton className="h-4 w-48" />
          </div>
          <div className="flex items-center gap-3">
            <Skeleton className="h-8 w-24" />
            <Skeleton className="h-8 w-16" />
            <Skeleton className="h-8 w-8" />
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          <Skeleton className="h-48 w-full" />
          <Skeleton className="h-48 w-full" />
          <Skeleton className="h-48 w-full" />
          <Skeleton className="h-48 w-full" />
          <Skeleton className="h-48 w-full" />
        </div>
      </div>
    )
  }

  return (
    <div className="py-6 px-1 space-y-6">
      {/* 顶部控制栏 */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between pb-4 gap-4">
        <div className="flex items-center gap-2">
          <Activity className="size-5 text-primary" />
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">系统状态</h1>
          </div>
        </div>

        <div className="flex items-center flex-wrap gap-4 bg-muted/30 p-1.5 rounded-lg border border-border/40 backdrop-blur-sm">
          <div className="flex items-center gap-2 px-1">
            <Switch
              id="auto-refresh"
              checked={autoRefresh}
              onCheckedChange={setAutoRefresh}
            />
            <label htmlFor="auto-refresh" className="text-xs font-medium cursor-pointer select-none">
              自动刷新
            </label>
          </div>

          {autoRefresh && (
            <Select value={intervalTime} onValueChange={setIntervalTime}>
              <SelectTrigger className="h-7 w-20 text-[11px] bg-background border-border/40">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="1000">1s</SelectItem>
                <SelectItem value="2000">2s</SelectItem>
                <SelectItem value="5000">5s</SelectItem>
                <SelectItem value="10000">10s</SelectItem>
              </SelectContent>
            </Select>
          )}

          <Button
            size="sm"
            variant="secondary"
            className="h-7 text-[11px] gap-1.5"
            onClick={() => fetchStatus()}
            disabled={wavelet}
          >
            <RefreshCw className={`size-3 ${wavelet ? "animate-spin" : ""}`} />
            刷新
          </Button>
        </div>
      </div>

      {status && (
        <>
          {/* 主网格排版 */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">

            {/* 1. 服务概览 */}
            <Card className="shadow-sm border-border/40 bg-card/50 backdrop-blur-md hover:border-primary/20 transition-all duration-300">
              <CardHeader className="flex flex-row items-center justify-between pb-2 space-y-0">
                <div className="space-y-0.5">
                  <CardTitle className="text-sm font-semibold">服务概览</CardTitle>
                  <CardDescription className="text-[10px]">服务的基础生命指标</CardDescription>
                </div>
                <Badge variant="secondary" className="p-1 rounded-full bg-primary/10 text-primary border-none">
                  <Activity className="size-4" />
                </Badge>
              </CardHeader>
              <CardContent className="space-y-1">
                <RenderMetricRow label="服务运行时间" value={status.uptime} field="uptime" />
                <RenderMetricRow label="当前 Goroutines 数量" value={status.num_goroutine} field="num_goroutine" />
                <RenderMetricRow label="Heap 对象数量" value={status.heap_objects} field="heap_objects" />
              </CardContent>
            </Card>

            {/* 2. 内存统计 */}
            <Card className="shadow-sm border-border/40 bg-card/50 backdrop-blur-md hover:border-primary/20 transition-all duration-300">
              <CardHeader className="flex flex-row items-center justify-between pb-2 space-y-0">
                <div className="space-y-0.5">
                  <CardTitle className="text-sm font-semibold">内存统计</CardTitle>
                  <CardDescription className="text-[10px]">主内存消耗概览</CardDescription>
                </div>
                <Badge variant="secondary" className="p-1 rounded-full bg-primary/10 text-primary border-none">
                  <Cpu className="size-4" />
                </Badge>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="space-y-1">
                  <RenderMetricRow label="当前内存使用量" value={status.alloc} field="alloc" />
                  <RenderMetricRow label="所有已分配的内存" value={status.total_alloc} field="total_alloc" />
                  <RenderMetricRow label="内存占用量" value={status.sys} field="sys" />
                  <RenderMetricRow label="下次 GC 内存回收量" value={status.next_gc} field="next_gc" />
                </div>
                {/* 物理内存水位比例 */}
                <div className="space-y-1 px-1 pt-1">
                  <div className="flex justify-between text-[10px] text-muted-foreground">
                    <span>当前使用率 (Alloc / Sys)</span>
                    <span className="font-mono">{getMemoryUsagePercent()}%</span>
                  </div>
                  <Progress value={getMemoryUsagePercent()} className="h-1.5" />
                </div>
              </CardContent>
            </Card>

            {/* 3. 堆/栈详情 */}
            <Card className="shadow-sm border-border/40 bg-card/50 backdrop-blur-md hover:border-primary/20 transition-all duration-300">
              <CardHeader className="flex flex-row items-center justify-between pb-2 space-y-0">
                <div className="space-y-0.5">
                  <CardTitle className="text-sm font-semibold">堆/栈详情</CardTitle>
                  <CardDescription className="text-[10px]">运行时堆栈内存空间细节</CardDescription>
                </div>
                <Badge variant="secondary" className="p-1 rounded-full bg-primary/10 text-primary border-none">
                  <Database className="size-4" />
                </Badge>
              </CardHeader>
              <CardContent className="space-y-1">
                <RenderMetricRow label="当前 Heap 内存使用量" value={status.heap_alloc} field="heap_alloc" />
                <RenderMetricRow label="Heap 内存占用量" value={status.heap_sys} field="heap_sys" />
                <RenderMetricRow label="Heap 内存空闲量" value={status.heap_idle} field="heap_idle" />
                <RenderMetricRow label="正在使用的 Heap 内存" value={status.heap_inuse} field="heap_inuse" />
                <RenderMetricRow label="已释放的 Heap 内存" value={status.heap_released} field="heap_released" />
                <RenderMetricRow label="启动 Stack 使用量" value={status.stack_inuse} field="stack_inuse" />
                <RenderMetricRow label="已分配的 Stack 内存" value={status.stack_sys} field="stack_sys" />
              </CardContent>
            </Card>

            {/* 4. 底层组件与结构体 */}
            <Card className="shadow-sm border-border/40 bg-card/50 backdrop-blur-md hover:border-primary/20 transition-all duration-300">
              <CardHeader className="flex flex-row items-center justify-between pb-2 space-y-0">
                <div className="space-y-0.5">
                  <CardTitle className="text-sm font-semibold">底层及结构体内存</CardTitle>
                  <CardDescription className="text-[10px]">运行时底层管理结构体开销</CardDescription>
                </div>
                <Badge variant="secondary" className="p-1 rounded-full bg-primary/10 text-primary border-none">
                  <HardDrive className="size-4" />
                </Badge>
              </CardHeader>
              <CardContent className="space-y-1">
                <RenderMetricRow label="MSpan 结构内存使用量" value={status.mspan_inuse} field="mspan_inuse" />
                <RenderMetricRow label="已分配的 MSpan 结构内存" value={status.mspan_sys} field="mspan_sys" />
                <RenderMetricRow label="MCache 结构内存使用量" value={status.mcache_inuse} field="mcache_inuse" />
                <RenderMetricRow label="已分配的 MCache 结构内存" value={status.mcache_sys} field="mcache_sys" />
                <RenderMetricRow label="已分配的剖析哈希表内存" value={status.buck_hash_sys} field="buck_hash_sys" />
                <RenderMetricRow label="已分配的 GC 元数据内存" value={status.gc_sys} field="gc_sys" />
                <RenderMetricRow label="其它已分配的系统内存" value={status.other_sys} field="other_sys" />
              </CardContent>
            </Card>

            {/* 5. 垃圾回收与分配计数 */}
            <Card className="shadow-sm border-border/40 bg-card/50 backdrop-blur-md hover:border-primary/20 transition-all duration-300 md:col-span-2">
              <CardHeader className="flex flex-row items-center justify-between pb-2 space-y-0">
                <div className="space-y-0.5">
                  <CardTitle className="text-sm font-semibold">垃圾回收与分配计数</CardTitle>
                  <CardDescription className="text-[10px]">GC 历史数据与分配频次</CardDescription>
                </div>
                <Badge variant="secondary" className="p-1 rounded-full bg-primary/10 text-primary border-none">
                  <Layers className="size-4" />
                </Badge>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-6">
                  <div>
                    <RenderMetricRow label="GC 执行次数" value={status.num_gc} field="num_gc" />
                    <RenderMetricRow label="距离上次 GC 时间" value={status.last_gc_time} field="last_gc_time" />
                    <RenderMetricRow label="GC 暂停时间总量" value={status.pause_total_ns} field="pause_total_ns" />
                    <RenderMetricRow label="上次 GC 暂停时间" value={status.last_pause} field="last_pause" />
                  </div>
                  <div>
                    <RenderMetricRow label="内存分配次数" value={status.mallocs} field="mallocs" />
                    <RenderMetricRow label="内存释放次数" value={status.frees} field="frees" />
                    <RenderMetricRow label="指针查找次数" value={status.lookups} field="lookups" />
                  </div>
                </div>
              </CardContent>
            </Card>

          </div>
        </>
      )}
    </div>
  )
}
