"use client"

import Link from "next/link"
import {useMemo, useState} from "react"
import {useMutation, useQuery, useQueryClient} from "@tanstack/react-query"
import {MapPin, Plus, RefreshCw, Trash2} from "lucide-react"
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
import {type OriginItem, OriginService} from "@/lib/services/openflare"
import {formatDateTime} from "@/lib/utils"

import {OriginEditorDialog} from "./components/origin-editor-dialog"

const originsQueryKey = ["openflare", "origins"] as const

export default function OriginsPage() {
  const queryClient = useQueryClient()
  const [editorOpen, setEditorOpen] = useState(false)
  const [editingOrigin, setEditingOrigin] = useState<OriginItem | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<OriginItem | null>(null)

  const originsQuery = useQuery({
    queryKey: originsQueryKey,
    queryFn: () => OriginService.list(),
  })

  const origins = useMemo(() => originsQuery.data ?? [], [originsQuery.data])

  const deleteMutation = useMutation({
    mutationFn: (id: number) => OriginService.deleteById(id),
    onSuccess: async () => {
      toast.success("源站已删除")
      setDeleteTarget(null)
      await queryClient.invalidateQueries({ queryKey: originsQueryKey })
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : "删除失败")
    },
  })

  return (
    <div className="py-6 px-1 space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2">
          <MapPin className="size-5 text-primary" />
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">源站</h1>
            <p className="text-sm text-muted-foreground">
              集中维护规则复用的源站地址，减少批量改地址时的重复操作。
            </p>
          </div>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => void originsQuery.refetch()}
            disabled={originsQuery.isFetching}
          >
            <RefreshCw
              className={`size-3.5 mr-1 ${originsQuery.isFetching ? "animate-spin" : ""}`}
            />
            刷新
          </Button>
          <Button
            size="sm"
            onClick={() => {
              setEditingOrigin(null)
              setEditorOpen(true)
            }}
          >
            <Plus className="size-3.5 mr-1" />
            新增源站
          </Button>
        </div>
      </div>

      {originsQuery.isError ? (
        <ErrorInline
          message={
            originsQuery.error instanceof Error
              ? originsQuery.error.message
              : "加载失败"
          }
          onRetry={() => void originsQuery.refetch()}
        />
      ) : null}

      <div className="border border-dashed rounded-lg overflow-hidden bg-background">
        {originsQuery.isLoading ? (
          <LoadingStateWithBorder />
        ) : origins.length === 0 ? (
          <EmptyStateWithBorder
            title="暂无源站"
            description="点击右上角新增源站，后续规则可直接复用这些地址。"
          />
        ) : (
          <div className="grid gap-0 md:grid-cols-2">
            {origins.map((origin) => (
              <article
                key={origin.id}
                className="border-b border-dashed p-4 md:[&:nth-child(odd)]:border-r"
              >
                <div className="flex flex-col gap-4">
                  <div className="space-y-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <h2 className="text-base font-semibold">{origin.name}</h2>
                      <Badge
                        variant="outline"
                        className={`text-[10px] ${
                          origin.route_count > 0
                            ? "text-emerald-600 border-emerald-500/20"
                            : "text-amber-600 border-amber-500/20"
                        }`}
                      >
                        {origin.route_count} 条规则
                      </Badge>
                    </div>
                    <p className="text-sm">{origin.address}</p>
                    <p className="text-sm text-muted-foreground">
                      {origin.remark || "暂无备注"}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      最后更新：{formatDateTime(origin.updated_at)}
                    </p>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <Button variant="outline" size="sm" asChild>
                      <Link href={`/origins/detail?id=${origin.id}`}>
                        详情
                      </Link>
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        setEditingOrigin(origin)
                        setEditorOpen(true)
                      }}
                    >
                      编辑
                    </Button>
                    <Button
                      variant="destructive"
                      size="sm"
                      onClick={() => setDeleteTarget(origin)}
                    >
                      <Trash2 className="size-3.5 mr-1" />
                      删除
                    </Button>
                  </div>
                </div>
              </article>
            ))}
          </div>
        )}
      </div>

      <OriginEditorDialog
        open={editorOpen}
        onOpenChange={setEditorOpen}
        origin={editingOrigin}
      />

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除源站</AlertDialogTitle>
            <AlertDialogDescription>
              确认删除源站 {deleteTarget?.name} 吗？
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => {
                if (deleteTarget) deleteMutation.mutate(deleteTarget.id)
              }}
            >
              确认删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
