"use client"

import {usePublicConfig} from "@/hooks/use-public-config"

export function RobotsMeta() {
  const { config } = usePublicConfig()

  // Default to noindex, nofollow if config is loading or if it's explicitly disabled
  const enabled = config?.search_engine_indexing_enabled === "true"

  return (
    <meta
      name="robots"
      content={enabled ? "index, follow" : "noindex, nofollow"}
    />
  )
}
