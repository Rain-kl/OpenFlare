"use client"

import {useQuery} from '@tanstack/react-query'
import {useEffect} from 'react'
import {ConfigService} from '@/lib/services/config'
import {usePathname} from 'next/navigation'

export function SiteTitleUpdater() {
  const pathname = usePathname()
  const publicConfigQuery = useQuery({
    queryKey: ["public-config"],
    queryFn: () => ConfigService.getPublicConfig(),
  })

  useEffect(() => {
    const siteName = publicConfigQuery.data?.site_name || "OpenFlare"

    // Determine the page suffix based on path
    let suffix = ""
    if (pathname === "/login") {
      suffix = " - 登录"
    } else if (pathname === "/register") {
      suffix = " - 注册"
    } else if (pathname.startsWith("/admin")) {
      suffix = " - 后台管理"
    } else if (pathname === "/") {
      suffix = " - 总览"
    }

    document.title = `${siteName}${suffix}`
  }, [publicConfigQuery.data?.site_name, pathname])

  return null
}
