"use client"

import type {ReactNode} from "react"
import {Loader2} from "lucide-react"

import {ErrorPage} from "@/components/layout/error"
import {useUser} from "@/contexts/user-context"

type RequireAuthProps = {
  children: ReactNode
  fallback?: ReactNode
  minHeightClassName?: string
}

/**
 * Guards page content until the session user is available.
 * Redirect to login is handled by the parent layout; this only covers the content slot.
 */
export function RequireAuth({
  children,
  fallback,
  minHeightClassName = "min-h-[400px]",
}: RequireAuthProps) {
  const {user, loading} = useUser()

  if (loading) {
    return fallback ?? (
      <div className={`flex items-center justify-center ${minHeightClassName}`}>
        <Loader2 className="size-6 animate-spin text-primary" />
      </div>
    )
  }

  if (!user) {
    return null
  }

  return <>{children}</>
}

type RequireAdminAuthProps = {
  children: ReactNode
}

/** Guards admin routes after the shared shell has rendered. */
export function RequireAdminAuth({children}: RequireAdminAuthProps) {
  const {user, loading} = useUser()

  if (loading) {
    return (
      <div className="flex min-h-[400px] items-center justify-center">
        <Loader2 className="size-6 animate-spin text-primary" />
      </div>
    )
  }

  if (!user?.is_admin) {
    return (
      <ErrorPage
        title="访问被拒绝"
        message="您没有权限访问此页面"
      />
    )
  }

  return <>{children}</>
}