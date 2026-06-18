"use client"

import Link from "next/link"
import {useState} from "react"
import {useMutation, useQuery, useQueryClient} from "@tanstack/react-query"
import {useSearchParams} from "next/navigation"
import {ArrowLeft, MapPin, Trash2} from "lucide-react"
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
import {OriginService} from "@/lib/services/openflare"
import {formatDateTime} from "@/lib/utils"

import {OriginEditorDialog} from "../components/origin-editor-dialog"

export function OriginDetailPageClient() {
  const searchParams = useSearchParams()
  const queryClient = useQueryClient()
  const originId = searchParams.get("id")?.trim() ?? ""
  const parsedId = Number(originId)
  const enabled = originId !== "" && Number.isFinite(parsedId)

  const [editorOpen, setEditorOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)

  const originQuery = useQuery({
    queryKey: ["openflare", "origins", originId],
    queryFn: () => OriginService.getById(parsedId),
    enabled,
  })

  const deleteMutation = useMutation({
    mutationFn: () => OriginService.deleteById(parsedId),
    onSuccess: async () => {
      toast.success("源站已删除")
      await queryClient.invalidateQueries({ queryKey: ["openflare", "origins"] })
      window.location.href = "/origins"
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : "删除失败")
    },
  })

  const origin = originQuery.data

  if (!enabled) {
    return (
      <div className="py-6 px-1">
        <EmptyStateWithBorder description="缺少有效的源站 ID。" />
      </div>
    )
  }

  if (originQuery.isLoading) {
    return (
      <div className="py-6 px-1">
        <LoadingStateWithBorder icon={MapPin} description="加载源站详情..." />
      </div>
    )
  }

  if (originQuery.isError) {
    return (
      <div className="py-6 px-1">
        <ErrorInline
          message={
            originQuery.error instanceof Error
              ? originQuery.error.message
              : "加载失败"
          }
          onRetry={() => void originQuery.refetch()}
        />
      </div>
    )
  }

  if (!origin) {
    return (
      <div className="py-6 px-1 space-y-4">
        <Button variant="ghost" size="sm" asChild>
          <Link href="/origins">
            <ArrowLeft className="size-4 mr-1" />
            返回列表
          </Link>
        </Button>
        <EmptyStateWithBorder description="源站不存在或已被删除。" />
      </div>
    )
  }

  return (
    <div className="py-6 px-1 space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-2">
          <Button variant="ghost" size="sm" className="h-8 px-2 -ml-2" asChild>
            <Link href="/origins">
              <ArrowLeft className="size-4 mr-1" />
              返回列表
            </Link>
          </Button>
          <div className="flex items-center gap-2">
            <MapPin className="size-5 text-primary" />
            <h1 className="text-2xl font-semibold tracking-tight">{origin.name}</h1>
          </div>
          <p className="text-sm text-muted-foreground">{origin.address}</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={() => setEditorOpen(true)}>
            编辑源站
          </Button>
          <Button variant="destructive" size="sm" onClick={() => setDeleteOpen(true)}>
            <Trash2 className="size-3.5 mr-1" />
            删除源站
          </Button>
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <div className="rounded-lg border border-dashed px-4 py-3">
          <p className="text-[10px] uppercase tracking-wider text-muted-foreground">绑定规则</p>
          <Badge variant="outline" className="mt-2 text-[10px]">
            {origin.route_count} 条规则
          </Badge>
        </div>
        <div className="rounded-lg border border-dashed px-4 py-3">
          <p className="text-[10px] uppercase tracking-wider text-muted-foreground">创建时间</p>
          <p className="mt-2 text-sm">{formatDateTime(origin.created_at)}</p>
        </div>
        <div className="rounded-lg border border-dashed px-4 py-3">
          <p className="text-[10px] uppercase tracking-wider text-muted-foreground">更新时间</p>
          <p className="mt-2 text-sm">{formatDateTime(origin.updated_at)}</p>
        </div>
        <div className="rounded-lg border border-dashed px-4 py-3 sm:col-span-2 xl:col-span-1">
          <p className="text-[10px] uppercase tracking-wider text-muted-foreground">备注</p>
          <p className="mt-2 text-sm text-muted-foreground">{origin.remark || "暂无备注"}</p>
        </div>
      </div>

      <div className="border border-dashed rounded-lg overflow-hidden bg-background">
        <div className="px-4 py-3 border-b border-dashed">
          <h2 className="text-sm font-semibold">关联规则</h2>
          <p className="text-xs text-muted-foreground mt-1">
            展示当前源站作为主源站绑定的规则。
          </p>
        </div>
        {origin.routes.length === 0 ? (
          <EmptyStateWithBorder
            title="暂无关联规则"
            description="当前源站还没有被任何规则引用。"
          />
        ) : (
          <Table>
            <TableHeader className="bg-muted/40">
              <TableRow className="border-dashed hover:bg-transparent">
                <TableHead className="text-xs font-semibold">域名</TableHead>
                <TableHead className="text-xs font-semibold">源站地址</TableHead>
                <TableHead className="text-xs font-semibold">状态</TableHead>
                <TableHead className="text-xs font-semibold">更新时间</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {origin.routes.map((route) => (
                <TableRow key={route.id} className="border-dashed">
                  <TableCell className="text-xs font-medium">{route.domain}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {route.origin_url}
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline" className="text-[10px]">
                      {route.enabled ? "启用" : "停用"}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {formatDateTime(route.updated_at)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>

      <OriginEditorDialog
        open={editorOpen}
        onOpenChange={setEditorOpen}
        origin={origin}
        onSaved={() => void originQuery.refetch()}
      />

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除源站</AlertDialogTitle>
            <AlertDialogDescription>
              确认删除源站 {origin.name} 吗？
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => deleteMutation.mutate()}
            >
              确认删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
