"use client"

import {useCallback, useEffect, useMemo, useState} from "react"
import {Eye, GitCompare, History, Loader2, Play, RefreshCw, Trash2,} from "lucide-react"
import {toast} from "sonner"

import {EmptyStateWithBorder} from "@/components/layout/empty"
import {ErrorInline} from "@/components/layout/error"
import {LoadingStateWithBorder} from "@/components/layout/loading"
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {Badge} from "@/components/ui/badge"
import {Button} from "@/components/ui/button"
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow,} from "@/components/ui/table"
import {
  type ConfigDiffResult,
  type ConfigPreviewResult,
  ConfigVersionService,
  type ConfigVersionSummary,
} from "@/lib/services/openflare"
import {formatDateTime} from "@/lib/utils"

import {CleanupDialog} from "./components/cleanup-dialog"
import {DiffDialog} from "./components/diff-dialog"
import {PreviewSheet} from "./components/preview-sheet"
import {VersionSnapshotSheet} from "./components/version-snapshot-sheet"

function truncateChecksum(checksum: string) {
  if (!checksum) return "—"
  return checksum.length > 16 ? `${checksum.slice(0, 16)}...` : checksum
}

function hasConfigDiff(diff: ConfigDiffResult) {
  return (
    diff.added_domains.length > 0 ||
    diff.removed_domains.length > 0 ||
    diff.modified_domains.length > 0 ||
    diff.main_config_changed ||
    diff.changed_option_keys.length > 0 ||
    !diff.active_version
  )
}

