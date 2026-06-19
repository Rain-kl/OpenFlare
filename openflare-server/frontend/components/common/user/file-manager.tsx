"use client"

import * as React from "react"
import {useMutation, useQuery, useQueryClient} from "@tanstack/react-query"
import {
  ChevronLeft,
  ChevronRight,
  Download,
  FileArchive,
  FileAudio,
  FileImage,
  FileText,
  FileVideo,
  Globe,
  Loader2,
  Lock,
  Plus,
  Search,
  Trash2,
  Upload
} from "lucide-react"
import {toast} from "sonner"

import {Button} from "@/components/ui/button"
import {Input} from "@/components/ui/input"
import {Badge} from "@/components/ui/badge"
import {Card, CardContent} from "@/components/ui/card"
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

import {FileImagePreview} from "@/components/common/file-image-preview"
import services, {formatFileSize} from "@/lib/services"
import type {Upload as UploadRecord} from "@/lib/services/upload/types"

/* ─── 工具函数 ─────────────────────────────────────────── */

function getFileIcon(mimeType: string, className = "size-10") {
  if (mimeType.startsWith("image/")) return <FileImage className={`${className} text-blue-400`} />
  if (mimeType.startsWith("video/")) return <FileVideo className={`${className} text-purple-400`} />
  if (mimeType.startsWith("audio/")) return <FileAudio className={`${className} text-green-400`} />
  if (mimeType.includes("zip") || mimeType.includes("tar") || mimeType.includes("gzip"))
    return <FileArchive className={`${className} text-amber-400`} />
  return <FileText className={`${className} text-slate-400`} />
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  })
}

