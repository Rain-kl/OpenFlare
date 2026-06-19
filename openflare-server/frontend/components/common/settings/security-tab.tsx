"use client"

import {useEffect, useState} from "react"
import {useMutation, useQuery, useQueryClient, type UseQueryResult} from "@tanstack/react-query"
import {
  Clock,
  Fingerprint,
  Globe,
  Loader2,
  Lock,
  Mail,
  Pencil,
  Plus,
  Settings,
  Shield,
  Trash2,
  UserPlus
} from "lucide-react"

import {Button} from "@/components/ui/button"
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card"
import {Switch} from "@/components/ui/switch"
import {Input} from "@/components/ui/input"
import {Label} from "@/components/ui/label"
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from "@/components/ui/select"
import {AuthSourceModal} from "@/components/common/settings/auth-source-modal"
import services from "@/lib/services"
import type {AuthSource, SystemConfig} from "@/lib/services/admin"
import {toast} from "sonner"

const SECURITY_KEYS = [
  {
    key: "password_login_enabled",
    title: "允许密码登录",
    description: "关闭后仅保留第三方 OIDC 认证源进行系统登录。",
    icon: Lock,
  },
  {
    key: "registration_enabled",
    title: "允许注册",
    description: "关闭后系统将禁止新用户进行自主账号注册。",
    icon: UserPlus,
  },
  {
    key: "password_register_enabled",
    title: "允许密码注册",
    description: "关闭后只能通过管理员创建或第三方认证关联建号。",
    icon: Fingerprint,
  },
  {
    key: "oidc_login_enabled",
    title: "允许 OIDC 登录",
    description: "关闭后所有的第三方 OIDC 认证登录入口都会被隐藏。",
    icon: Globe,
  },
  {
    key: "email_login_verification_enabled",
    title: "邮箱登录验证",
    description: "开启后，使用账号密码登录时需要通过邮箱接收并验证 6 位验证码。",
    icon: Mail,
  },
  {
    key: "email_register_verification_enabled",
    title: "邮箱注册验证",
    description: "开启后，用户注册账号时需要通过邮箱接收并验证 6 位验证码。",
    icon: Mail,
  },
] as const

interface SecurityTabProps {
  configs: Record<string, SystemConfig>
  systemConfigsQuery: UseQueryResult<SystemConfig[], Error>
}

