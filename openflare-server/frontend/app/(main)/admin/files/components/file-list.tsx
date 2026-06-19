"use client"

import * as React from "react"
import {useMutation, useQuery, useQueryClient} from "@tanstack/react-query"
import {AnimatePresence, motion} from "motion/react"
import {
  Download,
  Eye,
  FileArchive,
  FileAudio,
  FileImage,
  FileText,
  FileVideo,
  Loader2,
  Search,
  Trash2,
  Upload,
  X,
} from "lucide-react"
import {toast} from "sonner"

import {Button} from "@/components/ui/button"
import {Input} from "@/components/ui/input"
import {Badge} from "@/components/ui/badge"
import {Checkbox} from "@/components/ui/checkbox"
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow} from "@/components/ui/table"
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
import {Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle} from "@/components/ui/sheet"
import {FileImagePreview} from "@/components/common/file-image-preview"
import services, {formatFileSize} from "@/lib/services"
import type {Upload as UploadRecord} from "@/lib/services/upload/types"

/* ─── 工具函数 ─────────────────────────────────────────── */

function getFileIcon(mimeType: string, className = "size-8") {
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

export function FileList() {
  const queryClient = useQueryClient()
  const [keyword, setKeyword] = React.useState("")
  const [debouncedKeyword, setDebouncedKeyword] = React.useState("")
  const [selectedIds, setSelectedIds] = React.useState<Set<string>>(new Set())
  const [deleteTarget, setDeleteTarget] = React.useState<UploadRecord | null>(null)
  const [detailTarget, setDetailTarget] = React.useState<UploadRecord | null>(null)
  const [page, setPage] = React.useState(1)
  const pageSize = 15 // 表格视图下 15 条更紧凑

  // 搜索防抖
  React.useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedKeyword(keyword)
      setPage(1)
    }, 400)
    return () => clearTimeout(timer)
  }, [keyword])

  // 文件列表查询
  const listQuery = useQuery({
    queryKey: ["files", "all", page, pageSize, debouncedKeyword],
    queryFn: () => services.adminUpload.listUploads(page, pageSize, debouncedKeyword || undefined),
  })

  const storageDriverQuery = useQuery({
    queryKey: ["admin", "storage-config", "driver"],
    queryFn: async () => {
      const record = await services.adminSystemConfig.getSystemConfig("storage_config")
      const cfg = JSON.parse(record.value) as {driver?: string}
      return cfg.driver ?? "local"
    },
  })

  const files = listQuery.data?.items ?? []
  const total = listQuery.data?.total ?? 0
  const totalPages = Math.ceil(total / pageSize)

  const isAllSelected = files.length > 0 && files.every((f) => selectedIds.has(f.id))
  const isSomeSelected = files.length > 0 && files.some((f) => selectedIds.has(f.id)) && !isAllSelected

  const handleSelectAll = () => {
    if (isAllSelected) {
      setSelectedIds((prev) => {
        const next = new Set(prev)
        files.forEach((f) => next.delete(f.id))
        return next
      })
    } else {
      setSelectedIds((prev) => {
        const next = new Set(prev)
        files.forEach((f) => next.add(f.id))
        return next
      })
    }
  }

  // 删除单文件
  const deleteMutation = useMutation({
    mutationFn: (id: string) => services.adminUpload.deleteFile(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["files", "my"] })
      void queryClient.invalidateQueries({ queryKey: ["files", "stats"] })
      toast.success("文件已删除")
      setDeleteTarget(null)
    },
    onError: (err: Error) => toast.error(err.message || "删除失败"),
  })

  // 批量 ZIP 下载
  const batchDownloadMutation = useMutation({
    mutationFn: (ids: string[]) => services.adminUpload.batchDownload(ids),
    onSuccess: (blob) => {
      const url = URL.createObjectURL(blob)
      const a = document.createElement("a")
      a.href = url
      a.download = "batch_download.zip"
      a.click()
      URL.revokeObjectURL(url)
      toast.success("批量下载已开始")
    },
    onError: () => toast.error("批量下载失败"),
  })

  const toggleSelect = (id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  const clearSelection = () => setSelectedIds(new Set())

  const selectAll = () => setSelectedIds(new Set(files.map((f) => f.id)))

  const handleDownload = (file: UploadRecord) => {
    const url = services.adminUpload.getDownloadUrl(file.id)
    const a = document.createElement("a")
    a.href = url
    a.download = file.file_name
    a.click()
  }

  return (
    <div className="space-y-6">
      {/* 搜索栏 */}
      <div className="flex items-center gap-3">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-3.5 text-muted-foreground" />
          <Input
            placeholder="搜索文件名..."
            className="pl-8 h-8 text-xs border-dashed rounded-lg focus-visible:ring-0"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
          />
          {keyword && (
            <button
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              onClick={() => setKeyword("")}
            >
              <X className="size-3" />
            </button>
          )}
        </div>
        {files.length > 0 && (
          <Button
            size="sm"
            variant="ghost"
            className="text-xs h-8 border border-dashed text-muted-foreground"
            onClick={selectedIds.size === files.length ? clearSelection : selectAll}
          >
            {selectedIds.size === files.length ? "取消全选" : "全选本页"}
          </Button>
        )}

        {/* 打包下载等批量操作 */}
        <div className="flex items-center gap-2 ml-auto shrink-0">
          <AnimatePresence>
            {selectedIds.size > 0 && (
              <motion.div
                initial={{ opacity: 0, scale: 0.9 }}
                animate={{ opacity: 1, scale: 1 }}
                exit={{ opacity: 0, scale: 0.9 }}
                className="flex items-center gap-2"
              >
                <Badge variant="secondary" className="text-xs px-2.5">
                  已选 {selectedIds.size} 个
                </Badge>
                <Button
                  size="sm"
                  variant="outline"
                  className="border-dashed text-xs h-8"
                  onClick={() => batchDownloadMutation.mutate([...selectedIds])}
                  disabled={batchDownloadMutation.isPending}
                >
                  {batchDownloadMutation.isPending ? (
                    <Loader2 className="size-3.5 mr-1 animate-spin" />
                  ) : (
                    <FileArchive className="size-3.5 mr-1" />
                  )}
                  打包下载
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  className="text-xs h-8 px-2"
                  onClick={clearSelection}
                >
                  <X className="size-3.5" />
                </Button>
              </motion.div>
            )}
          </AnimatePresence>
        </div>

        {total > 0 && (
          <span className="text-xs text-muted-foreground shrink-0">共 {total} 个文件</span>
        )}
      </div>

      {/* 文件列表 Table */}
      {listQuery.isPending ? (
        <div className="flex items-center justify-center py-20">
          <Loader2 className="size-6 animate-spin text-sky-500" />
        </div>
      ) : files.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 gap-4 text-muted-foreground">
          <Upload className="size-12 text-muted-foreground/30" />
          <p className="text-sm">
            {debouncedKeyword ? "没有匹配的文件" : "您还没有上传任何文件"}
          </p>
        </div>
      ) : (
        <div className="border border-dashed rounded-xl bg-card overflow-hidden">
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent border-dashed">
                <TableHead className="w-[50px] pl-4 py-3">
                  <Checkbox
                    checked={isAllSelected || (isSomeSelected ? "indeterminate" : false)}
                    onCheckedChange={handleSelectAll}
                  />
                </TableHead>
                <TableHead className="w-[80px] py-3">预览</TableHead>
                <TableHead className="w-[180px] py-3">ID</TableHead>
                <TableHead className="py-3">文件名</TableHead>
                <TableHead className="max-w-[200px] truncate py-3">路径</TableHead>
                <TableHead className="w-[100px] py-3">业务类别</TableHead>
                <TableHead className="w-[125px] py-3">MIME类型</TableHead>
                <TableHead className="w-[100px] py-3">大小</TableHead>
                <TableHead className="w-[150px] py-3">上传时间</TableHead>
                <TableHead className="w-[120px] text-right pr-4 py-3">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {files.map((file) => {
                const isSelected = selectedIds.has(file.id)
                return (
                  <TableRow
                    key={file.id}
                    className={`border-dashed hover:bg-muted/30 transition-colors ${
                      isSelected ? "bg-sky-500/5 hover:bg-sky-500/10" : ""
                    }`}
                  >
                    <TableCell className="pl-4 py-3">
                      <Checkbox
                        checked={isSelected}
                        onCheckedChange={() => toggleSelect(file.id)}
                      />
                    </TableCell>
                    <TableCell className="py-3">
                      <div className="flex items-center justify-center size-9 rounded-lg bg-muted/40 overflow-hidden border">
                        {file.mime_type.startsWith("image/") ? (
                          <FileImagePreview
                            fileId={file.id}
                            alt={file.file_name}
                            quality="low"
                            variant="compact"
                            className="size-full object-cover"
                          />
                        ) : (
                          getFileIcon(file.mime_type, "size-4.5")
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="py-3 font-mono text-[11px] text-muted-foreground select-all">
                      {file.id}
                    </TableCell>
                    <TableCell className="py-3 font-medium max-w-[180px] truncate text-xs" title={file.file_name}>
                      {file.file_name}
                    </TableCell>
                    <TableCell className="py-3 font-mono text-[11px] max-w-[200px] truncate text-muted-foreground select-all" title={file.file_path}>
                      {file.file_path}
                    </TableCell>
                    <TableCell className="py-3">
                      <Badge variant="secondary" className="text-[10px] py-0 px-1.5 font-normal rounded-md">
                        {file.type}
                      </Badge>
                    </TableCell>
                    <TableCell className="py-3 text-[11px] text-muted-foreground truncate max-w-[125px]" title={file.mime_type}>
                      {file.mime_type}
                    </TableCell>
                    <TableCell className="py-3 text-xs">
                      {formatFileSize(file.file_size)}
                    </TableCell>
                    <TableCell className="py-3 text-xs text-muted-foreground">
                      {formatDate(file.created_at)}
                    </TableCell>
                    <TableCell className="py-3 text-right pr-4">
                      <div className="flex items-center justify-end gap-0.5">
                        <Button
                          size="icon"
                          variant="ghost"
                          className="h-7 w-7 rounded-md hover:bg-muted text-muted-foreground hover:text-foreground"
                          title="查看详情"
                          onClick={() => setDetailTarget(file)}
                        >
                          <Eye className="size-3.5" />
                        </Button>
                        <Button
                          size="icon"
                          variant="ghost"
                          className="h-7 w-7 rounded-md hover:bg-muted"
                          title="下载"
                          onClick={() => handleDownload(file)}
                        >
                          <Download className="size-3.5" />
                        </Button>
                        <Button
                          size="icon"
                          variant="ghost"
                          className="h-7 w-7 rounded-md hover:bg-destructive/10 text-muted-foreground hover:text-destructive"
                          title="删除"
                          onClick={() => setDeleteTarget(file)}
                        >
                          <Trash2 className="size-3.5" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>
      )}

      {/* 分页 */}
      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-2 pt-2">
          <Button
            size="sm"
            variant="outline"
            className="border-dashed text-xs h-7 px-3"
            disabled={page <= 1}
            onClick={() => setPage((p) => p - 1)}
          >
            上一页
          </Button>
          <span className="text-xs text-muted-foreground">
            {page} / {totalPages}
          </span>
          <Button
            size="sm"
            variant="outline"
            className="border-dashed text-xs h-7 px-3"
            disabled={page >= totalPages}
            onClick={() => setPage((p) => p + 1)}
          >
            下一页
          </Button>
        </div>
      )}

      {/* 详情查看 Sheet */}
      <Sheet open={!!detailTarget} onOpenChange={(open) => !open && setDetailTarget(null)}>
        <SheetContent className="w-full sm:max-w-md overflow-y-auto p-6">
          <SheetHeader className="border-b pb-4 px-0">
            <SheetTitle>文件详情</SheetTitle>
            <SheetDescription>查看文件的完整元数据与配置属性。</SheetDescription>
          </SheetHeader>

          {detailTarget && (
            <div className="space-y-6 py-4">
              {/* 大图/格式预览 */}
              <div className="flex items-center justify-center h-48 rounded-xl bg-muted/30 border border-dashed overflow-hidden p-2">
                {detailTarget.mime_type.startsWith("image/") ? (
                  <FileImagePreview
                    fileId={detailTarget.id}
                    alt={detailTarget.file_name}
                    quality="low"
                    className="max-h-full max-w-full object-contain rounded-lg shadow-sm"
                    fallbackClassName="min-h-32 w-full rounded-lg"
                  />
                ) : (
                  <div className="flex flex-col items-center gap-3">
                    {getFileIcon(detailTarget.mime_type, "size-14")}
                    <span className="text-xs font-medium text-muted-foreground uppercase">
                      {detailTarget.extension} 文件
                    </span>
                  </div>
                )}
              </div>

              {/* 元数据列表 */}
              <div className="space-y-4 text-xs">
                <div className="grid grid-cols-3 gap-2 border-b border-dashed pb-2">
                  <span className="text-muted-foreground font-medium">文件 ID</span>
                  <span className="col-span-2 font-mono break-all select-all text-foreground/90">{detailTarget.id}</span>
                </div>

                <div className="grid grid-cols-3 gap-2 border-b border-dashed pb-2">
                  <span className="text-muted-foreground font-medium">文件名</span>
                  <span className="col-span-2 break-all font-medium text-foreground/90">{detailTarget.file_name}</span>
                </div>

                <div className="grid grid-cols-3 gap-2 border-b border-dashed pb-2">
                  <span className="text-muted-foreground font-medium">存储路径</span>
                  <span className="col-span-2 font-mono break-all select-all text-foreground/90">{detailTarget.file_path}</span>
                </div>

                <div className="grid grid-cols-3 gap-2 border-b border-dashed pb-2">
                  <span className="text-muted-foreground font-medium">大小</span>
                  <span className="col-span-2 text-foreground/90">
                    {formatFileSize(detailTarget.file_size)} ({detailTarget.file_size.toLocaleString()} 字节)
                  </span>
                </div>

                <div className="grid grid-cols-3 gap-2 border-b border-dashed pb-2">
                  <span className="text-muted-foreground font-medium">MIME 类型</span>
                  <span className="col-span-2 font-mono text-foreground/90">{detailTarget.mime_type}</span>
                </div>

                <div className="grid grid-cols-3 gap-2 border-b border-dashed pb-2">
                  <span className="text-muted-foreground font-medium">文件后缀</span>
                  <span className="col-span-2 font-mono text-foreground/90">{detailTarget.extension}</span>
                </div>

                <div className="grid grid-cols-3 gap-2 border-b border-dashed pb-2">
                  <span className="text-muted-foreground font-medium">Hash (SHA-256)</span>
                  <span className="col-span-2 font-mono break-all select-all text-foreground/90">{detailTarget.hash}</span>
                </div>

                <div className="grid grid-cols-3 gap-2 border-b border-dashed pb-2">
                  <span className="text-muted-foreground font-medium">业务类型</span>
                  <span className="col-span-2">
                    <Badge variant="secondary" className="text-[10px] py-0 px-2 rounded-md font-normal">
                      {detailTarget.type}
                    </Badge>
                  </span>
                </div>

                <div className="grid grid-cols-3 gap-2 border-b border-dashed pb-2">
                  <span className="text-muted-foreground font-medium">存储驱动</span>
                  <span className="col-span-2 font-mono text-foreground/90">{storageDriverQuery.data ?? "local"}</span>
                </div>

                <div className="grid grid-cols-3 gap-2 border-b border-dashed pb-2">
                  <span className="text-muted-foreground font-medium">上传者 ID</span>
                  <span className="col-span-2 font-mono text-foreground/90">{detailTarget.user_id}</span>
                </div>

                <div className="grid grid-cols-3 gap-2 border-b border-dashed pb-2">
                  <span className="text-muted-foreground font-medium">上传时间</span>
                  <span className="col-span-2 text-foreground/90">{formatDate(detailTarget.created_at)}</span>
                </div>

                {detailTarget.metadata && Object.keys(detailTarget.metadata).length > 0 && (
                  <div className="space-y-1.5 pt-2">
                    <span className="text-muted-foreground font-medium block">额外元数据</span>
                    <pre className="p-3 bg-muted/40 border rounded-lg text-[10px] font-mono overflow-x-auto text-foreground/90 max-h-40 overflow-y-auto">
                      {JSON.stringify(detailTarget.metadata, null, 2)}
                    </pre>
                  </div>
                )}
              </div>
            </div>
          )}
        </SheetContent>
      </Sheet>

      {/* 删除确认 Dialog */}
      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除文件</AlertDialogTitle>
            <AlertDialogDescription>
              确定要删除文件{" "}
              <span className="font-semibold text-foreground">「{deleteTarget?.file_name}」</span>{" "}
              吗？此操作不可撤销。
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
