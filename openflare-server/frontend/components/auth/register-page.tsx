"use client"

import {RegisterForm} from "@/components/auth/register-form"
import {AuthShell} from "@/components/auth/auth-shell"

export function RegisterPage() {
  return (
    <AuthShell wide>
      <RegisterForm />
    </AuthShell>
  )
}
