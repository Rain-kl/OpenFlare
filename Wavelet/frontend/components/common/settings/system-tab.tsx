"use client"

import {useEffect, useState} from "react"
import {useMutation, useQueryClient, type UseQueryResult} from "@tanstack/react-query"
import {Globe, Loader2, Mail, Search, Server, Sparkles} from "lucide-react"

import {Button} from "@/components/ui/button"
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card"
import {Input} from "@/components/ui/input"
import {Label} from "@/components/ui/label"
import {Switch} from "@/components/ui/switch"
import {Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle} from "@/components/ui/dialog"
import {Badge} from "@/components/ui/badge"
import services from "@/lib/services"
import type {SystemConfig} from "@/lib/services/admin"
import {toast} from "sonner"

interface SystemTabProps {
  configs: Record<string, SystemConfig>
  systemConfigsQuery: UseQueryResult<SystemConfig[], Error>
}

export function SystemTab({ configs, systemConfigsQuery }: SystemTabProps) {
  const queryClient = useQueryClient()
  const [serverAddress, setServerAddress] = useState("")
  const [siteName, setSiteName] = useState("")
  const [smtpHost, setSmtpHost] = useState("")
  const [smtpPort, setSmtpPort] = useState("")
  const [smtpUsername, setSmtpUsername] = useState("")
  const [smtpPassword, setSmtpPassword] = useState("")
  const [smtpTestOpen, setSmtpTestOpen] = useState(false)
  const [smtpTestTo, setSmtpTestTo] = useState("")
  const [smtpTestLog, setSmtpTestLog] = useState("")
  const [smtpTestSuccess, setSmtpTestSuccess] = useState<boolean | null>(null)
  const [smtpTestError, setSmtpTestError] = useState("")

  useEffect(() => {
    if (systemConfigsQuery.data) {
      setServerAddress(configs["server_address"]?.value || "")
      setSiteName(configs["site_name"]?.value || "")
      setSmtpHost(configs["smtp_host"]?.value || "")
      setSmtpPort(configs["smtp_port"]?.value || "587")
      setSmtpUsername(configs["smtp_username"]?.value || "")
      setSmtpPassword(configs["smtp_password"]?.value || "")
    }
  }, [systemConfigsQuery.data, configs])

  const handleDetectAddress = () => {
    if (typeof window !== "undefined") {
      setServerAddress(window.location.origin)
      toast.success("已自动获取当前域名并填充")
    }
  }

  const updateConfigMutation = useMutation({
    mutationFn: async ({ key, value }: { key: string; value: boolean }) => {
      const config = configs[key]
      if (!config) {
        throw new Error(`缺少配置项: ${key}`)
      }
      await services.adminSystemConfig.updateSystemConfig(key, {
        value: value ? "true" : "false",
        description: config.description,
      })
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["admin", "system-configs"] })
      await queryClient.invalidateQueries({ queryKey: ["public-config"] })
      toast.success("配置已更新")
    },
    onError: (error: Error) => {
      toast.error(error.message || "更新配置失败")
    },
  })

  const saveSystemMutation = useMutation({
    mutationFn: async () => {
      const currentAddrCfg = configs["server_address"]
      const currentSiteCfg = configs["site_name"]
      await Promise.all([
        services.adminSystemConfig.updateSystemConfig("server_address", {
          value: serverAddress,
          description: currentAddrCfg?.description || "服务器地址",
        }),
        services.adminSystemConfig.updateSystemConfig("site_name", {
          value: siteName,
          description: currentSiteCfg?.description || "系统平台的展示名称",
        }),
      ])
    },
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["admin", "system-configs"] }),
        queryClient.invalidateQueries({ queryKey: ["public-config"] }),
      ])
      toast.success("通用配置已成功保存")
    },
    onError: (error: Error) => {
      toast.error(error.message || "保存配置失败")
    },
  })

  const handleSystemSave = (e: React.FormEvent) => {
    e.preventDefault()
    saveSystemMutation.mutate()
  }

  const saveSmtpMutation = useMutation({
    mutationFn: async () => {
      const updates = [
        { key: "smtp_host", value: smtpHost },
        { key: "smtp_port", value: smtpPort },
        { key: "smtp_username", value: smtpUsername },
        { key: "smtp_password", value: smtpPassword },
      ]

      for (const update of updates) {
        const currentCfg = configs[update.key]
        if (update.key === "smtp_password" && (update.value === "" || update.value === "******")) {
          // If already configured and sent empty or mask, skip updating it (keep existing)
          if (currentCfg && currentCfg.value === "******") {
            continue
          }
        }
        await services.adminSystemConfig.updateSystemConfig(update.key, {
          value: update.value,
          description: currentCfg?.description || "",
        })
      }
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["admin", "system-configs"] })
      toast.success("SMTP 邮件配置已成功保存")
    },
    onError: (error: Error) => {
      toast.error(error.message || "保存配置失败")
    },
  })

  const handleSmtpSave = (e: React.FormEvent) => {
    e.preventDefault()
    saveSmtpMutation.mutate()
  }

  const testSmtpMutation = useMutation({
    mutationFn: async () => {
      setSmtpTestLog("正在发起连接测试...\n")
      setSmtpTestSuccess(null)
      setSmtpTestError("")

      const res = await services.adminSystemConfig.testSMTP({
        smtp_host: smtpHost,
        smtp_port: parseInt(smtpPort, 10) || 587,
        smtp_username: smtpUsername,
        smtp_password: smtpPassword,
        to: smtpTestTo,
      })
      return res
    },
    onSuccess: (data) => {
      setSmtpTestLog(data.log)
      if (data.success) {
        setSmtpTestSuccess(true)
        toast.success("测试邮件发送成功")
      } else {
        setSmtpTestSuccess(false)
        setSmtpTestError(data.error || "发送失败，请检查配置和日志。")
        toast.error("测试邮件发送失败")
      }
    },
    onError: (error: Error) => {
      setSmtpTestSuccess(false)
      setSmtpTestError(error.message || "请求发送失败")
      setSmtpTestLog((prev) => prev + `\n[请求错误] ${error.message}\n`)
      toast.error(error.message || "测试请求发送失败")
    },
  })

  const handleSmtpTestSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!smtpTestTo) {
      toast.error("请输入目标邮箱地址")
      return
    }
    testSmtpMutation.mutate()
  }

  const indexingEnabled = configs["search_engine_indexing_enabled"]?.value === "true"

  return (
    <div className="space-y-8">
      {/* 通用设置 */}
      <Card className="border border-dashed shadow-sm">
        <CardHeader className="border-b border-dashed pb-4">
          <div className="flex items-center gap-2">
            <div className="rounded-lg bg-indigo-500/10 p-1.5 text-indigo-500">
              <Server className="size-4" />
            </div>
            <div>
              <CardTitle className="text-base font-semibold">通用设置</CardTitle>
              <CardDescription className="text-xs">
                配置站点标识、服务地址与搜索引擎收录策略
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className="pt-6">
          <form onSubmit={handleSystemSave} className="flex flex-col gap-6">
            <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="site_name" className="text-xs font-semibold">
                  站点名称
                </Label>
                <Input
                  id="site_name"
                  type="text"
                  value={siteName}
                  onChange={(e) => setSiteName(e.target.value)}
                  placeholder="例如: OpenFlare"
                  className="border-dashed bg-card text-xs"
                />
                <p className="text-[11px] leading-relaxed text-muted-foreground">
                  用于网页标题、登录注册页面、管理后台与系统邮件。
                </p>
              </div>

              <div className="flex flex-col gap-1.5">
                <div className="flex items-center justify-between gap-3">
                  <Label htmlFor="server_address" className="text-xs font-semibold">
                    服务器访问地址
                  </Label>
                  <Button
                    type="button"
                    variant="link"
                    size="sm"
                    onClick={handleDetectAddress}
                    className="h-auto px-0 text-xs"
                  >
                    <Sparkles data-icon="inline-start" />
                    使用当前域名
                  </Button>
                </div>
                <Input
                  id="server_address"
                  type="url"
                  value={serverAddress}
                  onChange={(e) => setServerAddress(e.target.value)}
                  placeholder="例如: https://example.com"
                  className="border-dashed bg-card text-xs"
                />
                <p className="text-[11px] leading-relaxed text-muted-foreground">
                  限定 API 的允许来源；留空将开放任意来源访问。
                </p>
              </div>
            </div>

            <div className="flex flex-col gap-3 rounded-lg border border-dashed p-3 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex min-w-0 items-start gap-3">
                <div className="rounded-md bg-muted p-1.5 text-muted-foreground">
                  <Search className="size-4" />
                </div>
                <div className="flex min-w-0 flex-col gap-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-xs font-semibold">搜索引擎抓取与收录</span>
                    <Badge variant={indexingEnabled ? "default" : "secondary"}>
                      {indexingEnabled ? "允许收录" : "禁止收录"}
                    </Badge>
                  </div>
                  <p className="text-[11px] leading-relaxed text-muted-foreground">
                    控制 Google、Baidu、Bing 等搜索引擎是否可以索引公开页面。
                  </p>
                </div>
              </div>

              <Switch
                aria-label="允许搜索引擎抓取与收录"
                checked={indexingEnabled}
                disabled={updateConfigMutation.isPending || systemConfigsQuery.isPending}
                onCheckedChange={(checked) =>
                  updateConfigMutation.mutate({ key: "search_engine_indexing_enabled", value: checked })
                }
              />
            </div>

            <div className="flex flex-col-reverse gap-2 border-t border-dashed pt-4 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex items-center gap-2 text-[11px] text-muted-foreground">
                <Globe className="size-3.5" />
                地址变更会影响跨域访问策略，请填写完整协议与域名。
              </div>
              <Button
                type="submit"
                size="sm"
                disabled={saveSystemMutation.isPending}
              >
                {saveSystemMutation.isPending ? (
                  <>
                    <Loader2 data-icon="inline-start" className="animate-spin" />
                    保存中...
                  </>
                ) : (
                  "保存配置"
                )}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      {/* SMTP 邮件设置 */}
      <Card className="border border-dashed shadow-sm">
        <CardHeader className="border-b border-dashed pb-4">
          <div className="flex items-center gap-2">
            <div className="p-1.5 rounded-lg bg-indigo-500/10 text-indigo-500">
              <Mail className="size-4" />
            </div>
            <div>
              <CardTitle className="text-base font-semibold">SMTP 邮件设置</CardTitle>
              <CardDescription className="text-xs">配置系统的邮件发送服务 (SMTP)</CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className="pt-6">
          <form onSubmit={handleSmtpSave} className="space-y-6">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div className="space-y-1.5">
                <Label htmlFor="smtp_host" className="text-xs font-semibold">SMTP 服务器地址</Label>
                <Input
                  id="smtp_host"
                  type="text"
                  value={smtpHost}
                  onChange={(e) => setSmtpHost(e.target.value)}
                  placeholder="例如: smtp.example.com"
                  className="bg-card border-dashed text-xs"
                />
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="smtp_port" className="text-xs font-semibold">SMTP 端口</Label>
                <Input
                  id="smtp_port"
                  type="number"
                  value={smtpPort}
                  onChange={(e) => setSmtpPort(e.target.value)}
                  placeholder="例如: 587 或 465"
                  className="bg-card border-dashed text-xs"
                />
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="smtp_username" className="text-xs font-semibold">SMTP 账户</Label>
                <Input
                  id="smtp_username"
                  type="text"
                  value={smtpUsername}
                  onChange={(e) => setSmtpUsername(e.target.value)}
                  placeholder="例如: sender@example.com"
                  className="bg-card border-dashed text-xs"
                />
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="smtp_password" className="text-xs font-semibold">SMTP 访问凭证</Label>
                <Input
                  id="smtp_password"
                  type="password"
                  value={smtpPassword}
                  onChange={(e) => setSmtpPassword(e.target.value)}
                  placeholder={configs["smtp_password"]?.value === "******" ? "•••••• (已配置，留空或输入新值)" : "输入凭证密码"}
                  className="bg-card border-dashed text-xs"
                />
              </div>
            </div>

            <div className="flex justify-end gap-2 pt-4 border-t border-dashed">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => {
                  setSmtpTestOpen(true)
                  setSmtpTestTo("")
                  setSmtpTestLog("")
                  setSmtpTestSuccess(null)
                  setSmtpTestError("")
                }}
                disabled={saveSmtpMutation.isPending}
              >
                测试发件
              </Button>
              <Button
                type="submit"
                size="sm"
                disabled={saveSmtpMutation.isPending}
              >
                {saveSmtpMutation.isPending ? (
                  <>
                    <Loader2 className="mr-1.5 size-3.5 animate-spin" />
                    保存中...
                  </>
                ) : (
                  "保存配置"
                )}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      <Dialog open={smtpTestOpen} onOpenChange={setSmtpTestOpen}>
        <DialogContent className="max-w-lg border border-dashed">
          <DialogHeader>
            <DialogTitle className="text-base font-semibold">SMTP 发件测试</DialogTitle>
            <DialogDescription className="text-xs">
              输入接收测试邮件的邮箱地址。系统将使用您在表单中当前填写的 SMTP 配置进行发件测试。
            </DialogDescription>
          </DialogHeader>

          <form onSubmit={handleSmtpTestSubmit} className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="smtp_test_to" className="text-xs font-semibold">目标邮箱地址</Label>
              <Input
                id="smtp_test_to"
                type="email"
                required
                value={smtpTestTo}
                onChange={(e) => setSmtpTestTo(e.target.value)}
                placeholder="例如: receiver@example.com"
                className="bg-card border-dashed text-xs"
                disabled={testSmtpMutation.isPending}
              />
            </div>

            {smtpTestLog && (
              <div className="space-y-1.5">
                <Label className="text-xs font-semibold">连接与传输日志</Label>
                <pre className="bg-zinc-950 text-zinc-50 font-mono p-4 rounded-lg text-[10px] h-60 overflow-y-auto whitespace-pre-wrap border border-dashed border-zinc-800 leading-relaxed">
                  {smtpTestLog}
                </pre>
              </div>
            )}

            {smtpTestSuccess === true && (
              <div className="p-3 rounded-lg border border-dashed border-emerald-500/30 bg-emerald-500/5 text-emerald-500 text-xs">
                测试成功！邮件已顺利发出。
              </div>
            )}

            {smtpTestSuccess === false && (
              <div className="p-3 rounded-lg border border-dashed border-rose-500/30 bg-rose-500/5 text-rose-500 text-xs break-all">
                测试失败：{smtpTestError}
              </div>
            )}

            <DialogFooter className="gap-2 sm:gap-0 border-t border-dashed pt-4">
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => setSmtpTestOpen(false)}
                disabled={testSmtpMutation.isPending}
              >
                关闭
              </Button>
              <Button
                type="submit"
                size="sm"
                disabled={testSmtpMutation.isPending}
              >
                {testSmtpMutation.isPending ? (
                  <>
                    <Loader2 className="mr-1.5 size-3.5 animate-spin" />
                    测试中...
                  </>
                ) : (
                  "开始测试"
                )}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}
