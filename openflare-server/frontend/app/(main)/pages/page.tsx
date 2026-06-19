"use client"

import {useState} from "react"
import {useQuery} from "@tanstack/react-query"
import {FileText, Plus, RefreshCw} from "lucide-react"

import {EmptyStateWithBorder} from "@/components/layout/empty"
import {ErrorInline} from "@/components/layout/error"
import {LoadingStateWithBorder} from "@/components/layout/loading"
import {Button} from "@/components/ui/button"
import {PagesService} from "@/lib/services/openflare"

import {ProjectEditorDialog} from "./components/project-editor-dialog"
import {ProjectListItem} from "./components/project-list-item"
import {projectsQueryKey} from "./components/pages-utils"

export default function PagesPage() {
  const [editorOpen, setEditorOpen] = useState(false)

  const projectsQuery = useQuery({
    queryKey: projectsQueryKey,
    queryFn: () => PagesService.listProjects(),
  })

  return (
    <div className="py-6 px-1 space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2">
          <FileText className="size-5 text-primary" />
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">Pages</h1>
            <p className="text-sm text-muted-foreground">
              边缘静态站点托管，上传 zip 部署包并在代理规则中选择 Pages 上游。
            </p>
          </div>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => void projectsQuery.refetch()}
            disabled={projectsQuery.isFetching}
          >
            <RefreshCw
              className={`size-3.5 mr-1 ${projectsQuery.isFetching ? "animate-spin" : ""}`}
            />
            刷新
          </Button>
          <Button size="sm" onClick={() => setEditorOpen(true)}>
            <Plus className="size-3.5 mr-1" />
            新建项目
          </Button>
        </div>
      </div>

      {projectsQuery.isError ? (
        <ErrorInline
          message={
            projectsQuery.error instanceof Error
              ? projectsQuery.error.message
              : "加载失败"
          }
          onRetry={() => void projectsQuery.refetch()}
        />
      ) : null}

      <div className="space-y-3">
        {projectsQuery.isLoading ? (
          <LoadingStateWithBorder />
        ) : (projectsQuery.data ?? []).length === 0 ? (
          <EmptyStateWithBorder
            title="还没有 Pages 项目"
            description="先创建一个项目，再上传静态资源部署包。"
          />
        ) : (
          (projectsQuery.data ?? []).map((project) => (
            <ProjectListItem key={project.id} project={project} />
          ))
        )}
      </div>

      <ProjectEditorDialog open={editorOpen} onOpenChange={setEditorOpen} />
    </div>
  )
}