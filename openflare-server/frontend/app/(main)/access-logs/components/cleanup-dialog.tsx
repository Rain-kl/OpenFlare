"use client"

import {useState} from "react"
import {Loader2} from "lucide-react"

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
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue,} from "@/components/ui/select"

const CLEANUP_PRESETS = [3, 7, 30]

interface CleanupDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: (retentionDays: number) => void
  loading: boolean
}

export function CleanupDialog({
  open,
  onOpenChange,
  onConfirm,
  loading,
}: CleanupDialogProps) {
  const [mode, setMode] = useState<string>("7")
  const [customDays, setCustomDays] = useState("14")
  const [error, setError] = useState<string | null>(null)

  const handleConfirm = () => {
    const retentionDays =
      mode === "custom"
        ? Number.parseInt(customDays, 10)
        : Number.parseInt(mode, 10)

    if (!Number.isFinite(retentionDays) || retentionDays < 1) {
      setError("保留天数必须大于 0")
      return
    }

    setError(null)
    onConfirm(retentionDays)
  }

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>清理访问日志</AlertDialogTitle>
          <AlertDialogDescription>
            删除早于指定保留天数的访问日志记录，操作不可恢复。
          </AlertDialogDescription>
        </AlertDialogHeader>

        <div className="space-y-3">
          <div className="space-y-1.5">
            <Label>保留策略</Label>
            <Select value={mode} onValueChange={setMode}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {CLEANUP_PRESETS.map((days) => (
                  <SelectItem key={days} value={String(days)}>
                    保留最近 {days} 天
                  </SelectItem>
                ))}
                <SelectItem value="custom">自定义天数</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {mode === "custom" ? (
            <div className="space-y-1.5">
              <Label htmlFor="customDays">自定义保留天数</Label>
              <Input
                id="customDays"
                type="number"
                min={1}
                value={customDays}
                onChange={(e) => setCustomDays(e.target.value)}
                disabled={loading}
              />
            </div>
          ) : null}

          {error ? <p className="text-xs text-destructive">{error}</p> : null}
        </div>

        <AlertDialogFooter>
          <AlertDialogCancel disabled={loading}>取消</AlertDialogCancel>
          <Button variant="destructive" onClick={handleConfirm} disabled={loading}>
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