export function UserFileManager() {
  const queryClient = useQueryClient()
  const [keyword, setKeyword] = React.useState("")
  const [debouncedKeyword, setDebouncedKeyword] = React.useState("")
  const [page, setPage] = React.useState(1)
  const [uploading, setUploading] = React.useState(false)
  const [deleteTarget, setDeleteTarget] = React.useState<UploadRecord | null>(null)
  const [uploadAccessMode, setUploadAccessMode] = React.useState<number>(0) // 默认私有
  const fileInputRef = React.useRef<HTMLInputElement>(null)
  const pageSize = 12 // 每页 12 个卡片更均衡（3x4 或 4x3 布局）

  // 搜索防抖
  React.useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedKeyword(keyword)
      setPage(1)
    }, 400)
    return () => clearTimeout(timer)
  }, [keyword])

  // 我的文件列表查询
  const { data, isPending } = useQuery({
    queryKey: ["user-files", page, pageSize, debouncedKeyword],
    queryFn: () => services.upload.listMyUploads(page, pageSize, debouncedKeyword || undefined),
  })

  const files = data?.items ?? []
  const total = data?.total ?? 0
  const totalPages = Math.ceil(total / pageSize)

  // 上传文件 Mutation
  const uploadMutation = useMutation({
    mutationFn: async (file: File) => {
      return services.upload.uploadFile(file, "generic", undefined, uploadAccessMode)
    },
    onMutate: () => {
      setUploading(true)
    },
    onSuccess: () => {
      toast.success("文件上传成功")
      void queryClient.invalidateQueries({ queryKey: ["user-files"] })
    },
    onError: (err: Error) => {
      toast.error(err.message || "上传失败")
    },
    onSettled: () => {
      setUploading(false)
      if (fileInputRef.current) {
        fileInputRef.current.value = ""
      }
    }
  })

  // 删除文件 Mutation
  const deleteMutation = useMutation({
    mutationFn: (id: string) => services.upload.deleteMyFile(id),
    onSuccess: () => {
      toast.success("文件已成功删除")
      void queryClient.invalidateQueries({ queryKey: ["user-files"] })
      setDeleteTarget(null)
    },
    onError: (err: Error) => {
      toast.error(err.message || "删除失败")
    }
  })

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) {
      uploadMutation.mutate(file)
    }
  }

  const triggerUploadClick = () => {
    fileInputRef.current?.click()
  }

  const handleDownload = (file: UploadRecord) => {
    const url = services.upload.getDownloadUrl(file.id)
    const a = document.createElement("a")
    a.href = url
    a.download = file.file_name
    a.click()
  }

  return (
    <div className="flex w-full flex-col gap-6">
      {/* 操作栏：搜索、权限选择、上传按钮 */}
      <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-4">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
          <Input
            placeholder="搜索文件名..."
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            className="pl-9 h-9"
          />
        </div>

        <div className="flex items-center gap-3 self-end sm:self-auto">
          {/* 上传权限设置 */}
          <div className="flex items-center gap-1.5 border rounded-lg p-1 bg-muted/40 h-9 text-xs">
            <Button
              variant={uploadAccessMode === 0 ? "secondary" : "ghost"}
              size="sm"
              className="h-7 px-2.5 text-xs font-normal"
              onClick={() => setUploadAccessMode(0)}
            >
              <Lock className="mr-1 size-3 text-amber-500" />
              私有
            </Button>
            <Button
              variant={uploadAccessMode === 1 ? "secondary" : "ghost"}
              size="sm"
              className="h-7 px-2.5 text-xs font-normal"
              onClick={() => setUploadAccessMode(1)}
            >
              <Globe className="mr-1 size-3 text-sky-500" />
              公开
            </Button>
          </div>

          <input
            type="file"
            ref={fileInputRef}
            onChange={handleFileChange}
            className="hidden"
            disabled={uploading}
          />
          <Button
            onClick={triggerUploadClick}
            disabled={uploading}
            className="h-9 gap-1.5 text-xs font-medium"
          >
            {uploading ? (
              <Loader2 className="size-3.5 animate-spin" />
            ) : (
              <Plus className="size-3.5" />
            )}
            {uploading ? "正在上传..." : "上传文件"}
          </Button>
        </div>
      </div>

      {/* 文件展示区 */}
      {isPending ? (
        <div className="flex items-center justify-center py-24">
          <Loader2 className="size-8 animate-spin text-primary" />
        </div>
      ) : files.length === 0 ? (
        <Card className="border border-dashed py-16 flex flex-col items-center justify-center text-center">
          <CardContent className="flex flex-col items-center gap-4">
            <div className="p-4 bg-muted rounded-full text-muted-foreground/60">
              <Upload className="size-10" />
            </div>
            <div className="space-y-1">
              <h3 className="font-semibold text-sm">还没有上传文件</h3>
              <p className="text-xs text-muted-foreground max-w-xs">
                {debouncedKeyword ? "未搜索到匹配的文件" : "上传您的第一个文件，支持图片、视频、文档和压缩包等格式。"}
              </p>
            </div>
            {!debouncedKeyword && (
              <Button onClick={triggerUploadClick} disabled={uploading} size="sm" className="mt-2 text-xs">
                选择文件上传
              </Button>
            )}
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
          {files.map((file) => (
            <Card
              key={file.id}
              className="group border border-border/40 hover:border-border/80 bg-card/60 hover:bg-card/95 transition-all duration-300 shadow-xs hover:shadow-md overflow-hidden relative flex flex-col h-full"
            >
              {/* 文件预览/图标区 */}
              <div className="h-36 bg-muted/30 border-b border-dashed relative flex items-center justify-center overflow-hidden p-2 group-hover:bg-muted/10 transition-colors">
                {file.mime_type.startsWith("image/") ? (
                  <FileImagePreview
                    fileId={file.id}
                    alt={file.file_name}
                    quality="medium"
                    className="max-h-full max-w-full object-contain rounded-md shadow-xs transition-transform duration-350 group-hover:scale-103"
                    fallbackClassName="min-h-full w-full rounded-md"
                  />
                ) : (
                  getFileIcon(file.mime_type)
                )}

                {/* 访问权限 Badge */}
                <div className="absolute top-2 right-2">
                  {file.access_mode === 0 ? (
                    <Badge variant="secondary" className="bg-amber-500/10 text-amber-600 dark:text-amber-400 text-[10px] gap-1 px-1.5 py-0 rounded-md border border-amber-500/20 font-normal">
                      <Lock className="size-2.5" />
                      私有
                    </Badge>
                  ) : (
                    <Badge variant="secondary" className="bg-sky-500/10 text-sky-600 dark:text-sky-400 text-[10px] gap-1 px-1.5 py-0 rounded-md border border-sky-500/20 font-normal">
                      <Globe className="size-2.5" />
                      公开
                    </Badge>
                  )}
                </div>
              </div>

              {/* 文件详情区 */}
              <CardContent className="p-4 flex-1 flex flex-col justify-between gap-3">
                <div className="space-y-1">
                  <h4 className="font-semibold text-xs text-foreground truncate select-all" title={file.file_name}>
                    {file.file_name}
                  </h4>
                  <div className="flex items-center justify-between text-[10px] text-muted-foreground font-mono">
                    <span>{formatFileSize(file.file_size)}</span>
                    <span className="truncate max-w-[120px]" title={file.mime_type}>{file.mime_type}</span>
                  </div>
                </div>

                <div className="flex items-center justify-between pt-1 border-t border-dashed">
                  <span className="text-[9px] text-muted-foreground/80">{formatDate(file.created_at)}</span>

                  <div className="flex items-center gap-1">
                    <Button
                      size="icon"
                      variant="ghost"
                      className="size-7 rounded-md hover:bg-muted text-muted-foreground hover:text-foreground"
                      title="下载文件"
                      onClick={() => handleDownload(file)}
                    >
                      <Download className="size-3.5" />
                    </Button>
                    <Button
                      size="icon"
                      variant="ghost"
                      className="size-7 rounded-md hover:bg-destructive/10 text-muted-foreground hover:text-destructive"
                      title="删除文件"
                      onClick={() => setDeleteTarget(file)}
                    >
                      <Trash2 className="size-3.5" />
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* 分页 */}
      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-2 pt-4">
          <Button
            size="sm"
            variant="outline"
            className="border-dashed text-xs h-8 px-3"
            disabled={page <= 1}
            onClick={() => setPage((p) => p - 1)}
          >
            <ChevronLeft className="mr-1 size-3.5" />
            上一页
          </Button>
          <span className="text-xs text-muted-foreground font-medium px-2">
            {page} / {totalPages}
          </span>
          <Button
            size="sm"
            variant="outline"
            className="border-dashed text-xs h-8 px-3"
            disabled={page >= totalPages}
            onClick={() => setPage((p) => p + 1)}
          >
            下一页
            <ChevronRight className="ml-1 size-3.5" />
          </Button>
        </div>
      )}

      {/* 删除确认 Dialog */}
      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <AlertDialogContent suppressHydrationWarning>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除文件</AlertDialogTitle>
            <AlertDialogDescription>
              确定要删除文件{" "}
              <span className="font-semibold text-foreground">「{deleteTarget?.file_name}」</span>{" "}
              吗？删除后此文件将无法恢复。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteMutation.isPending}>取消</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}
              disabled={deleteMutation.isPending}
              className="bg-destructive hover:bg-destructive/90 text-destructive-foreground"
            >
              {deleteMutation.isPending && <Loader2 className="mr-1.5 size-3.5 animate-spin" />}
              确认删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
