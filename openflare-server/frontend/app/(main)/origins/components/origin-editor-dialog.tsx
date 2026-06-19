"use client"

import {useEffect} from "react"
import {useMutation, useQueryClient} from "@tanstack/react-query"
import {zodResolver} from "@hookform/resolvers/zod"
import {useForm} from "react-hook-form"
import {Loader2} from "lucide-react"
import {toast} from "sonner"
import {z} from "zod"

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
import {Textarea} from "@/components/ui/textarea"
import {type OriginItem, type OriginMutationPayload, OriginService} from "@/lib/services/openflare"

const originSchema = z.object({
  name: z.string().max(255),
  address: z
    .string()
    .trim()
    .min(1, "请输入源站地址")
    .refine(
      (value) => !/[/?#]/.test(value) && !value.includes("://"),
      "源站地址格式不合法",
    ),
  remark: z.string().max(255),
})

type OriginFormValues = z.infer<typeof originSchema>

const originsQueryKey = ["openflare", "origins"] as const

function toFormValues(origin?: OriginItem | null): OriginFormValues {
  if (!origin) return { name: "", address: "", remark: "" }
  return {
    name: origin.name,
    address: origin.address,
    remark: origin.remark || "",
  }
}

function toPayload(values: OriginFormValues): OriginMutationPayload {
  return {
    name: values.name.trim(),
    address: values.address.trim(),
    remark: values.remark.trim(),
  }
}

interface OriginEditorDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  origin?: OriginItem | null
  onSaved?: () => void
}

export function OriginEditorDialog({
  open,
  onOpenChange,
  origin,
  onSaved,
}: OriginEditorDialogProps) {
  const queryClient = useQueryClient()
  const form = useForm<OriginFormValues>({
    resolver: zodResolver(originSchema),
    defaultValues: toFormValues(origin),
  })

  useEffect(() => {
    if (open) form.reset(toFormValues(origin))
  }, [form, origin, open])

  const mutation = useMutation({
    mutationFn: async (values: OriginFormValues) => {
      const payload = toPayload(values)
      return origin
        ? OriginService.update(origin.id, payload)
        : OriginService.create(payload)
    },
    onSuccess: async () => {
      toast.success(origin ? "源站已更新" : "源站已创建")
      await queryClient.invalidateQueries({ queryKey: originsQueryKey })
      if (origin) {
        await queryClient.invalidateQueries({
          queryKey: ["openflare", "origins", String(origin.id)],
        })
      }
      onSaved?.()
      onOpenChange(false)
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : "保存失败")
    },
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{origin ? "编辑源站" : "新增源站"}</DialogTitle>
          <DialogDescription>
            源站作为规则里的可复用地址目录，协议和端口仍由规则决定。
          </DialogDescription>
        </DialogHeader>

        <form
          id="origin-editor-form"
          className="space-y-4"
          onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
        >
          <div className="space-y-1.5">
            <Label htmlFor="address">源站地址</Label>
            <Input
              id="address"
              placeholder="origin.internal"
              {...form.register("address")}
            />
            {form.formState.errors.address ? (
              <p className="text-xs text-destructive">
                {form.formState.errors.address.message}
              </p>
            ) : null}
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="name">源站名</Label>
            <Input id="name" placeholder="主站源站" {...form.register("name")} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="remark">备注</Label>
            <Textarea id="remark" rows={2} {...form.register("remark")} />
          </div>
        </form>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button
            type="submit"
            form="origin-editor-form"
            disabled={mutation.isPending}
          >
            {mutation.isPending ? (
              <>
                <Loader2 className="size-4 animate-spin mr-1" />
                保存中...
              </>
            ) : origin ? (
              "保存修改"
            ) : (
              "新增源站"
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
