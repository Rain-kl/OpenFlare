"use client"

import Link from "next/link"
import {ChevronRight} from "lucide-react"

import {Badge} from "@/components/ui/badge"
import type {PagesProject} from "@/lib/services/openflare"
import {formatDateTime} from "@/lib/utils"

interface ProjectListItemProps {
  project: PagesProject
}

export function ProjectListItem({ project }: ProjectListItemProps) {
  return (
    <Link
      href={`/pages/detail?id=${project.id}`}
      className="group block rounded-lg border border-dashed bg-background p-4 transition hover:bg-muted/20"
    >
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div className="min-w-0 space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-base font-semibold">{project.name}</h2>
            <Badge variant="outline" className="text-[10px]">
              {project.enabled ? "已启用" : "已停用"}
            </Badge>
            <Badge variant="outline" className="text-[10px]">
              {project.spa_fallback_enabled ? "SPA fallback" : "严格 404"}
            </Badge>
          </div>
          <p className="text-sm text-muted-foreground">{project.slug}</p>
          {project.description ? (
            <p className="line-clamp-2 text-sm text-muted-foreground">{project.description}</p>
          ) : null}
        </div>

        <div className="rounded-lg border border-dashed px-4 py-3 text-sm md:min-w-36">
          <p className="text-xs text-muted-foreground">当前激活</p>
          <p className="mt-1 font-semibold">
            {project.active_deployment
              ? `#${project.active_deployment.deployment_number}`
              : "暂无"}
          </p>
        </div>
      </div>

      <div className="mt-4 flex items-center justify-between border-t border-dashed pt-3 text-xs text-muted-foreground">
        <span>
          激活时间：
          {project.active_deployment?.activated_at
            ? formatDateTime(project.active_deployment.activated_at)
            : "未激活"}
        </span>
        <span className="inline-flex items-center text-primary group-hover:translate-x-0.5 transition-transform">
          查看详情
          <ChevronRight className="size-3.5 ml-0.5" />
        </span>
      </div>
    </Link>
  )
}