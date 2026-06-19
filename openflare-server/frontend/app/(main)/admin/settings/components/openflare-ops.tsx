"use client"

import Link from "next/link"
import {useEffect, useMemo, useState} from "react"
import {useMutation, useQuery, useQueryClient} from "@tanstack/react-query"
import {Copy, ExternalLink, Loader2, RefreshCw, RotateCw, Save, Server, Trash2,} from "lucide-react"
import {toast} from "sonner"

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {Badge} from "@/components/ui/badge"
import {Button} from "@/components/ui/button"
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card"
import {Input} from "@/components/ui/input"
import {Label} from "@/components/ui/label"
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue,} from "@/components/ui/select"
import {Switch} from "@/components/ui/switch"
import {Textarea} from "@/components/ui/textarea"
import {ErrorInline} from "@/components/layout/error"
import {LoadingStateWithBorder} from "@/components/layout/loading"
import type {DatabaseCleanupTarget} from "@/lib/services/openflare"
import {AdminStatusService} from "@/lib/services/admin"
import {NodeService, OptionService, StatusService, UptimeKumaService,} from "@/lib/services/openflare"
import {VersionUpgradeDialog} from "@/app/(main)/components/version-upgrade-dialog"
import {adminUpdateStatusQueryKey, openflarePublicStatusQueryKey,} from "@/lib/hooks/use-openflare-server-upgrade"

import {
  agentOptionEntries,
  buildDiscoveryCommand,
  databaseAutoCleanupEntries,
  defaultOpenFlareOpsFields,
  formatDurationLabel,
  getBrowserOrigin,
  mapOptionsToOpsFields,
  type OpenFlareOpsFields,
  optionsToMap,
  uptimeKumaOptionEntries,
} from "./openflare-ops-utils"
import {UptimeKumaSiteSelectModal} from "./uptimekuma-site-modal"

const optionsQueryKey = ["openflare", "options"] as const

const cleanupTargets: Array<{
  target: DatabaseCleanupTarget
  label: string
  description: string
}> = [
  {
    target: "node_access_logs",
    label: "访问日志",
    description: "清理 node_access_logs，影响访问明细与 IP 汇总。",
  },
  {
    target: "node_metric_snapshots",
    label: "性能快照",
    description: "清理 node_metric_snapshots，影响节点资源趋势。",
  },
  {
    target: "node_request_reports",
    label: "请求聚合",
    description: "清理 node_request_reports，影响请求量与错误量统计。",
  },
]

async function copyText(value: string) {
  await navigator.clipboard.writeText(value)
}

