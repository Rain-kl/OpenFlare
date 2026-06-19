"use client"

import {useMemo} from "react"
import {useMutation, useQuery, useQueryClient, type UseQueryResult} from "@tanstack/react-query"
import {KeyRound, ShieldAlert, X} from "lucide-react"
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card"
import {Badge} from "@/components/ui/badge"
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from "@/components/ui/select"
import services from "@/lib/services"
import type {SystemConfig} from "@/lib/services/admin"
import {TemplatesManager} from "./templates"
import {toast} from "sonner"

interface OperationTabProps {
  configs: Record<string, SystemConfig>
  systemConfigsQuery: UseQueryResult<SystemConfig[], Error>
}

export function OperationTab({ configs, systemConfigsQuery }: OperationTabProps) {
  const queryClient = useQueryClient()

  const uploadTypesQuery = useQuery({
    queryKey: ["admin", "upload-types"],
    queryFn: () => services.adminSystemConfig.listUploadTypes(),
  })

  const updateWhitelistMutation = useMutation({
    mutationFn: async (newValue: string) => {
      const config = configs["file_access_whitelist"]
      if (!config) {
        throw new Error("缺少配置项: file_access_whitelist")
      }
      await services.adminSystemConfig.updateSystemConfig("file_access_whitelist", {
        value: newValue,
        description: config.description,
      })
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["admin", "system-configs"] })
      await queryClient.invalidateQueries({ queryKey: ["public-config"] })
      toast.success("文件访问白名单已更新")
    },
    onError: (error: Error) => {
      toast.error(error.message || "更新白名单失败")
    },
  })

  const whitelistConfig = configs["file_access_whitelist"]
  const currentWhitelist = useMemo<string[]>(() => {
    if (!whitelistConfig?.value) return ["avatar"]
    try {
      const parsed = JSON.parse(whitelistConfig.value)
      if (Array.isArray(parsed)) return parsed
    } catch {
      // 降级支持逗号分隔解析
      return whitelistConfig.value.split(",").map(s => s.trim()).filter(Boolean)
    }
    return ["avatar"]
  }, [whitelistConfig?.value])

  const handleAddType = (type: string) => {
    if (!type || currentWhitelist.includes(type)) return
    const newWhitelist = [...currentWhitelist, type]
    updateWhitelistMutation.mutate(JSON.stringify(newWhitelist))
  }

  const handleRemoveType = (typeToRemove: string) => {
    const newWhitelist = currentWhitelist.filter(t => t !== typeToRemove)
    updateWhitelistMutation.mutate(JSON.stringify(newWhitelist))
  }

  const availableTypes = useMemo(() => {
    const types = uploadTypesQuery.data ?? []
    return types.map(t => {
      let label = t
      if (t === "avatar") label = "头像 (avatar)"
      else if (t === "attachment") label = "附件 (attachment)"
      else if (t === "doc") label = "文档 (doc)"
      else if (t === "generic") label = "通用 (generic)"
      return { value: t, label }
    })
  }, [uploadTypesQuery.data])

  return (
    <div className="space-y-6">

      {/* 文件访问白名单设置 */}
      <Card className="border border-dashed shadow-sm">
        <CardHeader className="border-b border-dashed pb-4">
          <div className="flex items-center gap-2">
            <div className="p-1.5 rounded-lg bg-indigo-500/10 text-indigo-500">
              <KeyRound className="size-4" />
            </div>
            <div>
              <CardTitle className="text-base font-semibold">文件访问权限控制</CardTitle>
              <CardDescription className="text-xs">配置免登录直接访问的文件业务类型。不在白名单内的文件将要求登录鉴权。</CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className="pt-6 space-y-4">
          <div className="flex flex-col gap-4">
            <div className="flex items-center gap-3">
              <span className="text-sm font-medium text-muted-foreground">添加免鉴权类型:</span>
              <Select
                value=""
                onValueChange={handleAddType}
                disabled={updateWhitelistMutation.isPending || systemConfigsQuery.isPending || uploadTypesQuery.isPending}
              >
                <SelectTrigger className="w-[200px]" size="sm">
                  <SelectValue placeholder="选择业务类型..." />
                </SelectTrigger>
                <SelectContent>
                  {availableTypes
                    .filter(t => !currentWhitelist.includes(t.value))
                    .map(t => (
                      <SelectItem key={t.value} value={t.value}>
                        {t.label}
                      </SelectItem>
                    ))}
                  {availableTypes.filter(t => !currentWhitelist.includes(t.value)).length === 0 && (
                    <div className="text-xs text-muted-foreground p-2 text-center">所有类型已添加</div>
                  )}
                </SelectContent>
              </Select>
            </div>

            {/* 当前白名单列表 */}
            <div className="rounded-xl border border-dashed p-4 bg-card hover:bg-muted/10 hover:border-indigo-500/30 transition-all duration-300 shadow-sm space-y-3">
              <div className="flex items-center gap-2">
                <ShieldAlert className="size-4 text-indigo-500" />
                <span className="font-medium text-sm text-foreground">当前免鉴权列表</span>
              </div>

              {currentWhitelist.length > 0 ? (
                <div className="flex flex-wrap gap-2">
                  {currentWhitelist.map(type => (
                    <Badge
                      key={type}
                      variant="secondary"
                      className="px-2.5 py-1 text-xs gap-1.5 flex items-center bg-indigo-500/10 text-indigo-700 dark:text-indigo-300 dark:bg-indigo-500/20 border border-indigo-500/20"
                    >
                      {availableTypes.find(t => t.value === type)?.label || type}
                      <button
                        type="button"
                        onClick={() => handleRemoveType(type)}
                        disabled={updateWhitelistMutation.isPending || systemConfigsQuery.isPending}
                        className="rounded-full outline-hidden hover:bg-indigo-500/20 p-0.5 text-indigo-600 dark:text-indigo-400 cursor-pointer disabled:cursor-not-allowed"
                      >
                        <X className="size-3" />
                      </button>
                    </Badge>
                  ))}
                </div>
              ) : (
                <p className="text-xs text-muted-foreground">
                  白名单已空，所有类型文件的访问都将需要登录。
                </p>
              )}
            </div>
          </div>
        </CardContent>
      </Card>

      {/* 通知模板管理 */}
      <TemplatesManager />
    </div>
  )
}
