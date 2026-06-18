"use client"

import * as React from "react"
import {useMutation, useQuery, useQueryClient} from "@tanstack/react-query"
import {Database, Loader2, Play, Save} from "lucide-react"
import {toast} from "sonner"

import {Badge} from "@/components/ui/badge"
import {Button} from "@/components/ui/button"
import {Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle} from "@/components/ui/card"
import {Field, FieldDescription, FieldGroup, FieldLabel} from "@/components/ui/field"
import {Input} from "@/components/ui/input"
import {Progress} from "@/components/ui/progress"
import {Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue} from "@/components/ui/select"
import {Switch} from "@/components/ui/switch"
import services from "@/lib/services"
import type {
  ObjectStorageConfig,
  StorageConfig,
  StorageDriver,
  TaskExecution,
  TaskExecutionStatus
} from "@/lib/services/admin/types"

const storageConfigKey = "storage_config"
const storageMigrationTaskType = "storage_migration"

const driverLabels: Record<StorageDriver, string> = {
  local: "本地文件系统",
  s3: "AWS S3",
  r2: "Cloudflare R2",
  minio: "MinIO",
  oss: "阿里云 OSS",
  webdav: "WebDAV",
}

const emptyObjectConfig: ObjectStorageConfig = {
  endpoint: "",
  region: "",
  bucket: "",
  access_key_id: "",
  secret_access_key: "",
  path_style: false,
  key_prefix: "",
  cdn_url: "",
}

function normalizeConfig(config: StorageConfig): StorageConfig {
  const s3 = {...emptyObjectConfig, ...config.s3}
  const r2 = {...emptyObjectConfig, ...config.r2}
  const minio = {...emptyObjectConfig, ...config.minio}
  const oss = {...emptyObjectConfig, ...config.oss}
  const webdav = {...config.webdav}
  return {
    ...config,
    local: {root: config.local?.root || "."},
    s3: {...s3, region: s3.region || "us-east-1"},
    r2: {...r2, region: r2.region || "auto"},
    minio: {...minio, region: minio.region || "us-east-1", path_style: config.minio?.path_style ?? true},
    oss,
    webdav: {
      endpoint: webdav.endpoint || "",
      username: webdav.username || "",
      password: webdav.password || "",
      base_path: webdav.base_path || "",
    },
  }
}

type StorageMigrationView = {
  state: "idle" | TaskExecutionStatus
  id?: string
  task_id?: string
  source_driver?: StorageDriver
  target_driver?: StorageDriver
  error?: string
}

type StorageMigrationPayload = {
  target: StorageConfig
}

function parseMigrationPayload(payload: string): StorageConfig | undefined {
  try {
    const parsed = JSON.parse(payload) as Partial<StorageMigrationPayload>
    return parsed.target ? normalizeConfig(parsed.target) : undefined
  } catch {
    return undefined
  }
}

function driverLabel(driver?: StorageDriver): string {
  return driver ? driverLabels[driver] : "未知"
}

function latestMigration(executions: TaskExecution[], current?: StorageConfig): StorageMigrationView {
  const execution = executions[0]
  if (!execution || execution.status === "succeeded") {
    return {state: "idle"}
  }
  const target = parseMigrationPayload(execution.payload)
  return {
    state: execution.status,
    id: execution.id,
    task_id: execution.task_id,
    source_driver: current?.driver,
    target_driver: target?.driver,
    error: execution.error_message,
  }
}

