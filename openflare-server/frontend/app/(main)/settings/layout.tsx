"use client"

import {RequireAuth} from "@/components/auth/require-auth"

export default function SettingsLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return <RequireAuth>{children}</RequireAuth>
}