export function SecurityTab({ configs, systemConfigsQuery }: SecurityTabProps) {
  const queryClient = useQueryClient()
  const [authSourceModalOpen, setAuthSourceModalOpen] = useState(false)
  const [selectedSource, setSelectedSource] = useState<AuthSource | null>(null)

  const [capCount, setCapCount] = useState("")
  const [capDifficulty, setCapDifficulty] = useState("")
  const [capSize, setCapSize] = useState("")
  const [capTTL, setCapTTL] = useState("")
  const [capTokenTTL, setCapTokenTTL] = useState("")
  const [capAutoSolve, setCapAutoSolve] = useState(true)

  const [sessionTTL, setSessionTTL] = useState("168")
  const [customHours, setCustomHours] = useState("")

  const authSourcesQuery = useQuery({
    queryKey: ["auth", "sources"],
    queryFn: () => services.adminAuthSource.listAuthSources(),
  })

  useEffect(() => {
    if (systemConfigsQuery.data) {
      const cfgMap = configs
      setCapCount(cfgMap["cap_challenge_count"]?.value || "1")
      setCapDifficulty(cfgMap["cap_challenge_difficulty"]?.value || "4")
      setCapSize(cfgMap["cap_challenge_size"]?.value || "32")
      setCapTTL(cfgMap["cap_challenge_ttl_seconds"]?.value || "600")
      setCapTokenTTL(cfgMap["cap_token_ttl_seconds"]?.value || "1200")
      setCapAutoSolve(cfgMap["cap_auto_solve"]?.value !== "false")

      // 初始化登录保持设置
      const ttlVal = cfgMap["login_session_ttl_hours"]?.value || "0"
      if (ttlVal === "0" || ttlVal === "168" || ttlVal === "720" || ttlVal === "-1") {
        setSessionTTL(ttlVal)
        setCustomHours("")
      } else {
        setSessionTTL("custom")
        setCustomHours(ttlVal)
      }
    }
  }, [systemConfigsQuery.data, configs])

  const updateTTLMutation = useMutation({
    mutationFn: async (value: string) => {
      const config = configs["login_session_ttl_hours"]
      if (!config) {
        throw new Error("缺少配置项: login_session_ttl_hours")
      }
      await services.adminSystemConfig.updateSystemConfig("login_session_ttl_hours", {
        value: value,
        description: config.description,
      })
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["admin", "system-configs"] })
      toast.success("登录状态保持时间已更新")
    },
    onError: (error: Error) => {
      toast.error(error.message || "更新配置失败")
    },
  })

  const handleTTLChange = (val: string) => {
    setSessionTTL(val)
    if (val !== "custom") {
      updateTTLMutation.mutate(val)
    }
  }

  const handleCustomBlur = () => {
    const parsed = parseInt(customHours, 10)
    if (isNaN(parsed) || parsed <= 0) {
      toast.error("请输入有效的过期小时数（正整数）")
      // 重置为原本的值
      const originalVal = configs["login_session_ttl_hours"]?.value || "0"
      setCustomHours(originalVal === "custom" || ["0", "168", "720", "-1"].includes(originalVal) ? "" : originalVal)
      return
    }
    updateTTLMutation.mutate(parsed.toString())
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
      toast.success("系统安全配置已更新")
    },
    onError: (error: Error) => {
      toast.error(error.message || "更新配置失败")
    },
  })

  const toggleSourceMutation = useMutation({
    mutationFn: async (source: AuthSource) => {
      await services.adminAuthSource.toggleAuthSource(source.id, { is_active: !source.is_active })
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["auth", "sources"] })
      await queryClient.invalidateQueries({ queryKey: ["auth", "public-sources"] })
      toast.success("认证源状态已更新")
    },
    onError: (error: Error) => {
      toast.error(error.message || "切换状态失败")
    },
  })

  const deleteSourceMutation = useMutation({
    mutationFn: async (sourceId: string) => {
      await services.adminAuthSource.deleteAuthSource(sourceId)
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["auth", "sources"] })
      await queryClient.invalidateQueries({ queryKey: ["auth", "public-sources"] })
      toast.success("认证源已删除")
    },
    onError: (error: Error) => {
      toast.error(error.message || "删除认证源失败")
    },
  })

  const handleToggle = (key: string, checked: boolean) => {
    updateConfigMutation.mutate({ key, value: checked })
  }

  const saveCapMutation = useMutation({
    mutationFn: async () => {
      const updates = [
        { key: "cap_challenge_count", value: capCount },
        { key: "cap_challenge_difficulty", value: capDifficulty },
        { key: "cap_challenge_size", value: capSize },
        { key: "cap_challenge_ttl_seconds", value: capTTL },
        { key: "cap_token_ttl_seconds", value: capTokenTTL },
        { key: "cap_auto_solve", value: capAutoSolve ? "true" : "false" },
      ]

      for (const update of updates) {
        const currentCfg = configs[update.key]
        await services.adminSystemConfig.updateSystemConfig(update.key, {
          value: update.value,
          description: currentCfg?.description || "",
        })
      }
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["admin", "system-configs"] })
      toast.success("人机验证配置已成功保存")
    },
    onError: (error: Error) => {
      toast.error(error.message || "保存配置失败")
    },
  })

  const handleCapSave = (e: React.FormEvent) => {
    e.preventDefault()
    saveCapMutation.mutate()
  }

  return (
    <div className="space-y-6">
      {/* 系统登录与注册控制 */}
      <Card className="border border-dashed shadow-sm">
        <CardHeader className="border-b border-dashed pb-4">
          <div className="flex items-center gap-2">
            <div className="p-1.5 rounded-lg bg-indigo-500/10 text-indigo-500">
              <Settings className="size-4" />
            </div>
            <div>
              <CardTitle className="text-base font-semibold">系统登录/注册设置</CardTitle>
              <CardDescription className="text-xs">配置系统的登录限制与用户自主注册权限</CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className="pt-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {SECURITY_KEYS.map((item) => {
              const config = configs[item.key]
              const checked = config ? config.value === "true" : false
              const Icon = item.icon
              return (
                <div
                  key={item.key}
                  className="flex items-center justify-between gap-4 rounded-xl border border-dashed p-4 bg-card hover:bg-muted/10 hover:border-indigo-500/30 transition-all duration-300 shadow-sm"
                >
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      {Icon && <Icon className="size-4 text-indigo-500" />}
                      <span className="font-medium text-sm text-foreground">{item.title}</span>
                    </div>
                    <p className="text-xs text-muted-foreground leading-relaxed pr-2">{item.description}</p>
                  </div>
                  <Switch
                    checked={checked}
                    disabled={updateConfigMutation.isPending}
                    onCheckedChange={(value) => handleToggle(item.key, value)}
                  />
                </div>
              )
            })}

            {/* 登录状态保持时间 (选择后立即更改) */}
            <div
              className="flex items-center justify-between gap-4 rounded-xl border border-dashed p-4 bg-card hover:bg-muted/10 hover:border-indigo-500/30 transition-all duration-300 shadow-sm md:col-span-2"
            >
              <div className="space-y-1 pr-4">
                <div className="flex items-center gap-2">
                  <Clock className="size-4 text-indigo-500" />
                  <span className="font-medium text-sm text-foreground">登录状态保持时间</span>
                </div>
                <p className="text-xs text-muted-foreground leading-relaxed pr-2">
                  配置用户登录会话在浏览器中的保持期限。设置为“关闭”则在浏览器关闭后自动退登。
                </p>
              </div>

              <div className="flex items-center gap-2 flex-shrink-0">
                <Select
                  value={sessionTTL}
                  disabled={updateTTLMutation.isPending}
                  onValueChange={handleTTLChange}
                >
                  <SelectTrigger className="w-[180px] bg-card border-dashed text-xs h-8">
                    <SelectValue placeholder="选择保留时间" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="0">关闭 (浏览器关闭自动退登)</SelectItem>
                    <SelectItem value="168">7 天</SelectItem>
                    <SelectItem value="720">30 天</SelectItem>
                    <SelectItem value="-1">永不过期</SelectItem>
                    <SelectItem value="custom">自定义时长</SelectItem>
                  </SelectContent>
                </Select>

                {sessionTTL === "custom" && (
                  <Input
                    type="number"
                    min={1}
                    value={customHours}
                    onChange={(e) => setCustomHours(e.target.value)}
                    onBlur={handleCustomBlur}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") {
                        handleCustomBlur()
                      }
                    }}
                    placeholder="小时"
                    disabled={updateTTLMutation.isPending}
                    className="w-20 bg-card border-dashed text-xs h-8 px-2"
                  />
                )}
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* 认证源配置管理 */}
      <Card className="border border-dashed shadow-sm">
        <CardHeader className="border-b border-dashed pb-4 flex flex-row items-center justify-between gap-4">
          <div className="flex items-center gap-2">
            <div className="p-1.5 rounded-lg bg-indigo-500/10 text-indigo-500">
              <Globe className="size-4" />
            </div>
            <div>
              <CardTitle className="text-base font-semibold">认证源管理</CardTitle>
              <CardDescription className="text-xs">添加、修改并启用系统自定义的 OIDC 认证源</CardDescription>
            </div>
          </div>
          <Button
            type="button"
            size="sm"
            onClick={() => {
              setSelectedSource(null)
              setAuthSourceModalOpen(true)
            }}
            variant="secondary"
          >
            <Plus className="mr-1.5 size-3.5" />
            新增认证源
          </Button>
        </CardHeader>
        <CardContent className="pt-6 space-y-3">
          {authSourcesQuery.isPending ? (
            <div className="flex items-center justify-center p-8">
              <Loader2 className="size-6 animate-spin text-muted-foreground/50" />
            </div>
          ) : (authSourcesQuery.data ?? []).length > 0 ? (
            (authSourcesQuery.data ?? []).map((source) => (
              <div
                key={source.id}
                className="flex items-center justify-between rounded-xl border border-dashed p-4 bg-card hover:bg-muted/10 transition-all duration-300 shadow-sm"
              >
                <div className="space-y-1.5">
                  <div className="flex items-center gap-2">
                    <span className="font-semibold text-sm text-foreground">{source.display_name || source.name}</span>
                    <span className={`text-[10px] px-2 py-0.5 rounded-full border font-medium ${
                      source.is_active
                        ? "bg-emerald-500/10 text-emerald-500 border-emerald-500/20"
                        : "bg-amber-500/10 text-amber-500 border-amber-500/20"
                    }`}>
                      {source.is_active ? "已启用" : "已禁用"}
                    </span>
                  </div>
                  <div className="text-xs text-muted-foreground font-mono">
                    标识符: {source.name} · 类型: {source.type.toUpperCase()}
                  </div>
                </div>
                <div className="flex items-center gap-4">
                  <span className={`text-xs px-2.5 py-1 rounded-lg border font-medium hidden sm:inline-block ${
                    source.client_secret_configured
                      ? "bg-indigo-500/5 text-indigo-500 border-indigo-500/10"
                      : "bg-rose-500/5 text-rose-500 border-rose-500/10"
                  }`}>
                    {source.client_secret_configured ? "Secret 已配置" : "Secret 未配置"}
                  </span>

                  <div className="flex items-center gap-2">
                    <Switch
                      checked={source.is_active}
                      disabled={toggleSourceMutation.isPending}
                      className="scale-90 mr-2"
                      onCheckedChange={() => toggleSourceMutation.mutate(source)}
                    />
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="size-8 text-muted-foreground hover:text-indigo-500 hover:bg-indigo-500/10 rounded-lg transition-colors"
                      onClick={() => {
                        setSelectedSource(source)
                        setAuthSourceModalOpen(true)
                      }}
                    >
                      <Pencil className="size-4" />
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="size-8 text-muted-foreground hover:text-rose-500 hover:bg-rose-500/10 rounded-lg transition-colors"
                      disabled={deleteSourceMutation.isPending}
                      onClick={() => {
                        if (window.confirm(`确定删除认证源「${source.display_name || source.name}」吗？`)) {
                          deleteSourceMutation.mutate(source.id)
                        }
                      }}
                    >
                      <Trash2 className="size-4" />
                    </Button>
                  </div>
                </div>
              </div>
            ))
          ) : (
            <div className="rounded-xl border border-dashed border-border/50 px-4 py-8 text-center text-xs text-muted-foreground bg-muted/5 flex flex-col items-center justify-center gap-3">
              <span>暂无配置的认证源，点击上方按钮新增</span>
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={() => {
                  setSelectedSource(null)
                  setAuthSourceModalOpen(true)
                }}
                className="border-dashed"
              >
                <Plus className="mr-1.5 size-3.5" />
                新增认证源
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      {/* 人机验证配置 (Cap CAPTCHA) */}
      <Card className="border border-dashed shadow-sm">
        <CardHeader className="border-b border-dashed pb-4 flex flex-row items-center justify-between gap-4">
          <div className="flex items-center gap-2">
            <div className="p-1.5 rounded-lg bg-indigo-500/10 text-indigo-500">
              <Shield className="size-4" />
            </div>
            <div>
              <CardTitle className="text-base font-semibold">人机验证配置 (Cap CAPTCHA)</CardTitle>
              <CardDescription className="text-xs">配置基于 Proof-of-Work (PoW) 的无感人机验证，保护系统登录免受暴力破解和撞库攻击</CardDescription>
            </div>
          </div>
          <Switch
            checked={configs["cap_login_enabled"]?.value === "true"}
            disabled={updateConfigMutation.isPending}
            onCheckedChange={(checked) => handleToggle("cap_login_enabled", checked)}
          />
        </CardHeader>
        <CardContent className="pt-6">
          {/* 自动开始计算 Switch */}
          <div className="flex items-center justify-between rounded-xl border border-dashed p-4 bg-card mb-4">
            <div className="space-y-0.5">
              <p className="text-sm font-semibold">打开页面后自动开始计算</p>
            </div>
            <Switch
              checked={capAutoSolve}
              onCheckedChange={setCapAutoSolve}
            />
          </div>
          <form onSubmit={handleCapSave} className="space-y-6">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div className="space-y-1.5">
                <Label htmlFor="cap_challenge_count" className="text-xs font-semibold">难题数量 (Count)</Label>
                <Input
                  id="cap_challenge_count"
                  type="number"
                  min={1}
                  max={100}
                  value={capCount}
                  onChange={(e) => setCapCount(e.target.value)}
                  placeholder="50"
                  className="bg-card border-dashed text-xs"
                />
                <p className="text-[10px] text-muted-foreground leading-normal">客户端需求解的难题总数。默认 1，推荐 1 至 5</p>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="cap_challenge_difficulty" className="text-xs font-semibold">验证难度 (Difficulty)</Label>
                <Input
                  id="cap_challenge_difficulty"
                  type="number"
                  min={1}
                  max={10}
                  value={capDifficulty}
                  onChange={(e) => setCapDifficulty(e.target.value)}
                  placeholder="4"
                  className="bg-card border-dashed text-xs"
                />
                <p className="text-[10px] text-muted-foreground leading-normal">PoW 前缀哈希位数，每加 1 计算时间翻倍。默认 4，推荐 4</p>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="cap_challenge_size" className="text-xs font-semibold">盐值长度 (Size)</Label>
                <Input
                  id="cap_challenge_size"
                  type="number"
                  min={8}
                  max={64}
                  value={capSize}
                  onChange={(e) => setCapSize(e.target.value)}
                  placeholder="32"
                  className="bg-card border-dashed text-xs"
                />
                <p className="text-[10px] text-muted-foreground leading-normal">难题盐值混淆字符长度。默认 32</p>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="cap_challenge_ttl" className="text-xs font-semibold">难题超时时长 (秒)</Label>
                <Input
                  id="cap_challenge_ttl"
                  type="number"
                  min={10}
                  value={capTTL}
                  onChange={(e) => setCapTTL(e.target.value)}
                  placeholder="600"
                  className="bg-card border-dashed text-xs"
                />
                <p className="text-[10px] text-muted-foreground leading-normal">难题有效期限。默认 600 秒 (10 分钟)</p>
              </div>

              <div className="space-y-1.5 sm:col-span-2">
                <Label htmlFor="cap_token_ttl" className="text-xs font-semibold">验证凭证有效时长 (秒)</Label>
                <Input
                  id="cap_token_ttl"
                  type="number"
                  min={10}
                  value={capTokenTTL}
                  onChange={(e) => setCapTokenTTL(e.target.value)}
                  placeholder="1200"
                  className="bg-card border-dashed text-xs"
                />
                <p className="text-[10px] text-muted-foreground leading-normal">PoW 计算求解通过后，签发的登录凭证有效时长。默认 1200 秒 (20 分钟)</p>
              </div>
            </div>

            <div className="flex justify-end pt-4 border-t border-dashed">
              <Button
                type="submit"
                size="sm"
                disabled={saveCapMutation.isPending}
              >
                {saveCapMutation.isPending ? (
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

      <AuthSourceModal
        isOpen={authSourceModalOpen}
        source={selectedSource}
        onClose={() => setAuthSourceModalOpen(false)}
        onChanged={async () => {
          await queryClient.invalidateQueries({ queryKey: ["auth", "sources"] })
          await queryClient.invalidateQueries({ queryKey: ["auth", "public-sources"] })
          await authSourcesQuery.refetch()
        }}
      />
    </div>
  )
}
