"use client"

import {useEffect, useMemo, useRef, useState} from "react"
import {useMutation, useQuery} from "@tanstack/react-query"
import {useRouter, useSearchParams} from "next/navigation"
import {toast} from "sonner"
import Link from "next/link"

import {useAuth} from "@/components/providers/auth-provider"
import {Button} from "@/components/ui/button"
import {Input} from "@/components/ui/input"
import {Spinner} from "@/components/ui/spinner"
import {Field, FieldGroup, FieldLabel} from "@/components/ui/field"
import {AuthHeading} from "@/components/auth/auth-shell"
import {CapWidget} from "@/components/auth/cap-widget"
import {AuthService} from "@/lib/services/auth"
import {ConfigService} from "@/lib/services/config"
import type {RegisterRequest} from "@/lib/services/auth/types"
import {safeRedirectTarget} from "@/lib/utils"

function getRedirectTarget(searchParams: ReturnType<typeof useSearchParams>) {
  const callbackUrl = searchParams.get("callbackUrl")
  const storedRedirect =
    typeof window === "undefined"
      ? null
      : sessionStorage.getItem("redirect_after_login")
  return safeRedirectTarget(callbackUrl || storedRedirect || "/")
}


function configBool(value: string | undefined, fallback: boolean) {
  if (value === undefined) return fallback
  return value === "true"
}

