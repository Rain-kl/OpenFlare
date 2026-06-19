"use client"

import {useEffect} from "react"
import {zodResolver} from "@hookform/resolvers/zod"
import {Loader2} from "lucide-react"
import {useForm} from "react-hook-form"
import {z} from "zod"

import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {Button} from "@/components/ui/button"
import {Input} from "@/components/ui/input"
import {Label} from "@/components/ui/label"

const cleanupSchema = z.object({
  keepCount: z.number().int().min(3, "最少保留 3 个历史快照"),
})

type CleanupFormValues = z.infer<typeof cleanupSchema>

interface CleanupDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: (keepCount: number) => void
  loading: boolean
}

export function CleanupDialog({
  open,
  onOpenChange,
  onConfirm,
  loading,
}: CleanupDialogProps) {
  const form = useForm<CleanupFormValues>({
    resolver: zodResolver(cleanupSchema),
    defaultValues: { keepCount: 10 },
  })

  useEffect(() => {
    if (open) {
      form.reset({ keepCount: 10 })
    }
  }, [form, open])

  const handleConfirm = form.handleSubmit((values) => {
    onConfirm(values.keepCount)
  })

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>清理历史快照</AlertDialogTitle>
          <AlertDialogDescription>
            删除旧的历史快照配置，系统将始终保护当前已激活的版本不被删除。
          </AlertDialogDescription>
        </AlertDialogHeader>

        <div className="space-y-2">
          <Label htmlFor="keepCount">保留最近快照个数</Label>
          <Input
            id="keepCount"
            type="number"
            min={3}
            disabled={loading}
            {...form.register("keepCount", { valueAsNumber: true })}
          />
          <p className="text-xs text-muted-foreground">默认为 10 个，最少需保留 3 个。</p>
          {form.formState.errors.keepCount ? (
            <p className="text-xs text-destructive">{form.formState.errors.keepCount.message}</p>
          ) : null}
        </div>

        <AlertDialogFooter>
          <AlertDialogCancel disabled={loading}>取消</AlertDialogCancel>
          <Button variant="destructive" onClick={() => void handleConfirm()} disabled={loading}>
            {loading ? (
              <>
                <Loader2 className="size-4 animate-spin mr-1" />
                清理中...
              </>
            ) : (
              "确认清理"
            )}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}