export function OpenFlareOpsSettings() {
  const queryClient = useQueryClient()
  const [fields, setFields] = useState<OpenFlareOpsFields>(defaultOpenFlareOpsFields)
  const [savingSection, setSavingSection] = useState<string | null>(null)
  const [geoIPTestIP, setGeoIPTestIP] = useState("8.8.8.8")
  const [uptimeKumaModalOpen, setUptimeKumaModalOpen] = useState(false)
  const [cleanupTarget, setCleanupTarget] = useState<{
    target: DatabaseCleanupTarget
    label: string
  } | null>(null)
  const [cleanupRetentionDays, setCleanupRetentionDays] = useState("")
  const [versionDialogOpen, setVersionDialogOpen] = useState(false)

  const optionsQuery = useQuery({
    queryKey: optionsQueryKey,
    queryFn: () => OptionService.list(),
  })

  const statusQuery = useQuery({
    queryKey: openflarePublicStatusQueryKey,
    queryFn: () => StatusService.getPublicStatus(),
  })

  const bootstrapQuery = useQuery({
    queryKey: ["openflare", "bootstrap-token"],
    queryFn: () => NodeService.getBootstrapToken(),
  })

  const releaseQuery = useQuery({
    queryKey: adminUpdateStatusQueryKey,
    queryFn: () => AdminStatusService.getUpdateStatus(),
  })

  useEffect(() => {
    if (!optionsQuery.data) return
    const optionMap = optionsToMap(optionsQuery.data)
    const serverAddress =
      optionMap.ServerAddress ||
      statusQuery.data?.server_address ||
      getBrowserOrigin()
    setFields(mapOptionsToOpsFields(optionMap, serverAddress))
  }, [optionsQuery.data, statusQuery.data?.server_address])

  const geoIPMutation = useMutation({
    mutationFn: () => OptionService.lookupGeoIP(fields.GeoIPProvider, geoIPTestIP.trim()),
  })

  const saveMutation = useMutation({
    mutationFn: async ({
      section,
      entries,
    }: {
      section: string
      entries: Array<{ key: string; value: string }>
    }) => {
      setSavingSection(section)
      await OptionService.updateBatch(entries)
    },
    onSuccess: async () => {
      toast.success("OpenFlare 运维设置已保存")
      await queryClient.invalidateQueries({ queryKey: optionsQueryKey })
      setSavingSection(null)
    },
    onError: (error) => {
      setSavingSection(null)
      toast.error(error instanceof Error ? error.message : "保存失败")
    },
  })

  const rotateTokenMutation = useMutation({
    mutationFn: () => NodeService.rotateBootstrapToken(),
    onSuccess: async (data) => {
      toast.success("Discovery Token 已重新生成")
      await queryClient.invalidateQueries({ queryKey: ["openflare", "bootstrap-token"] })
      if (data.discovery_token) {
        try {
          await copyText(data.discovery_token)
        } catch {
          // ignore clipboard errors
        }
      }
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : "Token 轮换失败")
    },
  })

  const syncUptimeKumaMutation = useMutation({
    mutationFn: () => UptimeKumaService.sync(),
    onSuccess: () => toast.success("Uptime Kuma 同步任务已执行"),
    onError: (error) => toast.error(error instanceof Error ? error.message : "同步失败"),
  })

  const cleanupMutation = useMutation({
    mutationFn: (payload: { target: DatabaseCleanupTarget; retention_days?: number }) =>
      OptionService.cleanupDatabase(payload),
    onSuccess: (result) => {
      setCleanupTarget(null)
      setCleanupRetentionDays("")
      toast.success(
        result.delete_all
          ? `已清空${result.target_label}，共删除 ${result.deleted_count} 条。`
          : `已清理${result.target_label}，共删除 ${result.deleted_count} 条。`,
      )
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : "清理失败")
    },
  })

  const discoveryToken = bootstrapQuery.data?.discovery_token ?? ""
  const discoveryCommand = useMemo(() => {
    if (!fields.ServerAddress || !discoveryToken) return ""
    return buildDiscoveryCommand(fields.ServerAddress, discoveryToken)
  }, [discoveryToken, fields.ServerAddress])

  const updateField = <K extends keyof OpenFlareOpsFields>(
    key: K,
    value: OpenFlareOpsFields[K],
  ) => {
    setFields((previous) => ({ ...previous, [key]: value }))
  }

  const saveAgentSettings = () => {
    try {
      saveMutation.mutate({
        section: "agent",
        entries: agentOptionEntries(fields),
      })
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "参数校验失败")
    }
  }

  const saveUptimeKumaSettings = () => {
    try {
      saveMutation.mutate({
        section: "uptimekuma",
        entries: uptimeKumaOptionEntries(fields),
      })
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "参数校验失败")
    }
  }

  const saveDatabaseAutoCleanup = () => {
    try {
      saveMutation.mutate({
        section: "database-auto",
        entries: databaseAutoCleanupEntries(fields),
      })
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "参数校验失败")
    }
  }

  if (optionsQuery.isLoading) {
    return <LoadingStateWithBorder icon={Server} description="加载 OpenFlare 运维设置..." />
  }

  if (optionsQuery.isError) {
    return (
      <ErrorInline
        message={
          optionsQuery.error instanceof Error ? optionsQuery.error.message : "加载失败"
        }
        onRetry={() => void optionsQuery.refetch()}
      />
    )
  }

  return (
    <div className="space-y-6">
      <div className="grid gap-6 xl:grid-cols-2">
        <Card className="border-dashed shadow-none">
          <CardHeader className="flex flex-row items-center justify-between gap-4">
            <div>
              <CardTitle className="text-base">Agent 运行参数</CardTitle>
              <CardDescription>心跳间隔与离线阈值会在下个心跳周期同步到节点。</CardDescription>
            </div>
            <Button size="sm" disabled={savingSection === "agent"} onClick={saveAgentSettings}>
              {savingSection === "agent" ? (
                <Loader2 className="size-4 animate-spin mr-1" />
              ) : (
                <Save className="size-3.5 mr-1" />
              )}
              保存
            </Button>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-4 md:grid-cols-2">
              <FieldInput
                label={`心跳间隔 (${formatDurationLabel(fields.AgentHeartbeatInterval)})`}
                value={fields.AgentHeartbeatInterval}
                type="number"
                onChange={(value) => updateField("AgentHeartbeatInterval", value)}
              />
              <FieldInput
                label={`离线阈值 (${formatDurationLabel(fields.NodeOfflineThreshold)})`}
                value={fields.NodeOfflineThreshold}
                type="number"
                onChange={(value) => updateField("NodeOfflineThreshold", value)}
              />
            </div>
            <ToggleRow
              label="开启 WS 连接升级"
              description="HTTP 心跳成功后尝试升级为 WebSocket，配置发布可即时通知。"
              checked={fields.AgentWebsocketUpgradeEnabled}
              onChange={(value) => updateField("AgentWebsocketUpgradeEnabled", value)}
            />
            <FieldInput
              label="Agent 更新仓库"
              value={fields.AgentUpdateRepo}
              placeholder="Rain-kl/OpenFlare"
              onChange={(value) => updateField("AgentUpdateRepo", value)}
            />
          </CardContent>
        </Card>

        <Card className="border-dashed shadow-none">
          <CardHeader className="flex flex-row items-center justify-between gap-4">
            <div>
              <CardTitle className="text-base">IP 归属解析</CardTitle>
              <CardDescription>
                控制节点地图等场景的 GeoIP 来源；访客访问记录归属地固定使用 MaxMind mmdb。
              </CardDescription>
            </div>
            <Button size="sm" disabled={savingSection === "agent"} onClick={saveAgentSettings}>
              {savingSection === "agent" ? (
                <Loader2 className="size-4 animate-spin mr-1" />
              ) : (
                <Save className="size-3.5 mr-1" />
              )}
              保存
            </Button>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-1.5">
              <Label>归属方式</Label>
              <Select
                value={fields.GeoIPProvider}
                onValueChange={(value) => updateField("GeoIPProvider", value)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="disabled">关闭</SelectItem>
                  <SelectItem value="mmdb">MaxMind mmdb</SelectItem>
                  <SelectItem value="ip-api">ip-api.com</SelectItem>
                  <SelectItem value="geojs">geojs.io</SelectItem>
                  <SelectItem value="ipinfo">ipinfo.io</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-3 rounded-lg border border-dashed p-3 sm:flex-row sm:items-end">
              <FieldInput
                label="测试 IP"
                value={geoIPTestIP}
                onChange={setGeoIPTestIP}
                placeholder="8.8.8.8"
              />
              <Button
                type="button"
                variant="outline"
                disabled={geoIPMutation.isPending}
                onClick={() => geoIPMutation.mutate()}
              >
                {geoIPMutation.isPending ? "查询中..." : "查询归属"}
              </Button>
            </div>
            {geoIPMutation.data ? (
              <div className="grid gap-2 text-sm sm:grid-cols-2">
                <InfoCell label="国家/地区" value={geoIPMutation.data.name || "—"} />
                <InfoCell label="ISO Code" value={geoIPMutation.data.iso_code || "—"} />
              </div>
            ) : null}
          </CardContent>
        </Card>
      </div>

      <Card className="border-dashed shadow-none">
        <CardHeader className="flex flex-row items-center justify-between gap-4">
          <div>
            <CardTitle className="text-base">Discovery Token 与部署</CardTitle>
            <CardDescription>
              新节点首次接入使用 Discovery Token；轮换请前往节点管理或使用下方操作。
            </CardDescription>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" size="sm" asChild>
              <Link href="/nodes">
                <ExternalLink className="size-3.5 mr-1" />
                节点管理
              </Link>
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={rotateTokenMutation.isPending}
              onClick={() => rotateTokenMutation.mutate()}
            >
              <RotateCw className="size-3.5 mr-1" />
              轮换 Token
            </Button>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          <FieldInput
            label="Server URL"
            value={fields.ServerAddress}
            onChange={(value) => updateField("ServerAddress", value)}
            placeholder="https://yourdomain.com"
          />
          <div className="rounded-lg border border-dashed p-3">
            <p className="text-[10px] uppercase tracking-wider text-muted-foreground">
              Discovery Token（只读）
            </p>
            <p className="mt-2 break-all text-sm font-mono">
              {bootstrapQuery.isLoading ? "加载中..." : discoveryToken || "未生成"}
            </p>
          </div>
          <div className="space-y-1.5">
            <Label>一键部署命令</Label>
            <Textarea
              readOnly
              value={discoveryCommand || "请先填写可访问的 Server URL 并获取 Token。"}
              className="min-h-24 font-mono text-xs"
            />
            {discoveryCommand ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => void copyText(discoveryCommand).then(() => toast.success("命令已复制"))}
              >
                <Copy className="size-3.5 mr-1" />
                复制命令
              </Button>
            ) : null}
          </div>
        </CardContent>
      </Card>

      <Card className="border-dashed shadow-none">
        <CardHeader className="flex flex-row items-center justify-between gap-4">
          <div>
            <CardTitle className="text-base">Uptime Kuma 集成</CardTitle>
            <CardDescription>将反代站点差分同步至 Uptime Kuma 监控实例。</CardDescription>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={!fields.UptimeKumaEnabled || syncUptimeKumaMutation.isPending}
              onClick={() => syncUptimeKumaMutation.mutate()}
            >
              <RefreshCw className="size-3.5 mr-1" />
              立即同步
            </Button>
            <Button
              size="sm"
              disabled={savingSection === "uptimekuma"}
              onClick={saveUptimeKumaSettings}
            >
              保存
            </Button>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          <ToggleRow
            label="开启 Uptime Kuma"
            checked={fields.UptimeKumaEnabled}
            onChange={(value) => updateField("UptimeKumaEnabled", value)}
          />
          {fields.UptimeKumaEnabled ? (
            <>
              <div className="grid gap-4 md:grid-cols-2">
                <FieldInput
                  label="Uptime Kuma 地址"
                  value={fields.UptimeKumaUrl}
                  onChange={(value) => updateField("UptimeKumaUrl", value)}
                  placeholder="http://localhost:3001"
                />
                <FieldInput
                  label="用户名"
                  value={fields.UptimeKumaUsername}
                  onChange={(value) => updateField("UptimeKumaUsername", value)}
                />
                <FieldInput
                  label="密码"
                  value={fields.UptimeKumaPassword}
                  type="password"
                  onChange={(value) => updateField("UptimeKumaPassword", value)}
                  placeholder="留空表示不更新"
                />
                <FieldInput
                  label="同步间隔 (分钟)"
                  value={fields.UptimeKumaSyncInterval}
                  type="number"
                  onChange={(value) => updateField("UptimeKumaSyncInterval", value)}
                />
                <div className="space-y-1.5">
                  <Label>监控范围</Label>
                  <Select
                    value={fields.UptimeKumaMonitorScope}
                    onValueChange={(value) => updateField("UptimeKumaMonitorScope", value)}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">全部站点</SelectItem>
                      <SelectItem value="selected">选择站点</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              {fields.UptimeKumaMonitorScope === "selected" ? (
                <div className="rounded-lg border border-dashed p-3">
                  <div className="flex items-center justify-between gap-2">
                    <p className="text-sm font-medium">已选站点</p>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => setUptimeKumaModalOpen(true)}
                    >
                      选择监控站点
                    </Button>
                  </div>
                  <p className="mt-2 break-all text-xs text-muted-foreground">
                    {fields.UptimeKumaSelectedSites
                      ? fields.UptimeKumaSelectedSites.split(",").join(", ")
                      : "未选择任何站点"}
                  </p>
                </div>
              ) : null}
              <div className="grid gap-4 md:grid-cols-2">
                <FieldInput
                  label="检测频率 (秒)"
                  value={fields.UptimeKumaInterval}
                  type="number"
                  onChange={(value) => updateField("UptimeKumaInterval", value)}
                />
                <FieldInput
                  label="重试次数"
                  value={fields.UptimeKumaRetry}
                  type="number"
                  onChange={(value) => updateField("UptimeKumaRetry", value)}
                />
                <FieldInput
                  label="重试间隔 (秒)"
                  value={fields.UptimeKumaRetryInterval}
                  type="number"
                  onChange={(value) => updateField("UptimeKumaRetryInterval", value)}
                />
                <FieldInput
                  label="请求超时 (秒)"
                  value={fields.UptimeKumaTimeout}
                  type="number"
                  onChange={(value) => updateField("UptimeKumaTimeout", value)}
                />
              </div>
            </>
          ) : null}
        </CardContent>
      </Card>

      <div className="grid gap-6 xl:grid-cols-2">
        <Card className="border-dashed shadow-none">
          <CardHeader className="flex flex-row items-center justify-between gap-4">
            <div>
              <CardTitle className="text-base">数据库自动清理</CardTitle>
              <CardDescription>每天凌晨 3 点清理超出保留期的观测数据。</CardDescription>
            </div>
            <Button
              size="sm"
              disabled={savingSection === "database-auto"}
              onClick={saveDatabaseAutoCleanup}
            >
              保存
            </Button>
          </CardHeader>
          <CardContent className="space-y-4">
            <ToggleRow
              label="启用每日自动清理"
              checked={fields.DatabaseAutoCleanupEnabled}
              onChange={(value) => updateField("DatabaseAutoCleanupEnabled", value)}
            />
            <FieldInput
              label="保留天数"
              value={fields.DatabaseAutoCleanupRetentionDays}
              type="number"
              onChange={(value) => updateField("DatabaseAutoCleanupRetentionDays", value)}
            />
          </CardContent>
        </Card>

        <Card className="border-dashed shadow-none">
          <CardHeader>
            <CardTitle className="text-base">手动数据清理</CardTitle>
            <CardDescription>保留天数留空时将删除该类数据的全部历史记录。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {cleanupTargets.map((item) => (
              <div
                key={item.target}
                className="flex items-start justify-between gap-3 rounded-lg border border-dashed p-3"
              >
                <div>
                  <p className="text-sm font-medium">{item.label}</p>
                  <p className="mt-1 text-xs text-muted-foreground">{item.description}</p>
                </div>
                <Button
                  type="button"
                  variant="destructive"
                  size="sm"
                  onClick={() =>
                    setCleanupTarget({ target: item.target, label: item.label })
                  }
                >
                  <Trash2 className="size-3.5 mr-1" />
                  清理
                </Button>
              </div>
            ))}
          </CardContent>
        </Card>
      </div>

      <Card className="border-dashed shadow-none">
        <CardHeader className="flex flex-row items-center justify-between gap-4">
          <div>
            <CardTitle className="text-base">版本信息</CardTitle>
            <CardDescription>
              检查上游 GitHub Release 并升级当前服务。
            </CardDescription>
          </div>
          <Button type="button" size="sm" onClick={() => setVersionDialogOpen(true)}>
            管理升级
          </Button>
        </CardHeader>
        <CardContent className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <InfoCell label="当前版本" value={statusQuery.data?.version ?? releaseQuery.data?.current_version ?? "—"} />
          <InfoCell
            label="最新 Release"
            value={releaseQuery.data?.latest_version ?? "—"}
          />
          <div className="rounded-lg border border-dashed px-3 py-2">
            <p className="text-[10px] uppercase tracking-wider text-muted-foreground">更新状态</p>
            <div className="mt-2">
              {releaseQuery.data?.update_available ? (
                <Badge variant="secondary">有新版本</Badge>
              ) : (
                <Badge variant="outline">已是最新</Badge>
              )}
            </div>
          </div>
          <InfoCell
            label="启动时间"
            value={
              statusQuery.data?.start_time
                ? new Date(statusQuery.data.start_time * 1000).toLocaleString()
                : "—"
            }
          />
        </CardContent>
      </Card>

      <VersionUpgradeDialog
        open={versionDialogOpen}
        onOpenChange={setVersionDialogOpen}
        canUpgrade
      />

      <UptimeKumaSiteSelectModal
        open={uptimeKumaModalOpen}
        selectedSites={
          fields.UptimeKumaSelectedSites
            ? fields.UptimeKumaSelectedSites.split(",")
            : []
        }
        onOpenChange={setUptimeKumaModalOpen}
        onSave={(sites) => updateField("UptimeKumaSelectedSites", sites.join(","))}
      />

      <AlertDialog
        open={cleanupTarget !== null}
        onOpenChange={(open) => !open && setCleanupTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>清理{cleanupTarget?.label}</AlertDialogTitle>
            <AlertDialogDescription>
              输入保留天数后仅删除超出范围的数据；留空则删除全部历史记录，操作不可恢复。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <FieldInput
            label="保留天数"
            value={cleanupRetentionDays}
            type="number"
            onChange={setCleanupRetentionDays}
            placeholder="留空则全部删除"
          />
          <AlertDialogFooter>
            <AlertDialogCancel disabled={cleanupMutation.isPending}>取消</AlertDialogCancel>
            <AlertDialogAction
              disabled={cleanupMutation.isPending}
              onClick={(event) => {
                event.preventDefault()
                if (!cleanupTarget) return
                const trimmed = cleanupRetentionDays.trim()
                if (trimmed !== "") {
                  const retentionDays = Number.parseInt(trimmed, 10)
                  if (Number.isNaN(retentionDays) || retentionDays < 1) {
                    toast.error("保留天数至少为 1 天")
                    return
                  }
                  cleanupMutation.mutate({
                    target: cleanupTarget.target,
                    retention_days: retentionDays,
                  })
                  return
                }
                cleanupMutation.mutate({ target: cleanupTarget.target })
              }}
            >
              {cleanupMutation.isPending ? "清理中..." : "确认清理"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

function FieldInput({
  label,
  value,
  onChange,
  type = "text",
  placeholder,
}: {
  label: string
  value: string
  onChange: (value: string) => void
  type?: string
  placeholder?: string
}) {
  return (
    <div className="space-y-1.5">
      <Label>{label}</Label>
      <Input
        type={type}
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        className="h-9 text-xs"
      />
    </div>
  )
}

function ToggleRow({
  label,
  description,
  checked,
  onChange,
}: {
  label: string
  description?: string
  checked: boolean
  onChange: (value: boolean) => void
}) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-lg border border-dashed px-3 py-2">
      <div>
        <Label className="text-xs">{label}</Label>
        {description ? (
          <p className="mt-0.5 text-[11px] text-muted-foreground">{description}</p>
        ) : null}
      </div>
      <Switch checked={checked} onCheckedChange={onChange} />
    </div>
  )
}

function InfoCell({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-dashed px-3 py-2">
      <p className="text-[10px] uppercase tracking-wider text-muted-foreground">{label}</p>
      <p className="mt-2 text-sm font-medium break-all">{value}</p>
    </div>
  )
}
