"use client"

import * as React from "react"
import Link from "next/link"
import {motion, useAnimation} from "motion/react"
import {useMutation, useQuery, useQueryClient} from "@tanstack/react-query"
import {Avatar, AvatarFallback, AvatarImage} from "@/components/ui/avatar"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator
} from "@/components/ui/breadcrumb"
import {useUser} from "@/contexts/user-context"
import {
  ArrowRight,
  BookOpen,
  Camera,
  Edit,
  Globe,
  Info,
  Link2,
  Loader2,
  Lock,
  Mail,
  MapPin,
  Phone,
  Shield,
  Unlink,
  User as UserIcon
} from "lucide-react"
import {Button} from "@/components/ui/button"
import {Input} from "@/components/ui/input"
import {Separator} from "@/components/ui/separator"
import {Textarea} from "@/components/ui/textarea"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue,} from "@/components/ui/select"
import {Label} from "@/components/ui/label"
import type {ChangePasswordRequest, UpdateProfileRequest} from "@/lib/services/auth"
import {AuthService} from "@/lib/services/auth"
import services from "@/lib/services"
import {ImageCrop, ImageCropApply, ImageCropContent, ImageCropReset} from "@/components/ui/image-crop"
import {toast} from "sonner"

export function ProfileMain() {
  const { user, loading, refetch } = useUser()
  const controls = useAnimation()
  const isAnimatingRef = React.useRef(false)
  const queryClient = useQueryClient()

  // 修改密码 State
  const [oldPassword, setOldPassword] = React.useState("")
  const [newPassword, setNewPassword] = React.useState("")
  const [confirmPassword, setConfirmPassword] = React.useState("")

  // 编辑个人资料 State
  const [isEditDialogOpen, setIsEditDialogOpen] = React.useState(false)
  const [nickname, setNickname] = React.useState("")
  const [email, setEmail] = React.useState("")
  const [bio, setBio] = React.useState("")
  const [phone, setPhone] = React.useState("")
  const [gender, setGender] = React.useState("secret")
  const [website, setWebsite] = React.useState("")
  const [location, setLocation] = React.useState("")
  const [avatarUrl, setAvatarUrl] = React.useState("")

  // 头像裁剪 State
  const [cropFile, setCropFile] = React.useState<File | null>(null)
  const [isCropDialogOpen, setIsCropDialogOpen] = React.useState(false)
  const fileInputRef = React.useRef<HTMLInputElement>(null)

  // 初始化编辑表单数据
  React.useEffect(() => {
    if (user) {
      setNickname(user.nickname || "")
      setEmail(user.email || "")
      setBio(user.bio || "")
      setPhone(user.phone || "")
      setGender(user.gender || "secret")
      setWebsite(user.website || "")
      setLocation(user.location || "")
      setAvatarUrl(user.avatar_url || "")
    }
  }, [user, isEditDialogOpen])

  const changePasswordMutation = useMutation({
    mutationFn: (req: ChangePasswordRequest) => AuthService.changePassword(req),
    onSuccess: () => {
      toast.success("密码修改成功")
      setOldPassword("")
      setNewPassword("")
      setConfirmPassword("")
      void refetch()
    },
    onError: (error: Error) => {
      toast.error(error.message || "修改密码失败，请重试")
    },
  })

  const handlePasswordChange = (e: React.FormEvent) => {
    e.preventDefault()
    if (newPassword !== confirmPassword) {
      toast.error("两次输入的新密码不一致")
      return
    }
    if (newPassword.length < 8) {
      toast.error("新密码长度不能少于 8 位")
      return
    }
    changePasswordMutation.mutate({
      old_password: oldPassword,
      new_password: newPassword,
    })
  }

  const updateProfileMutation = useMutation({
    mutationFn: (req: UpdateProfileRequest) => AuthService.updateProfile(req),
    onSuccess: () => {
      toast.success("个人信息修改成功")
      setIsEditDialogOpen(false)
      void refetch()
    },
    onError: (error: Error) => {
      toast.error(error.message || "修改个人信息失败，请重试")
    },
  })

  const handleProfileSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    updateProfileMutation.mutate({
      nickname,
      email,
      avatar_url: avatarUrl,
      bio,
      phone,
      gender,
      website,
      location,
    })
  }

  const externalAccountBindingsQuery = useQuery({
    queryKey: ["auth", "external-accounts"],
    queryFn: () => AuthService.getExternalAccountBindings(),
  })

  const publicAuthSourcesQuery = useQuery({
    queryKey: ["auth", "public-sources"],
    queryFn: () => AuthService.getAuthSources(),
  })

  const bindSourceMutation = useMutation({
    mutationFn: async (sourceName: string) => {
      const { authorize_url } = await AuthService.getAuthorizeUrl(sourceName, "bind")
      sessionStorage.setItem("redirect_after_login", `${window.location.pathname}${window.location.search}`)
      window.location.href = authorize_url
    },
    onError: (error: Error) => {
      toast.error(error.message || "绑定认证源失败")
    },
  })

  const handleAvatarClick = () => {
    if (isAnimatingRef.current) return

    isAnimatingRef.current = true
    controls.start({
      rotate: [0, -20, 20, -20, 20, 0],
      transition: { duration: 0.5, ease: "easeInOut" }
    })

    setTimeout(() => {
      isAnimatingRef.current = false
    }, 650)
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) {
      setCropFile(file)
      setIsCropDialogOpen(true)
    }
  }

  const handleCroppedImage = async (croppedBase64: string) => {
    try {
      const res = await services.upload.uploadBase64Image(croppedBase64, "avatar", "avatar.png")
      setAvatarUrl(`/f/${res.id}`)
      setIsCropDialogOpen(false)
      setCropFile(null)
      toast.success("头像上传成功，点击保存以生效")
    } catch (err) {
      toast.error((err as Error).message || "上传头像失败")
    }
  }

  const getGenderLabel = (g?: string) => {
    switch (g) {
      case "male": return "男"
      case "female": return "女"
      case "secret": return "保密"
      default: return g || "保密"
    }
  }

  if (loading) {
    return (
      <div className="py-6 space-y-4 max-w-4xl mx-auto">
        <div className="border-b border-border pb-4">
          <h1 className="text-2xl font-semibold">个人资料</h1>
        </div>
      </div>
    )
  }

  if (!user) {
    return (
      <div className="py-6 space-y-6 max-w-4xl mx-auto">
        <div className="text-sm text-muted-foreground">未找到用户信息</div>
      </div>
    )
  }

  return (
    <div className="py-6 space-y-6 max-w-4xl mx-auto">
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
              <BreadcrumbPage className="text-base font-semibold">个人资料</BreadcrumbPage>
            </BreadcrumbItem>
          </BreadcrumbList>
        </Breadcrumb>
      </div>

      {/* 基本资料面板 */}
      <div className="space-y-6 bg-card border border-dashed rounded-lg p-6">
        <div className="border-b pb-4 flex justify-between items-center">
          <div>
            <h2 className="text-lg font-semibold tracking-tight">基本资料</h2>
            <p className="text-xs text-muted-foreground">您的个人账户基本信息</p>
          </div>
          <Button
            variant="outline"
            size="sm"
            className="text-xs border-dashed"
            onClick={() => setIsEditDialogOpen(true)}
          >
            <Edit className="size-3.5 mr-1.5" />
            编辑资料
          </Button>
        </div>

        <div className="flex flex-col sm:flex-row items-center sm:items-start gap-6 pt-2">
          <motion.div
            animate={controls}
            onClick={handleAvatarClick}
            className="cursor-pointer origin-center shrink-0"
            whileHover={{ scale: 1.05 }}
          >
            <Avatar className="size-20 md:size-24 border-2 border-primary/10 shadow-md">
              <AvatarImage src={user.avatar_url} alt={user.nickname || user.username} />
              <AvatarFallback className="text-2xl bg-indigo-600 text-white font-bold">
                {(user.nickname || user.username).slice(0, 2).toUpperCase()}
              </AvatarFallback>
            </Avatar>
          </motion.div>

          <div className="flex-1 w-full space-y-6">
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-6">
              <div className="space-y-1">
                <div className="text-xs text-muted-foreground flex items-center gap-1.5">
                  <UserIcon className="size-3 text-muted-foreground/70" />
                  账户
                </div>
                <div className="text-sm font-semibold">@{user.username}</div>
              </div>

              <div className="space-y-1">
                <div className="text-xs text-muted-foreground flex items-center gap-1.5">
                  <UserIcon className="size-3 text-muted-foreground/70" />
                  昵称
                </div>
                <div className="text-sm font-semibold">{user.nickname || '未设置'}</div>
              </div>

              <div className="space-y-1">
                <div className="text-xs text-muted-foreground flex items-center gap-1.5">
                  <Info className="size-3 text-muted-foreground/70" />
                  用户ID (UID)
                </div>
                <div className="text-sm font-mono font-semibold">{user.id}</div>
              </div>

              <div className="space-y-1">
                <div className="text-xs text-muted-foreground flex items-center gap-1.5">
                  <Mail className="size-3 text-muted-foreground/70" />
                  邮箱
                </div>
                <div className="text-sm font-semibold truncate max-w-[220px]">{user.email || '未绑定'}</div>
              </div>

              <div className="space-y-1">
                <div className="text-xs text-muted-foreground flex items-center gap-1.5">
                  <Phone className="size-3 text-muted-foreground/70" />
                  手机号码
                </div>
                <div className="text-sm font-semibold">{user.phone || '未设置'}</div>
              </div>

              <div className="space-y-1">
                <div className="text-xs text-muted-foreground flex items-center gap-1.5">
                  <UserIcon className="size-3 text-muted-foreground/70" />
                  性别
                </div>
                <div className="text-sm font-semibold">{getGenderLabel(user.gender)}</div>
              </div>

              <div className="space-y-1">
                <div className="text-xs text-muted-foreground flex items-center gap-1.5">
                  <Globe className="size-3 text-muted-foreground/70" />
                  个人网站
                </div>
                <div className="text-sm font-semibold truncate max-w-[220px]">
                  {user.website ? (
                    <a
                      href={user.website.startsWith("http") ? user.website : `http://${user.website}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-indigo-600 hover:underline"
                    >
                      {user.website}
                    </a>
                  ) : (
                    '未设置'
                  )}
                </div>
              </div>

              <div className="space-y-1">
                <div className="text-xs text-muted-foreground flex items-center gap-1.5">
                  <MapPin className="size-3 text-muted-foreground/70" />
                  所在地
                </div>
                <div className="text-sm font-semibold">{user.location || '未设置'}</div>
              </div>

              <div className="space-y-1">
                <div className="text-xs text-muted-foreground flex items-center gap-1.5">
                  <Shield className="size-3 text-muted-foreground/70" />
                  管理员身份
                </div>
                <div className="text-sm font-semibold flex items-center gap-1">
                  {user.is_admin ? (
                    <span className="text-rose-600 flex items-center gap-1">
                      <Shield className="size-3.5" />
                      是
                    </span>
                  ) : (
                    <span>否</span>
                  )}
                </div>
              </div>
            </div>

            <Separator className="border-dashed" />

            <div className="space-y-1">
              <div className="text-xs text-muted-foreground flex items-center gap-1.5">
                <BookOpen className="size-3 text-muted-foreground/70" />
                个人简介
              </div>
              <div className="text-sm text-foreground/80 leading-relaxed max-w-2xl bg-muted/20 border border-dashed rounded-lg p-3">
                {user.bio || '这个人很懒，什么都没有留下。'}
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* 下方左右并排布局容器 */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* 修改密码面板 */}
        <div className="space-y-6 bg-card border border-dashed rounded-lg p-6 flex flex-col justify-between">
          <div>
            <div className="border-b pb-4 flex items-center gap-2">
              <div className="p-1.5 rounded-lg bg-amber-500/10 text-amber-500">
                <Lock className="size-4" />
              </div>
              <div>
                <h2 className="text-base font-semibold tracking-tight">修改密码</h2>
                <p className="text-[11px] text-muted-foreground">更改您的账号密码以确保安全。密码长度不能少于 8 位。</p>
              </div>
            </div>

            {user.need_change_password && (
              <div className="mt-4 rounded-lg border border-amber-500/30 bg-amber-500/5 px-3.5 py-2.5 text-xs text-amber-500 flex items-start gap-2.5">
                <Info className="size-4 shrink-0 mt-0.5" />
                <div>
                  <p className="font-semibold">密码风险提示</p>
                  <p className="mt-0.5 text-amber-500/80 leading-relaxed font-normal">
                    为了账号安全，您必须修改初始密码。
                  </p>
                </div>
              </div>
            )}

            <form onSubmit={handlePasswordChange} className="space-y-3 pt-4">
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-muted-foreground">当前密码</label>
                <Input
                  type="password"
                  placeholder="请输入当前密码"
                  value={oldPassword}
                  onChange={(e) => setOldPassword(e.target.value)}
                  className="h-8 text-xs rounded-lg"
                  required
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-muted-foreground">新密码</label>
                <Input
                  type="password"
                  placeholder="新密码（至少 8 位）"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  className="h-8 text-xs rounded-lg"
                  required
                />
              </div>
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-muted-foreground">确认新密码</label>
                <Input
                  type="password"
                  placeholder="确认新密码"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  className="h-8 text-xs rounded-lg"
                  required
                />
              </div>

              <div className="pt-2">
                <Button
                  type="submit"
                  size="sm"
                  className="w-full sm:w-auto h-8 text-xs"
                  disabled={changePasswordMutation.isPending}
                >
                  {changePasswordMutation.isPending ? "提交中..." : "确认修改"}
                </Button>
              </div>
            </form>
          </div>
        </div>

        {/* 账号绑定面板 */}
        <div className="space-y-6 bg-card border border-dashed rounded-lg p-6 flex flex-col justify-between">
          <div>
            <div className="border-b pb-4 flex items-center gap-2">
              <div className="p-1.5 rounded-lg bg-indigo-500/10 text-indigo-500">
                <Link2 className="size-4" />
              </div>
              <div>
                <h2 className="text-base font-semibold tracking-tight">第三方账号绑定</h2>
                <p className="text-[11px] text-muted-foreground">管理并关联您的第三方授权账户，便于快捷登录与验证</p>
              </div>
            </div>

            {/* 已绑定账号列表 */}
            <div className="space-y-2 pt-4">
              <h3 className="text-[11px] font-semibold text-muted-foreground uppercase tracking-wider">已绑定账号</h3>
              {externalAccountBindingsQuery.isPending ? (
                <div className="flex items-center justify-center py-4">
                  <Loader2 className="size-4 animate-spin text-indigo-500" />
                </div>
              ) : (externalAccountBindingsQuery.data ?? []).length > 0 ? (
                <div className="space-y-2">
                  {(externalAccountBindingsQuery.data ?? []).map((binding) => (
                    <div
                      key={binding.id}
                      className="flex items-center justify-between gap-4 rounded-xl border border-dashed p-3 bg-card hover:bg-muted/10 transition-all duration-300"
                    >
                      <div className="space-y-0.5">
                        <span className="font-semibold text-xs text-foreground block">{binding.auth_source_label}</span>
                        <span className="text-[10px] text-muted-foreground font-mono block truncate max-w-[150px]">
                          {binding.external_username || binding.email || "未提供账号标识"}
                        </span>
                      </div>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        className="text-[11px] text-muted-foreground hover:text-rose-500 hover:bg-rose-500/10 rounded-lg h-7 px-2 transition-colors"
                        onClick={async () => {
                          await AuthService.deleteExternalAccountBinding(binding.id)
                          await queryClient.invalidateQueries({ queryKey: ["auth", "external-accounts"] })
                          toast.success("绑定已移除")
                        }}
                      >
                        <Unlink className="size-3 mr-1" />
                        解绑
                      </Button>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="rounded-xl border border-dashed border-border/50 px-4 py-6 text-center text-[11px] text-muted-foreground bg-muted/5 flex flex-col items-center justify-center">
                  <Link2 className="size-5 text-muted-foreground/30 mb-1" />
                  暂无绑定的第三方账号
                </div>
              )}
            </div>

            <Separator className="border-dashed my-4" />

            {/* 绑定新账号列表 */}
            <div className="space-y-2">
              <h3 className="text-[11px] font-semibold text-muted-foreground uppercase tracking-wider">绑定新账号</h3>
              {publicAuthSourcesQuery.isPending ? (
                <div className="flex items-center justify-center py-4">
                  <Loader2 className="size-4 animate-spin text-indigo-500" />
                </div>
              ) : (publicAuthSourcesQuery.data ?? []).length > 0 ? (
                <div className="grid grid-cols-1 gap-2">
                  {(publicAuthSourcesQuery.data ?? []).map((source) => (
                    <Button
                      key={source.id}
                      type="button"
                      variant="outline"
                      className="flex items-center justify-between w-full border border-dashed rounded-xl px-3 py-2 text-left font-normal text-xs hover:bg-indigo-500/5 hover:text-indigo-500 hover:border-indigo-500/30 transition-all duration-300 group h-8"
                      onClick={() => {
                        void bindSourceMutation.mutateAsync(source.name)
                      }}
                    >
                      <div className="flex items-center gap-1.5">
                        <Link2 className="size-3 text-muted-foreground group-hover:text-indigo-500" />
                        <span>绑定 {source.display_name || source.name}</span>
                      </div>
                      <ArrowRight className="size-3 opacity-0 -translate-x-1 group-hover:opacity-100 group-hover:translate-x-0 transition-all text-indigo-500" />
                    </Button>
                  ))}
                </div>
              ) : (
                <div className="text-[11px] text-muted-foreground text-center py-2">
                  暂无可用第三方认证源
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* 编辑资料 Dialog */}
      <Dialog open={isEditDialogOpen} onOpenChange={setIsEditDialogOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>编辑个人资料</DialogTitle>
            <DialogDescription>修改您的基本信息，保存后立即生效。</DialogDescription>
          </DialogHeader>

          <form onSubmit={handleProfileSubmit} className="space-y-4">
            {/* 上传头像域 */}
            <div className="flex flex-col items-center gap-2 py-2">
              <div
                className="group relative cursor-pointer rounded-full border border-dashed hover:border-primary/50 transition-all"
                onClick={() => fileInputRef.current?.click()}
              >
                <Avatar className="size-20 border-2 border-primary/5 shadow-md">
                  <AvatarImage src={avatarUrl} alt={nickname} />
                  <AvatarFallback className="text-xl bg-indigo-600 text-white font-bold">
                    {(nickname || "U").slice(0, 2).toUpperCase()}
                  </AvatarFallback>
                </Avatar>
                <div className="absolute inset-0 bg-black/40 rounded-full flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity">
                  <Camera className="size-5 text-white" />
                </div>
              </div>
              <span className="text-[10px] text-muted-foreground">点击上传/修改头像</span>
              <input
                type="file"
                ref={fileInputRef}
                accept="image/*"
                className="hidden"
                onChange={handleFileChange}
              />
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label htmlFor="nickname" className="text-xs">昵称</Label>
                <Input
                  id="nickname"
                  value={nickname}
                  onChange={(e) => setNickname(e.target.value)}
                  className="h-8 text-xs rounded-lg"
                  placeholder="请输入您的昵称"
                  required
                />
              </div>

              <div className="space-y-1">
                <Label htmlFor="email" className="text-xs">邮箱</Label>
                <Input
                  id="email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="h-8 text-xs rounded-lg"
                  placeholder="请输入您的邮箱"
                />
              </div>

              <div className="space-y-1">
                <Label htmlFor="phone" className="text-xs">手机号码</Label>
                <Input
                  id="phone"
                  value={phone}
                  onChange={(e) => setPhone(e.target.value)}
                  className="h-8 text-xs rounded-lg"
                  placeholder="请输入手机号"
                />
              </div>

              <div className="space-y-1">
                <Label htmlFor="gender" className="text-xs">性别</Label>
                <Select value={gender} onValueChange={setGender}>
                  <SelectTrigger id="gender" size="sm" className="w-full text-xs h-8">
                    <SelectValue placeholder="选择性别" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="male">男</SelectItem>
                    <SelectItem value="female">女</SelectItem>
                    <SelectItem value="secret">保密</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1">
                <Label htmlFor="website" className="text-xs">个人网站</Label>
                <Input
                  id="website"
                  value={website}
                  onChange={(e) => setWebsite(e.target.value)}
                  className="h-8 text-xs rounded-lg"
                  placeholder="https://..."
                />
              </div>

              <div className="space-y-1">
                <Label htmlFor="location" className="text-xs">所在地</Label>
                <Input
                  id="location"
                  value={location}
                  onChange={(e) => setLocation(e.target.value)}
                  className="h-8 text-xs rounded-lg"
                  placeholder="如：北京"
                />
              </div>
            </div>

            <div className="space-y-1">
              <Label htmlFor="bio" className="text-xs">个人简介</Label>
              <Textarea
                id="bio"
                value={bio}
                onChange={(e) => setBio(e.target.value)}
                className="text-xs min-h-[60px] rounded-lg"
                placeholder="介绍一下自己吧..."
              />
            </div>

            <DialogFooter className="pt-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-8 text-xs"
                onClick={() => setIsEditDialogOpen(false)}
              >
                取消
              </Button>
              <Button
                type="submit"
                size="sm"
                className="h-8 text-xs"
                disabled={updateProfileMutation.isPending}
              >
                {updateProfileMutation.isPending ? "保存中..." : "保存修改"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* 头像裁剪 Dialog */}
      <Dialog open={isCropDialogOpen} onOpenChange={setIsCropDialogOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>裁剪头像</DialogTitle>
            <DialogDescription>调整头像位置和大小，使其处于居中圆形区域内。</DialogDescription>
          </DialogHeader>

          {cropFile && (
            <div className="flex flex-col items-center gap-4 py-2">
              <ImageCrop file={cropFile} aspect={1} onCrop={handleCroppedImage}>
                <ImageCropContent className="max-w-sm rounded-lg border border-dashed" />
                <div className="mt-3 flex justify-center gap-2">
                  <ImageCropReset />
                  <ImageCropApply />
                </div>
              </ImageCrop>
            </div>
          )}

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="h-8 text-xs"
              onClick={() => {
                setIsCropDialogOpen(false)
                setCropFile(null)
              }}
            >
              取消
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