export function StorageConfigTab() {
  const queryClient = useQueryClient()
  const [config, setConfig] = React.useState<StorageConfig | null>(null)
  const query = useQuery({
    queryKey: ["admin", "storage-config"],
    queryFn: async () => {
      const [configRecord, executions] = await Promise.all([
        services.adminSystemConfig.getSystemConfig(storageConfigKey),
        services.adminTask.listTaskExecutions({task_type: storageMigrationTaskType, page: 1, page_size: 1}),
      ])
      const current = normalizeConfig(JSON.parse(configRecord.value) as StorageConfig)
      const migration = latestMigration(executions.items, current)
      return {
        config: current,
        migration,
      }
    },
    refetchInterval: (result) => {
      const state = result.state.data?.migration.state
      return state === "pending" || state === "running" ? 2000 : false
    },
  })

  React.useEffect(() => {
    if (query.data?.config) {
      setConfig((current) => {
        const normalized = normalizeConfig(query.data.config)
        if (current) {
          return {
            ...normalized,
            driver: current.driver,
          }
        }
        return normalized
      })
    }
  }, [query.data?.config])

  const saveMutation = useMutation({
    mutationFn: async (value: StorageConfig) => {
      await services.adminSystemConfig.updateSystemConfig(storageConfigKey, {
        value: JSON.stringify(value),
      })
    },
    onSuccess: () => {
      toast.success("存储配置已保存")
      void queryClient.invalidateQueries({queryKey: ["admin", "storage-config"]})
    },
    onError: (error: Error) => {
      toast.error(error.message || "保存存储配置失败")
      void queryClient.invalidateQueries({queryKey: ["admin", "storage-config"]})
    },
  })

  const migrateMutation = useMutation({
    mutationFn: async (value: StorageConfig) => {
      const payload: StorageMigrationPayload = {target: value}
      return services.adminTask.dispatchTask({
        task_type: storageMigrationTaskType,
        payload: JSON.stringify(payload),
      })
    },
    onSuccess: () => {
      toast.success("存储迁移任务已下发")
      void queryClient.invalidateQueries({queryKey: ["admin", "storage-config"]})
    },
    onError: (error: Error) => {
      toast.error(error.message || "下发存储迁移任务失败")
      void queryClient.invalidateQueries({queryKey: ["admin", "storage-config"]})
    },
  })
  const runMutation = useMutation({
    mutationFn: (executionID: string) => services.adminTask.retryTaskExecution(executionID),
    onSuccess: () => {
      toast.success("存储迁移任务已重新下发")
      void queryClient.invalidateQueries({queryKey: ["admin", "storage-config"]})
    },
    onError: (error: Error) => toast.error(error.message || "运行存储迁移失败"),
  })

  if (query.isPending || !config) {
    return <div className="flex justify-center py-20"><Loader2 className="animate-spin" /></div>
  }

  const migration = query.data?.migration
  const isReadOnly = migration ? migration.state !== "idle" : false
  const isFormDisabled =
    (migration ? (migration.state === "pending" || migration.state === "running") : false) ||
    saveMutation.isPending ||
    migrateMutation.isPending

  const updateObject = (driver: "s3" | "r2" | "minio" | "oss", patch: Partial<ObjectStorageConfig>) => {
    setConfig((current) => current ? {
      ...current,
      [driver]: {...current[driver], ...patch},
    } : current)
  }

  return (
    <div className="flex w-full flex-col gap-6">
      {isReadOnly && migration && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              存储维护模式
              <Badge variant={migration.state === "failed" ? "destructive" : "secondary"}>
                {migration.state}
              </Badge>
            </CardTitle>
            <CardDescription>
              {driverLabel(migration.source_driver)} → {driverLabel(migration.target_driver)}。迁移期间文件只允许读取，禁止上传、删除和清理。
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            <Progress value={migration.state === "succeeded" ? 100 : undefined} />
            <p className="text-sm text-muted-foreground">
              迁移进度请查看任务执行日志：{migration.task_id || "尚未下发"}
            </p>
            {migration.error && <p className="text-sm text-destructive">{migration.error}</p>}
          </CardContent>
          <CardFooter>
            <Button
              variant="outline"
              disabled={migration.state !== "failed" || !migration.id || runMutation.isPending}
              onClick={() => migration.id && runMutation.mutate(migration.id)}
            >
              {runMutation.isPending ? <Loader2 data-icon="inline-start" className="animate-spin" /> : <Play data-icon="inline-start" />}
              重试迁移
            </Button>
          </CardFooter>
        </Card>
      )}

      <Card className="border border-dashed shadow-sm">
        <CardHeader className="border-b border-dashed pb-4">
          <div className="flex items-center gap-2">
            <div className="rounded-lg bg-indigo-500/10 p-1.5 text-indigo-500">
              <Database className="size-4" />
            </div>
            <div>
              <CardTitle className="text-base font-semibold">文件存储</CardTitle>
              <CardDescription className="text-xs">
                默认使用本地存储。配置系统文件的存储媒介，切换存储类型且已有文件时，系统会自动进入维护模式并迁移文件。
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className="pt-6">
          <FieldGroup className="space-y-6">
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
              <Field className="flex flex-col gap-1.5 md:col-span-2">
                <FieldLabel className="text-xs font-semibold">存储类型</FieldLabel>
                <Select
                  value={config.driver}
                  disabled={isFormDisabled}
                  onValueChange={(value) => setConfig({...config, driver: value as StorageDriver})}
                >
                  <SelectTrigger className="w-full border-dashed bg-card text-xs">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {(Object.keys(driverLabels) as StorageDriver[]).map((driver) => (
                        <SelectItem key={driver} value={driver}>{driverLabels[driver]}</SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>

              {config.driver === "local" && (
                <div className="md:col-span-2">
                  <TextField
                    label="根目录"
                    value={config.local.root}
                    placeholder="."
                    onChange={(root) => setConfig({...config, local: {root}})}
                  />
                </div>
              )}

              {(config.driver === "s3" || config.driver === "r2" || config.driver === "minio" || config.driver === "oss") && (
                <ObjectFields
                  driver={config.driver as "s3" | "r2" | "minio" | "oss"}
                  value={config[config.driver as "s3" | "r2" | "minio" | "oss"]}
                  onChange={(patch) => updateObject(config.driver as "s3" | "r2" | "minio" | "oss", patch)}
                />
              )}

              {config.driver === "webdav" && (
                <>
                  <div className="md:col-span-2">
                    <TextField label="服务地址" value={config.webdav.endpoint} placeholder="https://dav.example.com" onChange={(endpoint) => setConfig({...config, webdav: {...config.webdav, endpoint}})} />
                  </div>
                  <TextField label="用户名" value={config.webdav.username} onChange={(username) => setConfig({...config, webdav: {...config.webdav, username}})} />
                  <TextField label="密码" type="password" value={config.webdav.password} onChange={(password) => setConfig({...config, webdav: {...config.webdav, password}})} />
                  <div className="md:col-span-2">
                    <TextField label="基础路径" value={config.webdav.base_path} placeholder="openflare" onChange={(base_path) => setConfig({...config, webdav: {...config.webdav, base_path}})} />
                  </div>
                </>
              )}
            </div>
          </FieldGroup>
        </CardContent>
        <CardFooter className="justify-end gap-2 flex-col sm:flex-row items-end sm:items-center border-t border-dashed pt-4 mt-6">
          {config.driver !== query.data?.config.driver && (
            <span className="text-xs text-amber-500 mr-auto text-left max-w-md">
              ⚠️ 您已切换存储类型。点击「保存配置」将立即切换活动存储引擎，并同步更新已有文件的存储驱动标记。仅在需要复制物理文件时才使用「开始迁移」。
            </span>
          )}
          <div className="flex gap-2">
            <Button
              variant="outline"
              disabled={isFormDisabled}
              onClick={() => saveMutation.mutate(config)}
              className="border-dashed"
            >
              {saveMutation.isPending ? <Loader2 data-icon="inline-start" className="animate-spin" /> : <Save data-icon="inline-start" />}
              保存配置
            </Button>
            {config.driver !== query.data?.config.driver && (
              <Button
                disabled={isFormDisabled}
                onClick={() => migrateMutation.mutate(config)}
              >
                {migrateMutation.isPending ? <Loader2 data-icon="inline-start" className="animate-spin" /> : <Play data-icon="inline-start" />}
                开始迁移
              </Button>
            )}
          </div>
        </CardFooter>
      </Card>
    </div>
  )
}

function ObjectFields({
  driver,
  value,
  onChange,
}: {
  driver: "s3" | "r2" | "minio" | "oss"
  value: ObjectStorageConfig
  onChange: (patch: Partial<ObjectStorageConfig>) => void
}) {
  return (
    <>
      {driver === "r2" && (
        <div className="md:col-span-2">
          <TextField label="Account ID" value={value.account_id || ""} onChange={(account_id) => onChange({account_id})} />
        </div>
      )}
      {driver !== "s3" && (
        <div className="md:col-span-2">
          <TextField label="Endpoint" value={value.endpoint} placeholder="https://..." onChange={(endpoint) => onChange({endpoint})} />
        </div>
      )}
      <TextField label="Region" value={value.region} onChange={(region) => onChange({region})} />
      <TextField label="Bucket" value={value.bucket} onChange={(bucket) => onChange({bucket})} />
      <TextField label="Access Key ID" value={value.access_key_id} onChange={(access_key_id) => onChange({access_key_id})} />
      <TextField label="Secret Access Key" type="password" value={value.secret_access_key} onChange={(secret_access_key) => onChange({secret_access_key})} />
      <TextField label="对象前缀" value={value.key_prefix} placeholder="uploads" onChange={(key_prefix) => onChange({key_prefix})} />
      <TextField label="CDN 地址" value={value.cdn_url} placeholder="https://cdn.example.com" onChange={(cdn_url) => onChange({cdn_url})} />
      {(driver === "s3" || driver === "minio") && (
        <div className="md:col-span-2">
          <Field orientation="horizontal" className="flex items-center justify-between border border-dashed rounded-lg p-4 bg-muted/30">
            <div className="flex flex-col gap-0.5">
              <FieldLabel className="text-xs font-semibold cursor-pointer">Path Style</FieldLabel>
              <FieldDescription className="text-[11px] leading-relaxed text-muted-foreground">MinIO 等自托管 S3 通常需要开启。</FieldDescription>
            </div>
            <Switch checked={value.path_style} onCheckedChange={(path_style) => onChange({path_style})} />
          </Field>
        </div>
      )}
    </>
  )
}

function TextField({
  label,
  value,
  onChange,
  placeholder,
  type = "text",
}: {
  label: string
  value: string
  onChange: (value: string) => void
  placeholder?: string
  type?: React.HTMLInputTypeAttribute
}) {
  return (
    <Field className="flex flex-col gap-1.5">
      <FieldLabel className="text-xs font-semibold">{label}</FieldLabel>
      <Input
        type={type}
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        className="border-dashed bg-card text-xs"
      />
    </Field>
  )
}