export default function ConfigVersionsPage() {
  const [versions, setVersions] = useState<ConfigVersionSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [previewOpen, setPreviewOpen] = useState(false)
  const [preview, setPreview] = useState<ConfigPreviewResult | null>(null)
  const [previewLoading, setPreviewLoading] = useState(false)
  const [previewError, setPreviewError] = useState<string | null>(null)

  const [diffOpen, setDiffOpen] = useState(false)
  const [diff, setDiff] = useState<ConfigDiffResult | null>(null)
  const [diffLoading, setDiffLoading] = useState(false)
  const [diffError, setDiffError] = useState<string | null>(null)

  const [snapshotVersion, setSnapshotVersion] = useState<ConfigVersionSummary | null>(null)
  const [snapshotOpen, setSnapshotOpen] = useState(false)

  const [publishConfirmOpen, setPublishConfirmOpen] = useState(false)
  const [forcePublishConfirmOpen, setForcePublishConfirmOpen] = useState(false)
  const [activateTarget, setActivateTarget] = useState<ConfigVersionSummary | null>(null)
  const [cleanupOpen, setCleanupOpen] = useState(false)

  const [publishing, setPublishing] = useState(false)
  const [activating, setActivating] = useState(false)
  const [cleaning, setCleaning] = useState(false)

  const canPublish = useMemo(
    () => Boolean(preview && preview.route_count > 0 && diff && hasConfigDiff(diff)),
    [preview, diff],
  )

  const fetchVersions = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await ConfigVersionService.list()
      setVersions(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载失败")
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void fetchVersions()
  }, [fetchVersions])

  const loadPreviewData = async () => {
    setPreviewLoading(true)
    setPreviewError(null)
    setDiffLoading(true)
    setDiffError(null)

    try {
      const [previewData, diffData] = await Promise.all([
        ConfigVersionService.preview(),
        ConfigVersionService.diff(),
      ])
      setPreview(previewData)
      setDiff(diffData)
    } catch (err) {
      const message = err instanceof Error ? err.message : "加载预览失败"
      setPreviewError(message)
      setDiffError(message)
    } finally {
      setPreviewLoading(false)
      setDiffLoading(false)
    }
  }

  const handleOpenPreview = async () => {
    setPreviewOpen(true)
    if (!preview) {
      await loadPreviewData()
    }
  }

  const handleOpenDiff = async () => {
    setDiffOpen(true)
    if (!diff) {
      setDiffLoading(true)
      setDiffError(null)
      try {
        const diffData = await ConfigVersionService.diff()
        setDiff(diffData)
      } catch (err) {
        setDiffError(err instanceof Error ? err.message : "加载差异失败")
      } finally {
        setDiffLoading(false)
      }
    }
  }

  const handlePublish = async (force = false) => {
    setPublishing(true)
    try {
      const version = await ConfigVersionService.publish(force)
      toast.success("发布成功", { description: `版本 ${version.version}` })
      setPreviewOpen(false)
      setPublishConfirmOpen(false)
      setForcePublishConfirmOpen(false)
      setPreview(null)
      setDiff(null)
      await fetchVersions()
    } catch (err) {
      toast.error("发布失败", {
        description: err instanceof Error ? err.message : "未知错误",
      })
    } finally {
      setPublishing(false)
    }
  }

  const handleActivate = async () => {
    if (!activateTarget) return

    setActivating(true)
    try {
      const version = await ConfigVersionService.activate(activateTarget.id)
      toast.success("激活成功", { description: `版本 ${version.version}` })
      setActivateTarget(null)
      await fetchVersions()
    } catch (err) {
      toast.error("激活失败", {
        description: err instanceof Error ? err.message : "未知错误",
      })
    } finally {
      setActivating(false)
    }
  }

  const handleCleanup = async (keepCount: number) => {
    setCleaning(true)
    try {
      const result = await ConfigVersionService.cleanup({ keep_count: keepCount })
      toast.success("清理完成", {
        description: `已删除 ${result.deleted_count} 个历史快照`,
      })
      setCleanupOpen(false)
      await fetchVersions()
    } catch (err) {
      toast.error("清理失败", {
        description: err instanceof Error ? err.message : "未知错误",
      })
    } finally {
      setCleaning(false)
    }
  }

  return (
    <div className="py-6 px-1 space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2">
          <History className="size-5 text-primary" />
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">配置版本</h1>
            <p className="text-sm text-muted-foreground">
              查看历史快照、预览待发布配置差异，并在需要时重新激活旧版本。
            </p>
          </div>
        </div>

        <div className="flex flex-wrap gap-2">
          <Button variant="outline" size="sm" onClick={() => void fetchVersions()} disabled={loading}>
            <RefreshCw className={`size-3.5 mr-1 ${loading ? "animate-spin" : ""}`} />
            刷新
          </Button>
          <Button variant="outline" size="sm" onClick={() => setCleanupOpen(true)}>
            <Trash2 className="size-3.5 mr-1" />
            清理旧版本
          </Button>
          <Button variant="outline" size="sm" onClick={() => void handleOpenDiff()}>
            <GitCompare className="size-3.5 mr-1" />
            查看差异
          </Button>
          <Button variant="outline" size="sm" onClick={() => setForcePublishConfirmOpen(true)}>
            强制发布
          </Button>
          <Button size="sm" onClick={() => void handleOpenPreview()}>
            <Eye className="size-3.5 mr-1" />
            预览并发布
          </Button>
        </div>
      </div>

      {error ? <ErrorInline message={error} onRetry={() => void fetchVersions()} /> : null}

      <div className="border border-dashed shadow-none rounded-lg overflow-hidden bg-background">
        {loading ? (
          <LoadingStateWithBorder />
        ) : versions.length === 0 ? (
          <EmptyStateWithBorder
            title="暂无历史版本"
            description="当前还没有可查看的发布记录，请先触发一次配置发布。"
          />
        ) : (
          <Table>
            <TableHeader className="bg-muted/40">
              <TableRow className="border-dashed hover:bg-transparent">
                <TableHead className="text-xs font-semibold">版本号</TableHead>
                <TableHead className="text-xs font-semibold">状态</TableHead>
                <TableHead className="text-xs font-semibold">创建人</TableHead>
                <TableHead className="text-xs font-semibold">Checksum</TableHead>
                <TableHead className="text-xs font-semibold">创建时间</TableHead>
                <TableHead className="text-xs font-semibold text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {versions.map((version) => (
                <TableRow
                  key={version.id}
                  className="border-dashed hover:bg-muted/10 transition-colors"
                >
                  <TableCell className="font-mono text-xs font-semibold">
                    {version.version}
                  </TableCell>
                  <TableCell>
                    {version.is_active ? (
                      <Badge
                        variant="outline"
                        className="text-[10px] bg-emerald-500/10 border-emerald-500/20 text-emerald-600 rounded-full py-0 px-2"
                      >
                        <span className="size-1 bg-emerald-500 rounded-full mr-1.5 shrink-0" />
                        当前激活
                      </Badge>
                    ) : (
                      <Badge variant="outline" className="text-[10px] rounded-full py-0 px-2">
                        历史版本
                      </Badge>
                    )}
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {version.created_by || "系统"}
                  </TableCell>
                  <TableCell
                    className="text-xs font-mono text-muted-foreground"
                    title={version.checksum}
                  >
                    {truncateChecksum(version.checksum)}
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {formatDateTime(version.created_at)}
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex items-center justify-end gap-1.5">
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-7 text-xs"
                        onClick={() => {
                          setSnapshotVersion(version)
                          setSnapshotOpen(true)
                        }}
                      >
                        <Eye className="size-3 mr-1" />
                        快照
                      </Button>
                      {!version.is_active ? (
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-7 text-xs"
                          onClick={() => setActivateTarget(version)}
                        >
                          <Play className="size-3 mr-1" />
                          激活
                        </Button>
                      ) : null}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>

      <PreviewSheet
        open={previewOpen}
        onOpenChange={setPreviewOpen}
        preview={preview}
        loading={previewLoading}
        error={previewError}
        publishing={publishing}
        canPublish={canPublish}
        onPublish={() => setPublishConfirmOpen(true)}
      />

      <DiffDialog
        open={diffOpen}
        onOpenChange={setDiffOpen}
        diff={diff}
        loading={diffLoading}
        error={diffError}
      />

      <VersionSnapshotSheet
        version={snapshotVersion}
        open={snapshotOpen}
        onOpenChange={setSnapshotOpen}
      />

      <CleanupDialog
        open={cleanupOpen}
        onOpenChange={setCleanupOpen}
        onConfirm={(keepCount) => void handleCleanup(keepCount)}
        loading={cleaning}
      />

      <AlertDialog open={publishConfirmOpen} onOpenChange={setPublishConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认发布配置</AlertDialogTitle>
            <AlertDialogDescription>
              将把当前待发布配置生成新版本并设为激活版本，节点将随后拉取更新。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={publishing}>取消</AlertDialogCancel>
            <Button onClick={() => void handlePublish(false)} disabled={publishing}>
              {publishing ? <Loader2 className="size-4 animate-spin" /> : "确认发布"}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={forcePublishConfirmOpen} onOpenChange={setForcePublishConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认强制发布</AlertDialogTitle>
            <AlertDialogDescription>
              将忽略配置变化检查并立即生成一个新版本，请确认你确实需要重新发布。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={publishing}>取消</AlertDialogCancel>
            <Button
              variant="destructive"
              onClick={() => void handlePublish(true)}
              disabled={publishing}
            >
              {publishing ? <Loader2 className="size-4 animate-spin" /> : "强制发布"}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={activateTarget !== null}
        onOpenChange={(open) => {
          if (!open) setActivateTarget(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认激活版本</AlertDialogTitle>
            <AlertDialogDescription>
              {activateTarget
                ? `确认将版本 ${activateTarget.version} 设为当前激活版本吗？`
                : ""}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={activating}>取消</AlertDialogCancel>
            <Button onClick={() => void handleActivate()} disabled={activating}>
              {activating ? <Loader2 className="size-4 animate-spin" /> : "确认激活"}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
