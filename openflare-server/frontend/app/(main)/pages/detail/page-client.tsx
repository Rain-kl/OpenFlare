"use client"

import Link from "next/link"
import {useMemo, useState} from "react"
import {useMutation, useQuery, useQueryClient} from "@tanstack/react-query"
import {useSearchParams} from "next/navigation"
import {ArrowLeft, ChevronDown, ChevronRight, FileText, Loader2, Trash2, Upload,} from "lucide-react"
import {toast} from "sonner"

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {EmptyStateWithBorder} from "@/components/layout/empty"
import {ErrorInline} from "@/components/layout/error"
import {LoadingStateWithBorder} from "@/components/layout/loading"
import {Badge} from "@/components/ui/badge"
import {Button} from "@/components/ui/button"
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow,} from "@/components/ui/table"
import {type PagesDeployment, PagesService} from "@/lib/services/openflare"
import {formatDateTime} from "@/lib/utils"

import {DeploymentUploadDialog} from "../components/deployment-upload-dialog"
import {ProjectEditorDialog} from "../components/project-editor-dialog"
import {
  deploymentFilesQueryKey,
  deploymentsQueryKey,
  formatBytes,
  projectQueryKey,
  projectsQueryKey,
} from "../components/pages-utils"

function DeploymentFilesPanel({
  projectId,
  deployment,
}: {
  projectId: number
  deployment: PagesDeployment
}) {
  const filesQuery = useQuery({
    queryKey: deploymentFilesQueryKey(projectId, deployment.id),
    queryFn: () => PagesService.listDeploymentFiles(projectId, deployment.id),
  })

  if (filesQuery.isLoading) {
    return <p className="px-4 py-3 text-xs text-muted-foreground">加载文件清单...</p>
  }

  if (filesQuery.isError) {
    return (
      <p className="px-4 py-3 text-xs text-destructive">
        {filesQuery.error instanceof Error ? filesQuery.error.message : "加载失败"}
      </p>
    )
  }

  const files = filesQuery.data ?? []
  if (files.length === 0) {
    return <p className="px-4 py-3 text-xs text-muted-foreground">暂无文件记录</p>
  }

  return (
    <div className="border-t border-dashed bg-muted/10">
      <Table>
        <TableHeader>
          <TableRow className="border-dashed hover:bg-transparent">
            <TableHead className="text-xs">路径</TableHead>
            <TableHead className="text-xs text-right">大小</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {files.map((file) => (
            <TableRow key={file.id} className="border-dashed">
              <TableCell className="text-xs font-mono">{file.path}</TableCell>
              <TableCell className="text-xs text-right text-muted-foreground">
                {formatBytes(file.size)}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

export function PagesDetailPageClient() {
  const searchParams = useSearchParams()
  const queryClient = useQueryClient()
  const projectId = searchParams.get("id")?.trim() ?? ""
  const parsedProjectId = Number(projectId)

  const [editorOpen, setEditorOpen] = useState(false)
  const [uploadOpen, setUploadOpen] = useState(false)
  const [expandedDeploymentId, setExpandedDeploymentId] = useState<number | null>(null)
  const [deleteProjectOpen, setDeleteProjectOpen] = useState(false)
  const [pendingDeploymentAction, setPendingDeploymentAction] = useState<{
    type: "activate" | "delete"
    deployment: PagesDeployment
  } | null>(null)

  const enabled = projectId !== "" && Number.isFinite(parsedProjectId)

  const projectQuery = useQuery({
    queryKey: projectQueryKey(projectId),
    queryFn: () => PagesService.getProject(parsedProjectId),
    enabled,
  })

  const deploymentsQuery = useQuery({
    queryKey: deploymentsQueryKey(parsedProjectId),
    queryFn: () => PagesService.listDeployments(parsedProjectId),
    enabled,
  })

  const activateMutation = useMutation({
    mutationFn: (deploymentId: number) =>
      PagesService.activateDeployment(parsedProjectId, deploymentId),
    onSuccess: async () => {
      toast.success("部署已激活")
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: deploymentsQueryKey(parsedProjectId) }),
        queryClient.invalidateQueries({ queryKey: projectQueryKey(projectId) }),
        queryClient.invalidateQueries({ queryKey: projectsQueryKey }),
      ])
      setPendingDeploymentAction(null)
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : "激活失败")
    },
  })

  const deleteDeploymentMutation = useMutation({
    mutationFn: (deploymentId: number) =>
      PagesService.deleteDeployment(parsedProjectId, deploymentId),
    onSuccess: async () => {
      toast.success("部署已删除")
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: deploymentsQueryKey(parsedProjectId) }),
        queryClient.invalidateQueries({ queryKey: projectQueryKey(projectId) }),
        queryClient.invalidateQueries({ queryKey: projectsQueryKey }),
      ])
      setPendingDeploymentAction(null)
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : "删除失败")
    },
  })

  const deleteProjectMutation = useMutation({
    mutationFn: () => PagesService.deleteProject(parsedProjectId),
    onSuccess: async () => {
      toast.success("项目已删除")
      await queryClient.invalidateQueries({ queryKey: projectsQueryKey })
      window.location.href = "/pages"
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : "删除失败")
    },
  })

  const project = projectQuery.data
  const deployments = useMemo(() => deploymentsQuery.data ?? [], [deploymentsQuery.data])

  if (!enabled) {
    return (
      <div className="py-6 px-1">
        <EmptyStateWithBorder description="缺少有效的 Pages 项目 ID。" />
      </div>
    )
  }

  if (projectQuery.isLoading) {
    return (
      <div className="py-6 px-1">
        <LoadingStateWithBorder icon={FileText} description="加载项目详情..." />
      </div>
    )
  }

  if (projectQuery.isError) {
    return (
      <div className="py-6 px-1">
        <ErrorInline
          message={
            projectQuery.error instanceof Error
              ? projectQuery.error.message
              : "加载失败"
          }
          onRetry={() => void projectQuery.refetch()}
        />
      </div>
    )
  }

  if (!project) {
    return (
      <div className="py-6 px-1 space-y-4">
        <Button variant="ghost" size="sm" asChild>
          <Link href="/pages">
            <ArrowLeft className="size-4 mr-1" />
            返回列表
          </Link>
        </Button>
        <EmptyStateWithBorder description="Pages 项目不存在或已被删除。" />
      </div>
    )
  }

  return (
    <div className="py-6 px-1 space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-2">
          <Button variant="ghost" size="sm" className="h-8 px-2 -ml-2" asChild>
            <Link href="/pages">
              <ArrowLeft className="size-4 mr-1" />
              返回列表
            </Link>
          </Button>
          <div className="flex items-center gap-2">
            <FileText className="size-5 text-primary" />
            <h1 className="text-2xl font-semibold tracking-tight">{project.name}</h1>
          </div>
          <p className="text-sm text-muted-foreground">
            {project.slug} · {project.deployment_count} 个部署
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" size="sm" onClick={() => setEditorOpen(true)}>
            编辑项目
          </Button>
          <Button size="sm" onClick={() => setUploadOpen(true)}>
            <Upload className="size-3.5 mr-1" />
            上传部署包
          </Button>
          <Button
            variant="destructive"
            size="sm"
            onClick={() => setDeleteProjectOpen(true)}
          >
            <Trash2 className="size-3.5 mr-1" />
            删除项目
          </Button>
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-3">
        <div className="rounded-lg border border-dashed px-4 py-3">
          <p className="text-[10px] uppercase tracking-wider text-muted-foreground">状态</p>
          <Badge variant="outline" className="mt-2 text-[10px]">
            {project.enabled ? "已启用" : "已停用"}
          </Badge>
        </div>
        <div className="rounded-lg border border-dashed px-4 py-3">
          <p className="text-[10px] uppercase tracking-wider text-muted-foreground">当前激活</p>
          <p className="mt-2 text-sm font-semibold">
            {project.active_deployment
              ? `#${project.active_deployment.deployment_number}`
              : "暂无"}
          </p>
        </div>
        <div className="rounded-lg border border-dashed px-4 py-3">
          <p className="text-[10px] uppercase tracking-wider text-muted-foreground">更新时间</p>
          <p className="mt-2 text-sm">{formatDateTime(project.updated_at)}</p>
        </div>
      </div>

      <div className="border border-dashed rounded-lg overflow-hidden bg-background">
        <div className="px-4 py-3 border-b border-dashed">
          <h2 className="text-sm font-semibold">部署历史</h2>
          <p className="text-xs text-muted-foreground mt-1">
            部署不可变；激活后发布配置，Agent 才会拉取并切换静态资源。
          </p>
        </div>

        {deploymentsQuery.isLoading ? (
          <LoadingStateWithBorder />
        ) : deployments.length === 0 ? (
          <EmptyStateWithBorder
            title="暂无部署"
            description="上传 zip 部署包后，可以在这里激活某个部署版本。"
          />
        ) : (
          <div className="divide-y divide-dashed">
            {deployments.map((deployment) => {
              const expanded = expandedDeploymentId === deployment.id
              return (
                <div key={deployment.id}>
                  <div className="flex flex-col gap-3 p-4 md:flex-row md:items-center md:justify-between">
                    <div className="flex items-start gap-2">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="size-7 shrink-0"
                        onClick={() =>
                          setExpandedDeploymentId(expanded ? null : deployment.id)
                        }
                      >
                        {expanded ? (
                          <ChevronDown className="size-4" />
                        ) : (
                          <ChevronRight className="size-4" />
                        )}
                      </Button>
                      <div>
                        <div className="flex items-center gap-2">
                          <p className="text-sm font-medium">
                            #{deployment.deployment_number}
                          </p>
                          {deployment.status === "active" ? (
                            <Badge variant="outline" className="text-[10px]">
                              已激活
                            </Badge>
                          ) : null}
                        </div>
                        <p className="mt-1 text-xs text-muted-foreground">
                          {deployment.checksum.slice(0, 16)} · {deployment.file_count} files ·{" "}
                          {formatBytes(deployment.total_size)}
                        </p>
                        <p className="mt-1 text-xs text-muted-foreground">
                          创建于 {formatDateTime(deployment.created_at)}
                        </p>
                      </div>
                    </div>
                    <div className="flex gap-2 md:ml-9">
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={
                          deployment.status === "active" || activateMutation.isPending
                        }
                        onClick={() =>
                          setPendingDeploymentAction({
                            type: "activate",
                            deployment,
                          })
                        }
                      >
                        激活
                      </Button>
                      <Button
                        variant="destructive"
                        size="sm"
                        disabled={
                          deployment.status === "active" ||
                          deleteDeploymentMutation.isPending
                        }
                        onClick={() =>
                          setPendingDeploymentAction({
                            type: "delete",
                            deployment,
                          })
                        }
                      >
                        删除
                      </Button>
                    </div>
                  </div>
                  {expanded ? (
                    <DeploymentFilesPanel
                      projectId={parsedProjectId}
                      deployment={deployment}
                    />
                  ) : null}
                </div>
              )
            })}
          </div>
        )}
      </div>

      <ProjectEditorDialog
        open={editorOpen}
        onOpenChange={setEditorOpen}
        project={project}
      />
      <DeploymentUploadDialog
        open={uploadOpen}
        onOpenChange={setUploadOpen}
        projectId={parsedProjectId}
      />

      <AlertDialog open={deleteProjectOpen} onOpenChange={setDeleteProjectOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除 Pages 项目</AlertDialogTitle>
            <AlertDialogDescription>
              确认删除项目 {project.name} 吗？此操作不可恢复。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => deleteProjectMutation.mutate()}
            >
              {deleteProjectMutation.isPending ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                "确认删除"
              )}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={pendingDeploymentAction !== null}
        onOpenChange={(open) => {
          if (!open) setPendingDeploymentAction(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {pendingDeploymentAction?.type === "activate" ? "激活部署" : "删除部署"}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {pendingDeploymentAction?.type === "activate"
                ? `确认激活部署 #${pendingDeploymentAction.deployment.deployment_number} 吗？`
                : `确认删除部署 #${pendingDeploymentAction?.deployment.deployment_number} 吗？`}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction
              className={
                pendingDeploymentAction?.type === "delete"
                  ? "bg-destructive text-destructive-foreground hover:bg-destructive/90"
                  : undefined
              }
              onClick={() => {
                if (!pendingDeploymentAction) return
                if (pendingDeploymentAction.type === "activate") {
                  activateMutation.mutate(pendingDeploymentAction.deployment.id)
                } else {
                  deleteDeploymentMutation.mutate(pendingDeploymentAction.deployment.id)
                }
              }}
            >
              确认
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
