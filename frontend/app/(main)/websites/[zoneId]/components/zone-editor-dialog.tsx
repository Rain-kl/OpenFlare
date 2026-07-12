'use client'

import {useEffect} from 'react'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {zodResolver} from '@hookform/resolvers/zod'
import {useForm} from 'react-hook-form'
import {Loader2} from 'lucide-react'
import {toast} from 'sonner'
import {z} from 'zod'
import {Button} from '@/components/ui/button'
import {Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle} from '@/components/ui/dialog'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {ZoneService, zoneQueryKey, type ZoneItem} from '@/lib/services/openflare'

const schema = z.object({
  domain: z
    .string()
    .trim()
    .min(1, '请输入 Zone 根域')
    .refine((value) => !/[*/?#@]|:\/\//.test(value), '请输入不含协议或通配符的根域'),
})
type Values = z.infer<typeof schema>

export function ZoneEditorDialog({
  open,
  onOpenChange,
  zone,
}: {
  open: boolean
  onOpenChange(open: boolean): void
  zone?: ZoneItem | null
}) {
  const queryClient = useQueryClient()
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {domain: ''},
  })
  useEffect(() => {
    if (open) form.reset({domain: zone?.domain ?? ''})
  }, [form, open, zone])
  const mutation = useMutation({
    mutationFn: (values: Values) =>
      zone
        ? ZoneService.update(zone.id, {domain: values.domain.toLowerCase()})
        : ZoneService.create({domain: values.domain.toLowerCase()}),
    onSuccess: async () => {
      toast.success(zone ? 'Zone 已更新' : 'Zone 已创建')
      await queryClient.invalidateQueries({queryKey: zoneQueryKey})
      onOpenChange(false)
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : '保存失败'),
  })
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{zone ? '编辑 Zone' : '新增 Zone'}</DialogTitle>
          <DialogDescription>Zone 仅接受可注册根域，例如 arctel.de。</DialogDescription>
        </DialogHeader>
        <form
          id="zone-editor"
          className="space-y-4"
          onSubmit={form.handleSubmit((values) =>
            mutation.mutate({domain: values.domain.toLowerCase()}),
          )}
        >
          <div className="space-y-1.5">
            <Label htmlFor="zone-domain">根域</Label>
            <Input id="zone-domain" placeholder="arctel.de" {...form.register('domain')} />
            {form.formState.errors.domain && (
              <p className="text-xs text-destructive">{form.formState.errors.domain.message}</p>
            )}
          </div>
        </form>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button form="zone-editor" type="submit" disabled={mutation.isPending}>
            {mutation.isPending && <Loader2 className="mr-1 size-4 animate-spin" />}
            {zone ? '保存修改' : '新增 Zone'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
