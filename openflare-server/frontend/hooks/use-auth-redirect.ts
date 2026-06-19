"use client"

import {useEffect} from "react"
import {usePathname, useRouter} from "next/navigation"

import {useUser} from "@/contexts/user-context"

/** Redirects unauthenticated users to login while allowing the shell to render first. */
export function useAuthRedirect() {
  const router = useRouter()
  const pathname = usePathname()
  const {user, loading} = useUser()

  useEffect(() => {
    if (loading || user) {
      return
    }

    const queryString = window.location.search
    const callbackUrl = queryString ? `${pathname}${queryString}` : pathname
    const loginUrl = new URL("/login", window.location.origin)

    loginUrl.searchParams.set("callbackUrl", callbackUrl)
    sessionStorage.setItem("redirect_after_login", callbackUrl)
    router.replace(loginUrl.toString())
  }, [loading, pathname, router, user])
}