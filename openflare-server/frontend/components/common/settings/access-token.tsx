"use client"

import * as React from "react"
import Link from "next/link"
import {useMutation, useQuery, useQueryClient} from "@tanstack/react-query"
import {motion} from "motion/react"
import {AlertTriangle, Check, Copy, Info, Key, Loader2, Plus, RefreshCw, Shield, Trash2} from "lucide-react"

import {Button} from "@/components/ui/button"
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card"
import {Input} from "@/components/ui/input"
import {Label} from "@/components/ui/label"
import {Badge} from "@/components/ui/badge"
import {Switch} from "@/components/ui/switch"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import type {CreateTokenResponse} from "@/lib/services/user"
import {UserService} from "@/lib/services/user"
import {useAuth} from "@/components/providers/auth-provider"
import {toast} from "sonner"

export function AccessTokenMain() {
  const { user } = useAuth()
  const queryClient = useQueryClient()
  const [createDialogOpen, setCreateDialogOpen] = React.useState(false)
  const [viewDialogOpen, setViewDialogOpen] = React.useState(false)
  const [tokenName, setTokenName] = React.useState("")
  const [tokenIsAdmin, setTokenIsAdmin] = React.useState(false)
  const [copiedId, setCopiedId] = React.useState<number | null>(null)
  const [newCreatedToken, setNewCreatedToken] = React.useState<CreateTokenResponse | null>(null)

  // 获取 Token 列表
  const accessTokensQuery = useQuery({
    queryKey: ["user", "access-tokens"],
    queryFn: () => UserService.getAccessTokens(),
  })

  // 创建 Token
  const createTokenMutation = useMutation({
    mutationFn: ({ name, isAdmin }: { name: string; isAdmin: boolean }) => UserService.createAccessToken(name, isAdmin),
    onSuccess: (data) => {
      setNewCreatedToken(data)
      setTokenName("")
      setTokenIsAdmin(false)
      setCreateDialogOpen(false)
      setViewDialogOpen(true)
      void queryClient.invalidateQueries({ queryKey: ["user", "access-tokens"] })
      toast.success("访问令牌创建成功")
    },
    onError: (error: Error) => {
      toast.error(error.message || "创建访问令牌失败")
    },
  })

  // 删除 Token
  const deleteTokenMutation = useMutation({
    mutationFn: (id: number) => UserService.deleteAccessToken(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["user", "access-tokens"] })
      toast.success("访问令牌已撤销")
    },
    onError: (error: Error) => {
      toast.error(error.message || "删除访问令牌失败")
    },
  })

  // 轮换 Token
  const rotateTokenMutation = useMutation({
    mutationFn: (id: number) => UserService.rotateAccessToken(id),
    onSuccess: (data) => {
      setNewCreatedToken(data)
      setViewDialogOpen(true)
      void queryClient.invalidateQueries({ queryKey: ["user", "access-tokens"] })
      toast.success("访问令牌轮换成功，旧密钥已失效")
    },
    onError: (error: Error) => {
      toast.error(error.message || "轮换访问令牌失败")
    },
  })

  const handleCreateToken = (e: React.FormEvent) => {
    e.preventDefault()
    if (!tokenName.trim()) {
      toast.error("请输入令牌名称")
      return
    }
    createTokenMutation.mutate({ name: tokenName.trim(), isAdmin: tokenIsAdmin })
  }

  const handleDeleteToken = (id: number, name: string) => {
    if (window.confirm(`确定要删除并撤销令牌「${name}」吗？删除后此令牌将立即失效且不可恢复。`)) {
      deleteTokenMutation.mutate(id)
    }
  }

  const handleRotateToken = (id: number, name: string) => {
    if (window.confirm(`确定要轮换令牌「${name}」的密钥吗？轮换后系统将生成全新密钥，原令牌密钥将立即失效。`)) {
      rotateTokenMutation.mutate(id)
    }
  }

  const handleCopyText = async (text: string, id: number) => {
    try {
      await navigator.clipboard.writeText(text)
      setCopiedId(id)
      toast.success("复制成功")
      setTimeout(() => setCopiedId(null), 2000)
    } catch {
      toast.error("复制失败")
    }
  }

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return "未使用"
    return new Date(dateStr).toLocaleString("zh-CN", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    })
  }

  return (
    <motion.div
      initial={{ opacity: 0, y: 15 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.35, ease: "easeOut" }}
      className="py-6 space-y-6 max-w-4xl mx-auto px-4"
    >
      <div className="font-semibold">
        <Breadcrumb>
          <BreadcrumbList>
            <BreadcrumbItem>
              <BreadcrumbLink asChild>
                <Link href="/settings" className="text-base text-primary">设置</Link>
              </BreadcrumbLink>
            </BreadcrumbItem>
            <BreadcrumbSeparator />
            <BreadcrumbItem>
              <BreadcrumbPage className="text-base font-semibold">访问令牌</BreadcrumbPage>
            </BreadcrumbItem>
          </BreadcrumbList>
        </Breadcrumb>
      </div>

      <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4 border-b pb-5">
        <div className="flex items-center gap-4">
          <div>
            <h1 className="text-xl font-bold tracking-tight bg-gradient-to-r from-foreground via-foreground/90 to-muted-foreground bg-clip-text text-transparent">个人访问令牌 (AccessToken)</h1>
            <p className="text-sm text-muted-foreground">管理您的 API 访问密钥，用于开发或第三方工具直接调用系统 API</p>
          </div>
        </div>
        <Button
          type="button"
          onClick={() => setCreateDialogOpen(true)}
          variant={'secondary'}
        >
          <Plus className="mr-1.5 size-4" />
          生成新令牌
        </Button>
      </div>

      {/* 安全警告提示 */}
      <div className="rounded-xl border border-amber-500/20 bg-amber-500/5 p-4 flex gap-3 text-amber-600 text-xs leading-relaxed">
        <AlertTriangle className="size-4 shrink-0 mt-0.5" />
        <div className="space-y-1">
          <span className="font-bold">安全提示：</span>
          <p className="text-muted-foreground">
            访问令牌具有您账户的完整接口调用权限。为了您的账户与资产安全，请切勿通过任何代码库提交、即时通讯工具或公共媒介泄露此令牌。推荐按需创建，不使用时及时撤销。
          </p>
        </div>
      </div>

      <Card className="border border-dashed shadow-sm">
        <CardHeader className="border-b border-dashed pb-4">
          <CardTitle className="text-base font-semibold">活动令牌列表</CardTitle>
          <CardDescription className="text-xs">当前可用的所有访问令牌</CardDescription>
        </CardHeader>
        <CardContent className="pt-6 space-y-4">
          {accessTokensQuery.isPending ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="size-6 animate-spin text-indigo-500" />
            </div>
          ) : (accessTokensQuery.data ?? []).length > 0 ? (
            <div className="space-y-3">
              {(accessTokensQuery.data ?? []).map((token) => (
                <div
                  key={token.id}
                  className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 rounded-xl border border-dashed p-4 bg-card hover:bg-muted/10 transition-all duration-300 shadow-sm"
                >
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <span className="font-semibold text-sm text-foreground">{token.name}</span>
                      {token.is_admin ? (
                        <Badge variant="outline" className="text-[10px] px-1.5 py-0 h-4 border-rose-500/40 text-rose-500 bg-rose-500/5 font-semibold">
                          <Shield className="size-2.5 mr-0.5" />
                          管理员
                        </Badge>
                      ) : (
                        <Badge variant="outline" className="text-[10px] px-1.5 py-0 h-4 border-border/50 text-muted-foreground bg-muted/10 font-semibold">
                          用户令牌
                        </Badge>
                      )}
                    </div>
                    <div className="flex flex-col gap-1 text-xs text-muted-foreground">
                      <div className="font-mono bg-muted/30 px-2 py-0.5 rounded border border-border/50 w-fit select-all">
                        {token.masked_token}
                      </div>
                      <div className="flex flex-wrap gap-x-4 gap-y-0.5 pt-1">
                        <span>创建于: {formatDate(token.created_at)}</span>
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-2 shrink-0 sm:self-center">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="text-xs border-dashed text-muted-foreground hover:text-indigo-500 hover:bg-indigo-500/5 rounded-lg h-8 px-2.5"
                      onClick={() => handleCopyText(token.masked_token, token.id)}
                    >
                      {copiedId === token.id ? (
                        <Check className="size-3.5 mr-1 text-emerald-500" />
                      ) : (
                        <Copy className="size-3.5 mr-1" />
                      )}
                      复制
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="text-xs border-dashed text-muted-foreground hover:text-indigo-500 hover:bg-indigo-500/5 rounded-lg h-8 px-2.5"
                      onClick={() => handleRotateToken(token.id, token.name)}
                      disabled={rotateTokenMutation.isPending}
                    >
                      <RefreshCw className={`size-3.5 mr-1 ${rotateTokenMutation.isPending && rotateTokenMutation.variables === token.id ? 'animate-spin' : ''}`} />
                      轮换
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="text-xs border-dashed text-muted-foreground hover:text-rose-500 hover:bg-rose-500/10 hover:border-rose-500/20 rounded-lg h-8 px-2.5"
                      onClick={() => handleDeleteToken(token.id, token.name)}
                      disabled={deleteTokenMutation.isPending}
                    >
                      <Trash2 className="size-3.5 mr-1" />
                      撤销
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="rounded-xl border border-dashed border-border/50 px-4 py-10 text-center text-xs text-muted-foreground bg-muted/5 flex flex-col items-center justify-center gap-3">
              <Key className="size-8 text-muted-foreground/30" />
              <span>您当前暂无生成任何访问令牌</span>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="border-dashed"
                onClick={() => setCreateDialogOpen(true)}
              >
                <Plus className="mr-1 size-3.5" />
                生成第一个令牌
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      {/* 创建令牌 Dialog */}
      <Dialog open={createDialogOpen} onOpenChange={setCreateDialogOpen}>
        <DialogContent className="sm:max-w-[425px]">
          <form onSubmit={handleCreateToken}>
            <DialogHeader>
              <DialogTitle className="text-base font-semibold">生成新令牌</DialogTitle>
              <DialogDescription className="text-xs text-muted-foreground">
                请为新的访问令牌设置一个易于识别的名称，以便将来管理。
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="token-name" className="text-xs font-semibold">令牌名称</Label>
                <Input
                  id="token-name"
                  placeholder="例如：my-development-key"
                  value={tokenName}
                  onChange={(e) => setTokenName(e.target.value)}
                  disabled={createTokenMutation.isPending}
                  className="rounded-xl border border-dashed focus:border-indigo-500 focus:ring-0 focus-visible:ring-0"
                />
              </div>
              {user?.is_admin && (
                <div className="flex items-center justify-between rounded-xl border border-dashed p-3 bg-muted/5">
                  <div className="space-y-0.5">
                    <Label htmlFor="token-admin" className="text-xs font-semibold flex items-center gap-1.5">
                      <Shield className="size-3.5 text-rose-500" />
                      管理员权限
                    </Label>
                    <p className="text-[11px] text-muted-foreground leading-normal">
                      开启后此令牌可访问 /admin/** 管理端点，默认关闭
                    </p>
                  </div>
                  <Switch
                    id="token-admin"
                    checked={tokenIsAdmin}
                    onCheckedChange={setTokenIsAdmin}
                    disabled={createTokenMutation.isPending}
                  />
                </div>
              )}
            </div>
            <DialogFooter className="gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => setCreateDialogOpen(false)}
                disabled={createTokenMutation.isPending}
                className="rounded-xl text-xs h-9 border-dashed"
              >
                取消
              </Button>
              <Button
                type="submit"
                disabled={createTokenMutation.isPending}
                variant={'secondary'}
              >
                {createTokenMutation.isPending ? (
                  <>
                    <Loader2 className="mr-1.5 size-3.5 animate-spin" />
                    正在生成...
                  </>
                ) : (
                  "生成令牌"
                )}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* 明文 Token 显示 Dialog (仅显示一次) */}
      <Dialog open={viewDialogOpen} onOpenChange={(open) => {
        if (!open) {
          setNewCreatedToken(null)
          setViewDialogOpen(false)
        }
      }}>
        <DialogContent className="sm:max-w-[500px]">
          <DialogHeader>
            <DialogTitle className="text-base font-bold text-foreground flex items-center gap-1.5">
              <Check className="size-5 text-emerald-500 border border-emerald-500 rounded-full p-0.5" />
              令牌密钥已就绪
            </DialogTitle>
          </DialogHeader>

          {newCreatedToken && (
            <div className="space-y-4 py-2">
              {/* 明文 Token 文本框 */}
              <div className="flex items-center gap-2 rounded-xl bg-indigo-500/5 border border-dashed border-indigo-500/30 p-3">
                <span className="font-mono text-xs select-all break-all flex-1 text-indigo-600 font-semibold leading-relaxed">
                  {newCreatedToken.token}
                </span>
                <Button
                  type="button"
                  size="icon"
                  variant="outline"
                  className="size-8 rounded-lg shrink-0 border-dashed text-indigo-500 hover:bg-indigo-500/10 hover:border-indigo-500/30 transition-colors"
                  onClick={() => handleCopyText(newCreatedToken.token, 9999)}
                >
                  {copiedId === 9999 ? (
                    <Check className="size-3.5 text-emerald-500" />
                  ) : (
                    <Copy className="size-3.5" />
                  )}
                </Button>
              </div>

              {/* 强提示 */}
              <div className="rounded-xl border border-rose-500/20 bg-rose-500/5 p-4 flex gap-3 text-rose-600 text-xs leading-relaxed">
                <Info className="size-4 shrink-0 mt-0.5" />
                <div className="space-y-1">
                  <span className="font-bold">重要提示：</span>
                  <p className="text-muted-foreground">
                    这是您唯一一次能够查看此访问令牌明文密钥的机会。请立即将其复制并安全地保存。
                  </p>
                </div>
              </div>
            </div>
          )}

          <DialogFooter>
            <Button
              type="button"
              onClick={() => {
                setNewCreatedToken(null)
                setViewDialogOpen(false)
              }}
              className="rounded-xl  w-full"
            >
              我已经复制并妥善保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </motion.div>
  )
}
