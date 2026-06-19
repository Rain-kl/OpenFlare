"use client"

import {useCallback, useEffect, useRef, useState} from "react"
import {AnimatePresence, motion} from "motion/react"
import {useRouter, useSearchParams} from "next/navigation"
import {toast} from "sonner"
import {Spinner} from "@/components/ui/spinner"
import {LoginForm} from "@/components/auth/login-form"
import {AuthShell} from "@/components/auth/auth-shell"
import {Check} from "lucide-react"

import {AuthService} from "@/lib/services/auth"
import {useAuth} from "@/components/providers/auth-provider"
import {safeRedirectTarget} from "@/lib/utils"


/**
 * 登录页面组件
 * 显示登录表单和登录按钮
 *
 * @example
 * ```tsx
 * <LoginPage />
 * ```
 * @returns {React.ReactNode} 登录页面组件
 */
export function LoginPage() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const { user, loading, setUser } = useAuth()
  const [showOTP, setShowOTP] = useState(false)

  /* 处理OAuth回调 */
  const isOAuthCallback = !!(searchParams.get('state') && searchParams.get('code'))
  const [isProcessingCallback, setIsProcessingCallback] = useState(isOAuthCallback)
  const isCheckingSession = !isOAuthCallback && loading

  const [loginSuccess, setLoginSuccess] = useState(false)
  const redirectedRef = useRef(false)
  const callbackProcessedRef = useRef(false)

  const resolveRedirectTarget = useCallback(() => {
    const callbackUrl = searchParams.get('callbackUrl')
    const storedRedirect = sessionStorage.getItem('redirect_after_login')
    const target = callbackUrl || storedRedirect || '/'

    if (storedRedirect) {
      sessionStorage.removeItem('redirect_after_login')
    }

    return safeRedirectTarget(target)
  }, [searchParams])

  const resolveRedirectTargetRef = useRef(resolveRedirectTarget)
  useEffect(() => {
    resolveRedirectTargetRef.current = resolveRedirectTarget
  }, [resolveRedirectTarget])


  /* 登录页兜底：已登录用户直接跳转 */
  useEffect(() => {
    const state = searchParams.get('state')
    const code = searchParams.get('code')

    if ((state && code) || loading || !user) {
      return
    }

    if (!redirectedRef.current) {
      redirectedRef.current = true
      router.replace(resolveRedirectTargetRef.current())
    }
  }, [loading, router, searchParams, user])

  /* 回调逻辑 */
  useEffect(() => {
    const handleOAuthCallback = async () => {
      const state = searchParams.get('state')
      const code = searchParams.get('code')

      if (state && code) {
        if (callbackProcessedRef.current) return
        callbackProcessedRef.current = true

        setIsProcessingCallback(true)
        try {
          const result = await AuthService.handleCallback({ state, code })
          if (result.status === "need_bind") {
            toast.info("您的第三方账号未绑定本地账号，系统已关闭注册。请登录已有本地账号进行绑定。")
            setIsProcessingCallback(false)
            router.replace('/login')
            return
          }
          if (result.user) {
            setUser(result.user)
          }
          setLoginSuccess(true)
          toast.success(result.status === "bound" ? "绑定成功" : "登录成功")

          setTimeout(() => {
            if (!redirectedRef.current) {
              redirectedRef.current = true
              router.replace(resolveRedirectTargetRef.current())
            }
          }, 1500)
        } catch (error) {
          console.error('OAuth callback error:', error)
          toast.error(error instanceof Error ? error.message : "登录失败，请重试")
          setIsProcessingCallback(false)
          router.replace('/login')
        }
      }
    }
    handleOAuthCallback()
  }, [router, searchParams, setUser])

  return (
    <AuthShell wide={showOTP}>
      <div className="w-full">
        <AnimatePresence mode="wait">
          {isProcessingCallback || isCheckingSession ? (
            <motion.div
              key={isProcessingCallback ? "processing" : "session-check"}
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className="w-full"
            >
              {isCheckingSession ? (
                <div className="flex flex-col items-center justify-center gap-4 py-16">
                  <div className="relative">
                    <Spinner className="size-8" />
                  </div>
                  <div className="flex flex-col gap-2 text-center">
                    <h3 className="font-semibold tracking-tight text-foreground">正在检查登录状态</h3>
                    <p className="text-xs text-muted-foreground">请稍候，我们正在确认当前会话...</p>
                  </div>
                </div>
              ) : loginSuccess ? (
                <div className="flex flex-col items-center justify-center gap-4 py-16">
                  <motion.div
                    initial={{ scale: 0.5, opacity: 0 }}
                    animate={{ scale: 1, opacity: 1 }}
                    transition={{ type: "spring", stiffness: 300, damping: 20 }}
                    className="flex size-8 items-center justify-center rounded-full bg-primary/10 text-primary ring-1 ring-primary/20"
                  >
                    <Check className="size-6" strokeWidth={3} />
                  </motion.div>
                  <div className="flex flex-col gap-2 text-center">
                    <h3 className="font-semibold tracking-tight text-foreground">登录成功</h3>
                    <p className="text-xs text-muted-foreground">正在跳转至控制台...</p>
                  </div>
                </div>
              ) : (
                <div className="flex flex-col items-center justify-center gap-4 py-16">
                  <div className="relative">
                    <Spinner className="size-8" />
                  </div>
                  <div className="flex flex-col gap-2 text-center">
                    <h3 className="font-semibold tracking-tight text-foreground">正在验证凭据</h3>
                    <p className="text-xs text-muted-foreground">请稍候，我们正在为您建立安全会话...</p>
                  </div>
                </div>
              )}
            </motion.div>
          ) : (
            <motion.div
              key="login-form-wrapper"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ duration: 0.4 }}
              className="w-full"
            >
              <LoginForm onOTPStateChange={setShowOTP} />
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </AuthShell>
  )
}
