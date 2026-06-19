"use client"

import {useRef, useState} from "react"
import {useMutation, useQueryClient} from "@tanstack/react-query"
import {Loader2, UploadCloud} from "lucide-react"
import {toast} from "sonner"

import {Button} from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {Input} from "@/components/ui/input"
import {Label} from "@/components/ui/label"
import {Progress} from "@/components/ui/progress"
import {PagesService} from "@/lib/services/openflare"
import {cn} from "@/lib/utils"

import {deploymentsQueryKey, formatBytes, projectQueryKey, projectsQueryKey} from "./pages-utils"

interface DeploymentUploadDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  projectId: number
}

export function DeploymentUploadDialog({
  open,
  onOpenChange,
  projectId,
}: DeploymentUploadDialogProps) {
  const queryClient = useQueryClient()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [file, setFile] = useState<File | null>(null)
  const [isDragActive, setIsDragActive] = useState(false)
  const [uploadProgress, setUploadProgress] = useState<number | null>(null)

  const resetForm = () => {
    setFile(null)
    setIsDragActive(false)
    setUploadProgress(null)
    if (fileInputRef.current) fileInputRef.current.value = ""
  }

  const handleClose = (nextOpen: boolean) => {
    if (!nextOpen) resetForm()
    onOpenChange(nextOpen)
  }

  const uploadMutation = useMutation({
    mutationFn: () => {
      if (!file) throw new Error("请选择 zip 部署包")
      return PagesService.uploadDeployment(projectId, {
        file,
        onProgress: setUploadProgress,
      })
    },
    onSuccess: async () => {
      toast.success("部署包上传成功")
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: deploymentsQueryKey(projectId) }),
        queryClient.invalidateQueries({ queryKey: projectQueryKey(projectId) }),
        queryClient.invalidateQueries({ queryKey: projectsQueryKey }),
      ])
      handleClose(false)
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : "上传失败")
      setUploadProgress(null)
    },
  })

  const handleFileSelect = (selected: File | null) => {
    if (!selected) return
    if (!selected.name.toLowerCase().endsWith(".zip")) {
      toast.error("仅支持 zip 格式的文件")
      return
    }
    setFile(selected)
  }

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>上传部署包</DialogTitle>
          <DialogDescription>上传已构建的 zip 静态资源包，部署后可在列表中激活。</DialogDescription>
        </DialogHeader>

        <div
          className={cn(
            "rounded-lg border border-dashed p-8 text-center transition",
            isDragActive ? "border-primary bg-primary/5" : "bg-muted/20",
          )}
          onDragEnter={(e) => {
            e.preventDefault()
            setIsDragActive(true)
          }}
          onDragOver={(e) => e.preventDefault()}
          onDragLeave={(e) => {
            e.preventDefault()
            setIsDragActive(false)
          }}
          onDrop={(e) => {
            e.preventDefault()
            setIsDragActive(false)
            handleFileSelect(e.dataTransfer.files[0] ?? null)
          }}
        >
          <UploadCloud className="size-8 mx-auto text-muted-foreground" />
          <p className="mt-3 text-sm">拖拽 zip 文件到此处，或点击选择文件</p>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="mt-3"
            onClick={() => fileInputRef.current?.click()}
          >
            选择文件
          </Button>
          <input
            ref={fileInputRef}
            type="file"
            accept=".zip"
            className="hidden"
            onChange={(e) => handleFileSelect(e.target.files?.[0] ?? null)}
          />
        </div>

        {file ? (
          <div className="rounded-lg border border-dashed px-4 py-3 text-sm">
            <p className="font-medium">{file.name}</p>
            <p className="text-xs text-muted-foreground mt-1">{formatBytes(file.size)}</p>
          </div>
        ) : null}

        {uploadProgress !== null ? (
          <div className="space-y-1.5">
            <div className="flex justify-between text-xs text-muted-foreground">
              <span>上传进度</span>
              <span>{uploadProgress}%</span>
            </div>
            <Progress value={uploadProgress} />
          </div>
        ) : null}

        <div className="space-y-1.5">
          <Label htmlFor="entryFile">入口文件</Label>
          <Input id="entryFile" defaultValue="index.html" disabled />
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => handleClose(false)}>
            取消
          </Button>
          <Button
            onClick={() => uploadMutation.mutate()}
            disabled={!file || uploadMutation.isPending}
          >
            {uploadMutation.isPending ? (
              <>
                <Loader2 className="size-4 animate-spin mr-1" />
                上传中...
              </>
            ) : (
              "上传并创建部署"
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}