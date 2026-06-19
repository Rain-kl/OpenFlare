"use client"

import Link from "next/link"
import {useEffect, useState} from "react"
import {useMutation, useQuery, useQueryClient} from "@tanstack/react-query"
import {ExternalLink, Gauge, Loader2, Save} from "lucide-react"
import {toast} from "sonner"

import {useAuth} from "@/components/providers/auth-provider"
import {EmptyStateWithBorder} from "@/components/layout/empty"
import {ErrorInline} from "@/components/layout/error"
import {LoadingStateWithBorder} from "@/components/layout/loading"
import {Button} from "@/components/ui/button"
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card"
import {Input} from "@/components/ui/input"
import {Label} from "@/components/ui/label"
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue,} from "@/components/ui/select"
import {Switch} from "@/components/ui/switch"
import {ConfigVersionService, OptionService} from "@/lib/services/openflare"

import {
  defaultPerformanceFields,
  entriesFromKeys,
  mapOptionsToFields,
  optionsToMap,
  type PerformanceFields,
  validateCacheFields,
  validateGzipFields,
  validateProxyFields,
  validateRuntimeFields,
} from "./components/performance-utils"

const optionsQueryKey = ["openflare", "options"] as const

export default function PerformancePage() {
  const { user, loading: authLoading } = useAuth()
  const queryClient = useQueryClient()
  const [fields, setFields] = useState<PerformanceFields>(defaultPerformanceFields)
  const [savingSection, setSavingSection] = useState<string | null>(null)

  const optionsQuery = useQuery({
    queryKey: optionsQueryKey,
    queryFn: () => OptionService.list(),
    enabled: !!user?.is_admin,
  })

  const previewQuery = useQuery({
    queryKey: ["openflare", "config-preview"],
    queryFn: () => ConfigVersionService.preview(),
    enabled: !!user?.is_admin,
  })

  useEffect(() => {
    if (!optionsQuery.data) return
    setFields(mapOptionsToFields(optionsToMap(optionsQuery.data)))
  }, [optionsQuery.data])

  const saveMutation = useMutation({
    mutationFn: async ({
      section,
      entries,
      validator,
    }: {
      section: string
      entries: Array<{ key: string; value: string }>
      validator?: (fields: PerformanceFields) => void
    }) => {
      validator?.(fields)
      setSavingSection(section)
      await OptionService.updateBatch(entries)
    },
    onSuccess: async () => {
      toast.success("性能参数已保存")
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: optionsQueryKey }),
        queryClient.invalidateQueries({ queryKey: ["openflare", "config-preview"] }),
        queryClient.invalidateQueries({ queryKey: ["openflare", "config-versions"] }),
      ])
      setSavingSection(null)
    },
    onError: (error) => {
      setSavingSection(null)
      toast.error(error instanceof Error ? error.message : "保存失败")
    },
  })

  const updateField = <K extends keyof PerformanceFields>(
    key: K,
    value: PerformanceFields[K],
  ) => {
    setFields((prev) => ({ ...prev, [key]: value }))
  }

  if (authLoading) {
    return (
      <div className="py-6 px-1">
        <LoadingStateWithBorder icon={Gauge} description="加载权限信息..." />
      </div>
    )
  }

  if (!user?.is_admin) {
    return (
      <div className="py-6 px-1">
        <EmptyStateWithBorder
          icon={Gauge}
          title="权限不足"
          description="只有管理员可以访问性能设置。"
        />
      </div>
    )
  }

  if (optionsQuery.isLoading) {
    return (
      <div className="py-6 px-1">
        <LoadingStateWithBorder icon={Gauge} description="加载性能参数..." />
      </div>
    )
  }

  if (optionsQuery.isError) {
    return (
      <div className="py-6 px-1">
        <ErrorInline
          message={
            optionsQuery.error instanceof Error
              ? optionsQuery.error.message
              : "加载失败"
          }
          onRetry={() => void optionsQuery.refetch()}
        />
      </div>
    )
  }

  return (
    <div className="py-6 px-1 space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="flex items-center gap-2">
          <Gauge className="size-5 text-primary" />
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">性能</h1>
            <p className="text-sm text-muted-foreground">
              集中管理 OpenResty 全局性能参数，保存后进入统一配置发布链路。
            </p>
          </div>
        </div>
        <Button variant="outline" size="sm" asChild>
          <Link href="/config-versions">
            <ExternalLink className="size-3.5 mr-1" />
            查看配置预览
          </Link>
        </Button>
      </div>

      <div className="grid gap-3 sm:grid-cols-3">
        <div className="rounded-lg border border-dashed px-4 py-3">
          <p className="text-[10px] uppercase tracking-wider text-muted-foreground">发布链路</p>
          <p className="mt-2 text-sm font-medium">受管模板渲染</p>
        </div>
        <div className="rounded-lg border border-dashed px-4 py-3">
          <p className="text-[10px] uppercase tracking-wider text-muted-foreground">预览规则数</p>
          <p className="mt-2 text-sm font-semibold">
            {previewQuery.data?.route_count ?? "—"} 条
          </p>
        </div>
        <div className="rounded-lg border border-dashed px-4 py-3">
          <p className="text-[10px] uppercase tracking-wider text-muted-foreground">配置预览</p>
          <p className="mt-2 text-sm text-muted-foreground">
            在配置发布页查看完整 nginx 渲染结果
          </p>
        </div>
      </div>

      <Card className="border-dashed shadow-none">
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle className="text-base">连接与事件</CardTitle>
            <CardDescription>worker、超时、请求体大小等运行参数</CardDescription>
          </div>
          <Button
            size="sm"
            disabled={savingSection === "runtime"}
            onClick={() =>
              saveMutation.mutate({
                section: "runtime",
                validator: validateRuntimeFields,
                entries: entriesFromKeys(fields, [
                  "OpenRestyDefaultServerReturnStatus",
                  "OpenRestyWorkerProcesses",
                  "OpenRestyResolvers",
                  "OpenRestyWorkerConnections",
                  "OpenRestyWorkerRlimitNofile",
                  "OpenRestyEventsUse",
                  "OpenRestyEventsMultiAcceptEnabled",
                  "OpenRestyKeepaliveTimeout",
                  "OpenRestyKeepaliveRequests",
                  "OpenRestyClientHeaderTimeout",
                  "OpenRestyClientBodyTimeout",
                  "OpenRestyClientMaxBodySize",
                  "OpenRestyLargeClientHeaderBuffers",
                  "OpenRestySendTimeout",
                ]),
              })
            }
          >
            {savingSection === "runtime" ? (
              <Loader2 className="size-4 animate-spin mr-1" />
            ) : (
              <Save className="size-3.5 mr-1" />
            )}
            保存
          </Button>
        </CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          <FieldInput label="worker_processes" value={fields.OpenRestyWorkerProcesses} onChange={(v) => updateField("OpenRestyWorkerProcesses", v)} />
          <FieldInput label="worker_connections" value={fields.OpenRestyWorkerConnections} onChange={(v) => updateField("OpenRestyWorkerConnections", v)} type="number" />
          <FieldInput label="worker_rlimit_nofile" value={fields.OpenRestyWorkerRlimitNofile} onChange={(v) => updateField("OpenRestyWorkerRlimitNofile", v)} type="number" />
          <div className="space-y-1.5">
            <Label>events use</Label>
            <Select value={fields.OpenRestyEventsUse} onValueChange={(v) => updateField("OpenRestyEventsUse", v)}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="epoll">epoll</SelectItem>
                <SelectItem value="kqueue">kqueue</SelectItem>
                <SelectItem value="poll">poll</SelectItem>
                <SelectItem value="select">select</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <ToggleRow label="multi_accept" checked={fields.OpenRestyEventsMultiAcceptEnabled} onChange={(v) => updateField("OpenRestyEventsMultiAcceptEnabled", v)} />
          <FieldInput label="keepalive_timeout" value={fields.OpenRestyKeepaliveTimeout} onChange={(v) => updateField("OpenRestyKeepaliveTimeout", v)} type="number" />
          <FieldInput label="client_max_body_size" value={fields.OpenRestyClientMaxBodySize} onChange={(v) => updateField("OpenRestyClientMaxBodySize", v)} />
          <FieldInput label="resolvers" value={fields.OpenRestyResolvers} onChange={(v) => updateField("OpenRestyResolvers", v)} placeholder="1.1.1.1 8.8.8.8" />
          <FieldInput label="default_server_return_status" value={fields.OpenRestyDefaultServerReturnStatus} onChange={(v) => updateField("OpenRestyDefaultServerReturnStatus", v)} type="number" />
        </CardContent>
      </Card>

      <Card className="border-dashed shadow-none">
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle className="text-base">反代缓冲与超时</CardTitle>
            <CardDescription>upstream 连接、缓冲与 WebSocket/HTTP3 开关</CardDescription>
          </div>
          <Button
            size="sm"
            disabled={savingSection === "proxy"}
            onClick={() =>
              saveMutation.mutate({
                section: "proxy",
                validator: validateProxyFields,
                entries: entriesFromKeys(fields, [
                  "OpenRestyProxyConnectTimeout",
                  "OpenRestyProxySendTimeout",
                  "OpenRestyProxyReadTimeout",
                  "OpenRestyWebsocketEnabled",
                  "OpenRestyHTTP3Enabled",
                  "OpenRestyProxyRequestBufferingEnabled",
                  "OpenRestyProxyBufferingEnabled",
                  "OpenRestyProxyBuffers",
                  "OpenRestyProxyBufferSize",
                  "OpenRestyProxyBusyBuffersSize",
                ]),
              })
            }
          >
            {savingSection === "proxy" ? <Loader2 className="size-4 animate-spin mr-1" /> : <Save className="size-3.5 mr-1" />}
            保存
          </Button>
        </CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-2">
          <FieldInput label="proxy_connect_timeout" value={fields.OpenRestyProxyConnectTimeout} onChange={(v) => updateField("OpenRestyProxyConnectTimeout", v)} type="number" />
          <FieldInput label="proxy_read_timeout" value={fields.OpenRestyProxyReadTimeout} onChange={(v) => updateField("OpenRestyProxyReadTimeout", v)} type="number" />
          <ToggleRow label="websocket" checked={fields.OpenRestyWebsocketEnabled} onChange={(v) => updateField("OpenRestyWebsocketEnabled", v)} />
          <ToggleRow label="http3" checked={fields.OpenRestyHTTP3Enabled} onChange={(v) => updateField("OpenRestyHTTP3Enabled", v)} />
          <ToggleRow label="proxy_buffering" checked={fields.OpenRestyProxyBufferingEnabled} onChange={(v) => updateField("OpenRestyProxyBufferingEnabled", v)} />
          <FieldInput label="proxy_buffers" value={fields.OpenRestyProxyBuffers} onChange={(v) => updateField("OpenRestyProxyBuffers", v)} />
        </CardContent>
      </Card>

      <div className="grid gap-6 xl:grid-cols-2">
        <Card className="border-dashed shadow-none">
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle className="text-base">压缩</CardTitle>
            <Button
              size="sm"
              disabled={savingSection === "gzip"}
              onClick={() =>
                saveMutation.mutate({
                  section: "gzip",
                  validator: validateGzipFields,
                  entries: entriesFromKeys(fields, [
                    "OpenRestyGzipEnabled",
                    "OpenRestyGzipMinLength",
                    "OpenRestyGzipCompLevel",
                  ]),
                })
              }
            >
              保存
            </Button>
          </CardHeader>
          <CardContent className="space-y-4">
            <ToggleRow label="gzip" checked={fields.OpenRestyGzipEnabled} onChange={(v) => updateField("OpenRestyGzipEnabled", v)} />
            <FieldInput label="gzip_min_length" value={fields.OpenRestyGzipMinLength} onChange={(v) => updateField("OpenRestyGzipMinLength", v)} type="number" />
            <FieldInput label="gzip_comp_level" value={fields.OpenRestyGzipCompLevel} onChange={(v) => updateField("OpenRestyGzipCompLevel", v)} type="number" />
          </CardContent>
        </Card>

        <Card className="border-dashed shadow-none">
          <CardHeader className="flex flex-row items-center justify-between">
            <div>
              <CardTitle className="text-base">缓存</CardTitle>
              <CardDescription>单节点反代缓存优化场景</CardDescription>
            </div>
            <Button
              size="sm"
              disabled={savingSection === "cache"}
              onClick={() =>
                saveMutation.mutate({
                  section: "cache",
                  validator: validateCacheFields,
                  entries: entriesFromKeys(fields, [
                    "OpenRestyCacheEnabled",
                    "OpenRestyCachePath",
                    "OpenRestyCacheLevels",
                    "OpenRestyCacheInactive",
                    "OpenRestyCacheMaxSize",
                    "OpenRestyCacheKeyTemplate",
                    "OpenRestyCacheLockEnabled",
                    "OpenRestyCacheLockTimeout",
                    "OpenRestyCacheUseStale",
                  ]),
                })
              }
            >
              保存
            </Button>
          </CardHeader>
          <CardContent className="space-y-4">
            <ToggleRow label="cache_enabled" checked={fields.OpenRestyCacheEnabled} onChange={(v) => updateField("OpenRestyCacheEnabled", v)} />
            <FieldInput label="proxy_cache_path" value={fields.OpenRestyCachePath} onChange={(v) => updateField("OpenRestyCachePath", v)} disabled={!fields.OpenRestyCacheEnabled} />
            <FieldInput label="levels" value={fields.OpenRestyCacheLevels} onChange={(v) => updateField("OpenRestyCacheLevels", v)} disabled={!fields.OpenRestyCacheEnabled} />
            <FieldInput label="max_size" value={fields.OpenRestyCacheMaxSize} onChange={(v) => updateField("OpenRestyCacheMaxSize", v)} disabled={!fields.OpenRestyCacheEnabled} />
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

function FieldInput({
  label,
  value,
  onChange,
  type = "text",
  placeholder,
  disabled,
}: {
  label: string
  value: string
  onChange: (value: string) => void
  type?: string
  placeholder?: string
  disabled?: boolean
}) {
  return (
    <div className="space-y-1.5">
      <Label className="text-xs text-muted-foreground">{label}</Label>
      <Input
        type={type}
        value={value}
        placeholder={placeholder}
        disabled={disabled}
        onChange={(e) => onChange(e.target.value)}
        className="h-9 text-xs"
      />
    </div>
  )
}

function ToggleRow({
  label,
  checked,
  onChange,
}: {
  label: string
  checked: boolean
  onChange: (value: boolean) => void
}) {
  return (
    <div className="flex items-center justify-between rounded-lg border border-dashed px-3 py-2">
      <Label className="text-xs">{label}</Label>
      <Switch checked={checked} onCheckedChange={onChange} />
    </div>
  )
}
