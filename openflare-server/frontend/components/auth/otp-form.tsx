"use client"

import * as React from "react"
import {RefreshCwIcon} from "lucide-react"

import {Button} from "@/components/ui/button"
import {Field, FieldLabel} from "@/components/ui/field"
import {InputOTP, InputOTPGroup, InputOTPSeparator, InputOTPSlot,} from "@/components/ui/input-otp"
import {AuthHeading} from "@/components/auth/auth-shell"
import {cn} from "@/lib/utils"

interface OTPFormProps {
  code: string
  setCode: (val: string) => void
  loginCodeTip: React.ReactNode
  loginCooldown: number
  isPending: boolean
  onResend: () => void
  onSubmit: () => void
}

export function OTPForm({
  code,
  setCode,
  loginCodeTip,
  loginCooldown,
  isPending,
  onResend,
  onSubmit,
}: OTPFormProps) {
  return (
    <div className="flex flex-col gap-6 [@media(max-height:700px)]:gap-4">
      <AuthHeading
        title="验证您的登录"
        description="输入发送到安全邮箱的 6 位验证码。"
      />
      {loginCodeTip ? (
        <p className="text-sm leading-6 text-muted-foreground">{loginCodeTip}</p>
      ) : null}
      <div className="flex flex-col gap-5 [@media(max-height:700px)]:gap-3">
        <Field className="gap-3">
          <div className="flex items-center justify-between">
            <FieldLabel htmlFor="otp-verification" className="text-sm font-medium">
              验证码
            </FieldLabel>
            <Button
              variant="outline"
              size="sm"
              type="button"
              onClick={onResend}
              disabled={loginCooldown > 0 || isPending}
              className="h-8 text-xs"
            >
              <RefreshCwIcon className={cn(isPending && "animate-spin")} />
              {loginCooldown > 0 ? `${loginCooldown}秒后重发` : "重新发送"}
            </Button>
          </div>
          <div className="flex justify-start">
            <InputOTP
              maxLength={6}
              id="otp-verification"
              required
              value={code}
              onChange={setCode}
              onComplete={onSubmit}
              disabled={isPending}
            >
              <InputOTPGroup className="*:data-[slot=input-otp-slot]:h-12 *:data-[slot=input-otp-slot]:w-11 *:data-[slot=input-otp-slot]:text-xl">
                <InputOTPSlot index={0} />
                <InputOTPSlot index={1} />
                <InputOTPSlot index={2} />
              </InputOTPGroup>
              <InputOTPSeparator className="mx-2" />
              <InputOTPGroup className="*:data-[slot=input-otp-slot]:h-12 *:data-[slot=input-otp-slot]:w-11 *:data-[slot=input-otp-slot]:text-xl">
                <InputOTPSlot index={3} />
                <InputOTPSlot index={4} />
                <InputOTPSlot index={5} />
              </InputOTPGroup>
            </InputOTP>
          </div>
        </Field>
        <Button
          type="button"
          className="h-10 w-full [@media(max-height:700px)]:h-9"
          variant="auth"
          onClick={onSubmit}
          disabled={isPending || code.length < 6}
        >
          {isPending ? "验证中..." : "验证"}
        </Button>
        <div className="text-center text-sm text-muted-foreground">
          遇到登录问题？{" "}
          <a
            href="#"
            className="underline underline-offset-4 transition-colors hover:text-primary"
          >
            联系客服
          </a>
        </div>
      </div>
    </div>
  )
}
