"use client"

import * as React from "react"
import {useCallback, useEffect, useState} from "react"
import {toast} from "sonner"
import {HardDrive, RefreshCw, Trash2} from "lucide-react"

import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card"
import {Button} from "@/components/ui/button"
import {Skeleton} from "@/components/ui/skeleton"
import {Label} from "@/components/ui/label"
import {Input} from "@/components/ui/input"
import {Switch} from "@/components/ui/switch"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import type {CacheStatus} from "@/lib/services/admin"
import services from "@/lib/services"

/**
 * 格式化字节大小
 */
const formatBytes = (bytes: number) => {
  if (bytes === 0) return "0 Bytes"
  const k = 1024
  const sizes = ["Bytes", "KB", "MB", "GB", "TB"]
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i]
}

/**
 * 格式化数字，每3位加逗号
 */
const formatNumber = (num: number | string) => {
  if (num === undefined || num === null) return "0"
  return num.toString().replace(/\B(?=(\d{3})+(?!\B))/g, ",")
}

interface CacheManagerProps {
  refreshTrigger: number
}

export function CacheManager({ refreshTrigger }: CacheManagerProps) {
  const [cacheStatus, setCacheStatus] = useState<CacheStatus | null>(null)
  const [loadingCache, setLoadingCache] = useState<boolean>(true)
  const [savingConfig, setSavingConfig] = useState<boolean>(false)
  const [clearingCache, setClearingCache] = useState<boolean>(false)
  const [showClearConfirm, setShowClearConfirm] = useState<boolean>(false)

  // 策略配置表单状态
  const [maxSizeMB, setMaxSizeMB] = useState<string>("100")
  const [ttlMinutes, setTtlMinutes] = useState<string>("60")
  const [lruEnabled, setLruEnabled] = useState<boolean>(true)

  // 获取磁盘缓存状态
  const fetchCacheStatus = useCallback(async (isSilent = false) => {
    if (!isSilent) setLoadingCache(true)
    try {
      const data = await services.adminCache.getCacheStatus()
      setCacheStatus(data)
      setMaxSizeMB(data.max_size_mb.toString())
      setTtlMinutes(data.ttl_minutes.toString())
      setLruEnabled(data.lru_enabled)
    } catch (err) {
      toast.error("获取磁盘缓存状态失败", {
        description: err instanceof Error ? err.message : "未知错误",
      })
    } finally {
      setLoadingCache(false)
    }
  }, [])

  // 保存配置
  const handleSaveConfig = async (e: React.FormEvent) => {
    e.preventDefault()
    const size = parseInt(maxSizeMB, 10)
    const ttl = parseInt(ttlMinutes, 10)
    if (isNaN(size) || size < 1) {
      toast.error("保存失败", { description: "最大容量限制必须是大于等于 1 的整数" })
      return
    }
    if (isNaN(ttl) || ttl < 0) {
      toast.error("保存失败", { description: "默认过期时间必须是大于等于 0 的整数" })
      return
    }
    setSavingConfig(true)
    try {
      await services.adminCache.updateCacheConfig({
        max_size_mb: size,
        ttl_minutes: ttl,
        lru_enabled: lruEnabled,
      })
      toast.success("保存成功", { description: "磁盘缓存策略已热更新" })
      await fetchCacheStatus(true)
    } catch (err) {
      toast.error("保存配置失败", {
        description: err instanceof Error ? err.message : "未知错误",
      })
    } finally {
      setSavingConfig(false)
    }
  }

  // 清空磁盘缓存数据
  const handleClearCache = async () => {
    setClearingCache(true)
    try {
      await services.adminCache.clearCache()
      toast.success("清空成功", { description: "缓存数据已全部清除" })
      setShowClearConfirm(false)
      await fetchCacheStatus(true)
    } catch (err) {
      toast.error("清空缓存失败", {
        description: err instanceof Error ? err.message : "未知错误",
      })
    } finally {
      setClearingCache(false)
    }
  }

  // 初始化拉取
  useEffect(() => {
    fetchCacheStatus()
  }, [fetchCacheStatus])

  // 当外部刷新触发器改变时自动刷新
  useEffect(() => {
    fetchCacheStatus(true)
  }, [refreshTrigger, fetchCacheStatus])

  return (
    <Card className="border-border/40 bg-card/50 backdrop-blur-sm shadow-sm">
      <CardHeader className="pb-3 border-b border-dashed flex flex-col md:flex-row md:items-center md:justify-between gap-4">
        <div className="space-y-0.5">
          <div className="flex items-center gap-2">
            <HardDrive className="size-4 text-primary animate-pulse" />
            <CardTitle className="text-sm font-semibold">缓存管理</CardTitle>
          </div>
          <CardDescription className="text-[11px]">管理和监控系统级磁盘缓存的资源占用、生命周期及淘汰策略</CardDescription>
        </div>

        <Button
          size="sm"
          variant="secondary"
          className="h-8 gap-1.5 text-xs self-start md:self-auto"
          onClick={() => fetchCacheStatus(true)}
          disabled={loadingCache}
        >
          <RefreshCw className={`size-3 ${loadingCache ? "animate-spin" : ""}`} />
          刷新状态
        </Button>
      </CardHeader>
      <CardContent className="pt-4">
        {loadingCache && !cacheStatus ? (
          <div className="space-y-3 py-6">
            <Skeleton className="h-6 w-full" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : cacheStatus ? (
          <div className="grid grid-cols-1 lg:grid-cols-5 gap-6">
            {/* 左边：状态区 (2/5 cols) */}
            <div className="lg:col-span-2 space-y-4">
              <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">运行状态</h4>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                {/* 已占空间 */}
                <div className="p-4 rounded-xl border border-border/40 bg-background/30 backdrop-blur-xs hover:border-primary/20 transition-all duration-300">
                  <p className="text-[10px] text-muted-foreground font-medium mb-1">已用空间</p>
                  <p className="text-xl font-bold tracking-tight text-foreground">
                    {formatBytes(cacheStatus.total_size)}
                  </p>
                </div>
                {/* Key数量 */}
                <div className="p-4 rounded-xl border border-border/40 bg-background/30 backdrop-blur-xs hover:border-primary/20 transition-all duration-300">
                  <p className="text-[10px] text-muted-foreground font-medium mb-1">缓存键数量</p>
                  <p className="text-xl font-bold tracking-tight text-foreground">
                    {formatNumber(cacheStatus.keys_count)} <span className="text-xs text-muted-foreground font-normal">个文件</span>
                  </p>
                </div>
              </div>

              {/* 存储路径 */}
              <div className="p-4 rounded-xl border border-border/40 bg-background/30 backdrop-blur-xs hover:border-primary/20 transition-all duration-300">
                <p className="text-[10px] text-muted-foreground font-medium mb-1.5">缓存基准目录</p>
                <code className="text-xs font-mono bg-muted/60 px-2 py-1 rounded-md block truncate" title={cacheStatus.base_path}>
                  {cacheStatus.base_path}
                </code>
              </div>
              <Button
                variant="secondary"
                size="sm"
                className="h-8 text-xs font-medium w-full"
                onClick={() => setShowClearConfirm(true)}
              >
                立即清空缓存
              </Button>
            </div>

            {/* 右边：配置区 (3/5 cols) */}
            <div className="lg:col-span-3 border-t lg:border-t-0 lg:border-l border-border/40 pt-6 lg:pt-0 lg:pl-6 space-y-4">
              <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">策略配置</h4>
              <form onSubmit={handleSaveConfig} className="space-y-4">
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  <div className="space-y-1.5">
                    <Label htmlFor="maxSizeMB" className="text-xs font-medium">最大容量限制 (MB)</Label>
                    <Input
                      id="maxSizeMB"
                      type="number"
                      min="1"
                      value={maxSizeMB}
                      onChange={(e) => setMaxSizeMB(e.target.value)}
                      className="h-8 text-xs bg-background/50 border-border/40"
                      placeholder="例如 100"
                      required
                    />
                    <p className="text-[9px] text-muted-foreground">当总大小超出该值时，自动触发淘汰算法</p>
                  </div>

                  <div className="space-y-1.5">
                    <Label htmlFor="ttlMinutes" className="text-xs font-medium">生存时间限制 (分钟)</Label>
                    <Input
                      id="ttlMinutes"
                      type="number"
                      min="0"
                      value={ttlMinutes}
                      onChange={(e) => setTtlMinutes(e.target.value)}
                      className="h-8 text-xs bg-background/50 border-border/40"
                      placeholder="例如 60，0 表示永不过期"
                      required
                    />
                    <p className="text-[9px] text-muted-foreground">缓存项的最长存活时间，超期后将失效被清理</p>
                  </div>
                </div>

                <div className="flex items-start justify-between p-4 rounded-xl border border-border/40 bg-background/20">
                  <div className="space-y-1 pr-4">
                    <Label htmlFor="lruEnabled" className="text-xs font-semibold block cursor-pointer">
                      启用 LRU 淘汰机制
                    </Label>
                    <span className="text-[10px] text-muted-foreground block">
                      在到达最大容量限制时，自动移除最久未被访问的缓存项。关闭该功能仅清理过期项。
                    </span>
                  </div>
                  <Switch
                    id="lruEnabled"
                    checked={lruEnabled}
                    onCheckedChange={setLruEnabled}
                  />
                </div>

                <div className="flex justify-end pt-2">
                  <Button
                    type="submit"
                    disabled={savingConfig}
                    className="h-8 text-xs px-4"
                  >
                    {savingConfig ? "正在保存..." : "保存设置"}
                  </Button>
                </div>
              </form>
            </div>
          </div>
        ) : (
          <div className="flex flex-col items-center justify-center py-10 text-muted-foreground">
            <HardDrive className="size-8 opacity-45 mb-2" />
            <span className="text-xs">未加载到缓存状态信息</span>
          </div>
        )}
      </CardContent>

      {/* 清除缓存确认对话框 */}
      <Dialog open={showClearConfirm} onOpenChange={setShowClearConfirm}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="text-sm font-semibold flex items-center gap-2 text-destructive">
              <Trash2 className="size-4" />
              确认清空所有缓存？
            </DialogTitle>
            <DialogDescription className="text-xs text-muted-foreground pt-1">
              该操作将彻底清空磁盘目录下的全部缓存文件（包含临时解压、处理后的图片及各种块文件），重置键数量统计为 0。此操作不可撤销，且可能导致用户拉取资源时出现一过性的响应变慢。
            </DialogDescription>
          </DialogHeader>
          <DialogFooter className="flex gap-2 justify-end mt-4">
            <Button
              variant="outline"
              size="sm"
              className="h-8 text-xs"
              onClick={() => setShowClearConfirm(false)}
              disabled={clearingCache}
            >
              取消
            </Button>
            <Button
              variant="destructive"
              size="sm"
              className="h-8 text-xs"
              onClick={handleClearCache}
              disabled={clearingCache}
            >
              {clearingCache ? "清理中..." : "确认清空"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  )
}
