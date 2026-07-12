'use client';

import {useEffect} from 'react';
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {zodResolver} from '@hookform/resolvers/zod';
import {useForm} from 'react-hook-form';
import {Loader2} from 'lucide-react';
import {toast} from 'sonner';
import {z} from 'zod';

import {Button} from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from '@/components/ui/select';
import {Textarea} from '@/components/ui/textarea';
import {TlsCertificateService, ZoneDomainService} from '@/lib/services/openflare';

const schema = z.object({
  domain: z
    .string()
    .trim()
    .min(1, '请输入完整域名')
    .refine((value) => !value.includes('*.'), 'Zone 域名不支持通配符'),
  cert_id: z.string(),
  remark: z.string().max(255),
});

type Values = z.infer<typeof schema>;

export function ZoneDomainDialog({
  open,
  onOpenChange,
  zoneId,
  onSaved,
}: {
  open: boolean;
  onOpenChange(open: boolean): void;
  zoneId: number;
  onSaved(): Promise<unknown> | void;
}) {
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {domain: '', cert_id: '', remark: ''},
  });
  const queryClient = useQueryClient();
  const certificatesQuery = useQuery({
    queryKey: ['openflare', 'tls-certificates'],
    queryFn: () => TlsCertificateService.list(),
    enabled: open,
  });

  useEffect(() => {
    if (open) {
      form.reset({domain: '', cert_id: '', remark: ''});
    }
  }, [form, open]);

  const mutation = useMutation({
    mutationFn: (values: Values) =>
      ZoneDomainService.create(zoneId, {
        domain: values.domain.toLowerCase(),
        cert_id: values.cert_id ? Number(values.cert_id) : null,
        remark: values.remark.trim(),
      }),
    onSuccess: async () => {
      toast.success('域名已添加');
      await Promise.all([
        onSaved(),
        queryClient.invalidateQueries({queryKey: ['openflare', 'zones']}),
      ]);
      onOpenChange(false);
    },
    onError: (error) =>
      toast.error(error instanceof Error ? error.message : '保存失败'),
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>添加 Zone 域名</DialogTitle>
          <DialogDescription>
            填写明确 FQDN；通配符仅可存在于所选证书的 SAN 中。
          </DialogDescription>
        </DialogHeader>
        <form
          id="zone-domain-create"
          className="space-y-4"
          onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
        >
          <div className="space-y-1.5">
            <Label htmlFor="fqdn">完整域名</Label>
            <Input id="fqdn" placeholder="api.arctel.de" {...form.register('domain')} />
            {form.formState.errors.domain ? (
              <p className="text-xs text-destructive">
                {form.formState.errors.domain.message}
              </p>
            ) : null}
          </div>
          <div className="space-y-1.5">
            <Label>证书</Label>
            <Select
              value={form.watch('cert_id') || '__none'}
              onValueChange={(value) =>
                form.setValue('cert_id', value === '__none' ? '' : value)
              }
            >
              <SelectTrigger>
                <SelectValue placeholder="不绑定证书" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__none">不绑定证书</SelectItem>
                {(certificatesQuery.data ?? []).map((certificate) => (
                  <SelectItem key={certificate.id} value={String(certificate.id)}>
                    {certificate.name} · {certificate.primary_domain}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="domain-remark">备注</Label>
            <Textarea id="domain-remark" rows={2} {...form.register('remark')} />
          </div>
        </form>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button
            type="submit"
            form="zone-domain-create"
            disabled={mutation.isPending}
          >
            {mutation.isPending ? (
              <Loader2 className="mr-1 size-4 animate-spin" />
            ) : null}
            添加域名
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
