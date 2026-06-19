"use client"

import {useMutation, useQuery} from "@tanstack/react-query"
import {ExternalLink, RefreshCw, Sparkles} from "lucide-react"
import {toast} from "sonner"

import services from "@/lib/services"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger
} from "@/components/ui/alert-dialog"
import {Badge} from "@/components/ui/badge"
import {Button} from "@/components/ui/button"
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card"
import {Spinner} from "@/components/ui/spinner"

function InfoRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 border-b border-dashed py-2 last:border-b-0">
      <span className="text-xs text-muted-foreground">{label}</span>
      <span className="text-right text-xs font-medium text-foreground break-all">{value || "-"}</span>
    </div>
  )
}

export function InfoTab() {
  const updateQuery = useQuery({
    queryKey: ["admin", "update"],
    queryFn: () => services.adminStatus.getUpdateStatus(),
    refetchInterval: 30 * 60 * 1000,
    staleTime: 5 * 60 * 1000,
  })

  const applyUpdateMutation = useMutation({
    mutationFn: () => services.adminStatus.applyUpdate(),
    onSuccess: () => {
      toast.success("升级包已校验完成，服务正在重启")
    },
    onError: (error: Error) => {
      toast.error(error.message || "应用升级失败")
    },
  })

  const update = updateQuery.data

  return (
    <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <Card className="border border-dashed shadow-sm">
        <CardHeader className="border-b border-dashed pb-4">
          <div className="flex items-start justify-between gap-4">
            <div className="flex items-center gap-2">
              <div className="rounded-lg bg-muted p-1.5 text-muted-foreground">
                <Sparkles className="size-4" />
              </div>
              <div>
                <CardTitle className="text-base font-semibold">应用更新</CardTitle>
                <CardDescription className="text-xs">
                  检查上游 GitHub Actions Release 并升级当前服务
                </CardDescription>
              </div>
            </div>
            {update?.update_available ? (
              <Badge>发现新版本</Badge>
            ) : update ? (
              <Badge variant="secondary">已是最新</Badge>
            ) : null}
          </div>
        </CardHeader>
        <CardContent className="flex flex-col gap-4 pt-4">
          {updateQuery.isLoading ? (
            <div className="flex min-h-32 items-center justify-center">
              <Spinner />
            </div>
          ) : updateQuery.isError ? (
            <div className="flex min-h-32 flex-col items-center justify-center gap-3 text-center">
              <p className="text-xs text-muted-foreground">
                {updateQuery.error.message || "无法获取上游版本信息"}
              </p>
              <Button type="button" variant="outline" size="sm" onClick={() => updateQuery.refetch()}>
                <RefreshCw data-icon="inline-start" />
                重新检查
              </Button>
            </div>
          ) : update ? (
            <>
              <div>
                <InfoRow label="当前版本" value={update.current_version} />
                <InfoRow label="最新版本" value={update.latest_version} />
                {update.build_time && <InfoRow label="构建时间" value={update.build_time} />}
                <InfoRow label="运行平台" value={update.platform} />
                <InfoRow label="上游仓库" value={update.upstream_repository} />
                <InfoRow label="Release 资产" value={update.asset_name} />
              </div>

              {update.release_notes && (
                <div className="flex flex-col gap-2 rounded-md border border-dashed p-3">
                  <p className="text-xs font-medium">更新说明</p>
                  <p className="max-h-40 overflow-y-auto whitespace-pre-wrap text-xs leading-relaxed text-muted-foreground">
                    {update.release_notes}
                  </p>
                </div>
              )}

              <div className="flex flex-wrap justify-end gap-2">
                <Button type="button" variant="ghost" size="sm" onClick={() => updateQuery.refetch()} disabled={updateQuery.isFetching}>
                  {updateQuery.isFetching ? <Spinner data-icon="inline-start" /> : <RefreshCw data-icon="inline-start" />}
                  检查更新
                </Button>
                {update.release_url && (
                  <Button asChild variant="outline" size="sm">
                    <a href={update.release_url} target="_blank" rel="noreferrer">
                      <ExternalLink data-icon="inline-start" />
                      查看 Release
                    </a>
                  </Button>
                )}
                <AlertDialog>
                  <AlertDialogTrigger asChild>
                    <Button type="button" size="sm" disabled={!update.can_upgrade || applyUpdateMutation.isPending}>
                      {applyUpdateMutation.isPending && <Spinner data-icon="inline-start" />}
                      立即升级
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>升级到 {update.latest_version}？</AlertDialogTitle>
                      <AlertDialogDescription>
                        服务将下载并校验 {update.asset_name}，随后替换当前二进制并重启。请确保安装目录可写，且服务允许原地重启。
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel>取消</AlertDialogCancel>
                      <AlertDialogAction onClick={() => applyUpdateMutation.mutate()}>
                        确认升级
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              </div>

              {!update.can_upgrade && (
                <p className="text-xs text-muted-foreground">
                  {update.current_version === "dev"
                    ? "开发构建没有可比较的 Release 版本，不能执行自动升级。"
                    : update.update_available
                      ? "当前平台暂不支持自动替换二进制，请从 Release 页面手动升级。"
                      : "当前版本无需升级。"}
                </p>
              )}
            </>
          ) : null}
        </CardContent>
      </Card>
    </div>
  )
}