export function RegisterForm() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const { setUser } = useAuth()
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [nickname, setNickname] = useState("")
  const [email, setEmail] = useState("")
  const [code, setCode] = useState("")
  const [registerCooldown, setRegisterCooldown] = useState(0)
  const [errorMessage, setErrorMessage] = useState("")

  useEffect(() => {
    if (registerCooldown > 0) {
      const timer = setTimeout(() => setRegisterCooldown(registerCooldown - 1), 1000)
      return () => clearTimeout(timer)
    }
  }, [registerCooldown])

  const publicConfigQuery = useQuery({
    queryKey: ["public-config"],
    queryFn: () => ConfigService.getPublicConfig(),
  })

  const redirectTarget = useMemo(
    () => getRedirectTarget(searchParams),
    [searchParams],
  )

  const registrationEnabled =
    configBool(publicConfigQuery.data?.registration_enabled, false) &&
    configBool(publicConfigQuery.data?.password_register_enabled, false)

  const emailRegisterEnabled = configBool(publicConfigQuery.data?.email_register_verification_enabled, false)

  const capEnabled = configBool(publicConfigQuery.data?.cap_login_enabled, true)
  const capAutoSolve = configBool(publicConfigQuery.data?.cap_auto_solve, true)

  const [capScope, setCapScope] = useState<'send_email_code' | 'register'>('send_email_code')

  // 监听 emailRegisterEnabled 改变初始 scope
  useEffect(() => {
    setCapScope(emailRegisterEnabled ? 'send_email_code' : 'register')
  }, [emailRegisterEnabled])

  const capTokenRef = useRef<string | null>(null)
  const [capReady, setCapReady] = useState(false)
  const [capError, setCapError] = useState(false)
  const [capResetKey, setCapResetKey] = useState(0)

  const handleCapToken = (token: string) => {
    capTokenRef.current = token
    setCapReady(true)
    setCapError(false)
  }

  const handleCapError = () => {
    capTokenRef.current = null
    setCapReady(false)
    setCapError(true)
  }

  // Redirect to login if registration is closed
  useEffect(() => {
    if (publicConfigQuery.isSuccess && !registrationEnabled) {
      toast.error("系统注册功能已关闭")
      router.replace("/login")
    }
  }, [publicConfigQuery.isSuccess, registrationEnabled, router])

  const registerMutation = useMutation({
    mutationFn: (req: RegisterRequest) => {
      const headers: Record<string, string> = {}
      if (capEnabled && capTokenRef.current) {
        headers["X-Cap-Token"] = capTokenRef.current
        capTokenRef.current = null
        setCapReady(false)
      }
      return AuthService.register(req, Object.keys(headers).length ? headers : undefined)
    },
    onSuccess: (user) => {
      setUser(user)
      router.replace(redirectTarget)
      toast.success("注册并登录成功")
    },
    onError: (error: Error) => {
      setErrorMessage(error.message || "注册失败，请重试")
      if (capEnabled) {
        capTokenRef.current = null
        setCapReady(false)
        setCapResetKey((key) => key + 1)
      }
    },
  })

  const sendRegisterCodeMutation = useMutation({
    mutationFn: (targetEmail: string) => {
      const headers: Record<string, string> = {}
      if (capEnabled && capTokenRef.current) {
        headers["X-Cap-Token"] = capTokenRef.current
        capTokenRef.current = null
        setCapReady(false)
      }
      return AuthService.sendEmailCode(targetEmail, "register", Object.keys(headers).length ? headers : undefined)
    },
    onSuccess: () => {
      setRegisterCooldown(60)
      toast.success("验证码已发送至您的邮箱，请查收")
      if (capEnabled) {
        setCapScope('register')
        capTokenRef.current = null
        setCapReady(false)
        setCapResetKey((key) => key + 1)
      }
    },
    onError: (error: Error) => {
      toast.error(error.message || "发送验证码失败，请重试")
      if (capEnabled) {
        capTokenRef.current = null
        setCapReady(false)
        setCapResetKey((key) => key + 1)
      }
    },
  })

  const registerDisabled =
    registerMutation.isPending ||
    (capEnabled && capScope === 'register' && capAutoSolve && !capReady && !capError)

  const sendCodeDisabled =
    registerCooldown > 0 ||
    sendRegisterCodeMutation.isPending ||
    (capEnabled && capScope === 'send_email_code' && capAutoSolve && !capReady && !capError)

  const handleSendRegisterCode = () => {
    const trimmedEmail = email.trim()
    if (!trimmedEmail) {
      toast.error("请先输入邮箱地址")
      return
    }
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
    if (!emailRegex.test(trimmedEmail)) {
      toast.error("请输入有效的邮箱地址")
      return
    }
    if (capEnabled && capScope === 'send_email_code' && !capReady) {
      toast.error(
        capAutoSolve
          ? "人机验证尚未完成，请稍候…"
          : "请先点击「开始验证」完成人机验证",
      )
      return
    }
    sendRegisterCodeMutation.mutate(trimmedEmail)
  }

  const handleRegister = () => {
    setErrorMessage("")
    if (!username.trim() || !password) {
      toast.error("用户名和密码不能为空")
      return
    }
    if (password.length < 8) {
      toast.error("密码长度不能少于 8 位")
      return
    }
    const trimmedEmail = email.trim()
    if (!trimmedEmail) {
      toast.error("邮箱不能为空")
      return
    }
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
    if (!emailRegex.test(trimmedEmail)) {
      toast.error("请输入有效的邮箱地址")
      return
    }
    if (emailRegisterEnabled && !code.trim()) {
      toast.error("验证码不能为空")
      return
    }
    if (capEnabled && capScope === 'register' && !capReady) {
      toast.error(
        capAutoSolve
          ? "人机验证尚未完成，请稍候…"
          : "请先点击「开始验证」完成人机验证",
      )
      return
    }
    registerMutation.mutate({
      username: username.trim(),
      password,
      nickname: nickname.trim() || undefined,
      email: trimmedEmail,
      code: code.trim() || undefined,
    })
  }

  if (publicConfigQuery.isPending) {
    return (
      <div className="flex items-center justify-center py-24">
        <Spinner />
      </div>
    )
  }

  if (!registrationEnabled) {
    return null
  }

  return (
    <div className="flex flex-col gap-6 [@media(max-height:700px)]:gap-4">
      <AuthHeading
        siteName={publicConfigQuery.data?.site_name}
        title="创建您的账号"
        description="填写以下信息，开始使用平台服务。"
      />

      <div className="flex flex-col gap-5 [@media(max-height:700px)]:gap-3">
        <FieldGroup className="gap-4 [@media(min-width:500px)]:grid [@media(min-width:500px)]:grid-cols-2 [@media(max-height:700px)]:gap-3">
          <Field className="gap-1.5">
            <FieldLabel htmlFor="username">用户名 <span className="text-destructive">*</span></FieldLabel>
            <Input
              id="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="请输入用户名"
              autoComplete="username"
              className="h-10 text-sm [@media(max-height:700px)]:h-9"
              onKeyDown={(e) => e.key === "Enter" && handleRegister()}
            />
          </Field>
          <Field className="gap-1.5">
            <FieldLabel htmlFor="nickname">
              昵称
              <span className="ml-1 text-xs font-normal text-muted-foreground">（可选）</span>
            </FieldLabel>
            <Input
              id="nickname"
              value={nickname}
              onChange={(e) => setNickname(e.target.value)}
              placeholder="请输入昵称"
              autoComplete="nickname"
              className="h-10 text-sm [@media(max-height:700px)]:h-9"
              onKeyDown={(e) => e.key === "Enter" && handleRegister()}
            />
          </Field>
          <Field className="gap-1.5">
            <FieldLabel htmlFor="email">电子邮箱 <span className="text-destructive">*</span></FieldLabel>
            <Input
              id="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="请输入电子邮箱"
              autoComplete="email"
              className="h-10 text-sm [@media(max-height:700px)]:h-9"
              onKeyDown={(e) => e.key === "Enter" && handleRegister()}
            />
          </Field>
          <Field className="gap-1.5">
            <FieldLabel htmlFor="password">密码 <span className="text-destructive">*</span></FieldLabel>
            <Input
              id="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              type="password"
              placeholder="请输入密码（至少 8 位）"
              autoComplete="new-password"
              className="h-10 text-sm [@media(max-height:700px)]:h-9"
              onKeyDown={(e) => e.key === "Enter" && handleRegister()}
            />
          </Field>
          {emailRegisterEnabled && (
            <Field className="gap-1.5 [@media(min-width:500px)]:col-span-2">
              <FieldLabel htmlFor="code">邮箱验证码 <span className="text-destructive">*</span></FieldLabel>
              <div className="flex gap-2">
                <Input
                  id="code"
                  value={code}
                  onChange={(e) => setCode(e.target.value)}
                  placeholder="请输入 6 位邮箱验证码"
                  maxLength={6}
                  className="h-10 flex-1 text-sm [@media(max-height:700px)]:h-9"
                  onKeyDown={(e) => e.key === "Enter" && handleRegister()}
                />
                <Button
                  type="button"
                  variant="outline"
                  onClick={handleSendRegisterCode}
                  disabled={sendCodeDisabled}
                  className="h-10 w-[120px] text-xs [@media(max-height:700px)]:h-9"
                >
                  {registerCooldown > 0 ? `${registerCooldown}秒后重发` : "获取验证码"}
                </Button>
              </div>
            </Field>
          )}
        </FieldGroup>

        {capEnabled && (
          <CapWidget
            key={capResetKey}
            scope={capScope}
            onToken={handleCapToken}
            onError={handleCapError}
            autoStart={capAutoSolve}
          />
        )}

        {errorMessage ? (
          <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
            {errorMessage}
          </div>
        ) : null}

        <Button
          type="button"
          className="h-10 w-full [@media(max-height:700px)]:h-9"
          variant="auth"
          onClick={handleRegister}
          disabled={registerDisabled}
        >
          {registerMutation.isPending ? (
            <>
              <Spinner />
              注册中...
            </>
          ) : (
            "创建账号"
          )}
        </Button>
      </div>

      <div className="text-center text-sm text-muted-foreground">
        已经有账号？{" "}
        <Link href="/login" className="font-medium text-foreground underline underline-offset-4">
          返回登录
        </Link>
      </div>
    </div>
  )
}
