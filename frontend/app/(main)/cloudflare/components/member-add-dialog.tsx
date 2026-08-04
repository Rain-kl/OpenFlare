'use client';

import { useEffect, useState } from 'react';

import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import type { CloudflareAvailableDomain } from '@/lib/services/openflare';

export function MemberAddDialog({
  open,
  onOpenChange,
  domains,
  defaultProxied,
  pending,
  onSubmit,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  domains: CloudflareAvailableDomain[];
  defaultProxied: boolean;
  pending: boolean;
  onSubmit: (zoneDomainID: number, proxied: boolean) => void;
}) {
  const [domainID, setDomainID] = useState('');
  const [proxied, setProxied] = useState(defaultProxied);

  useEffect(() => {
    if (!open) return;
    setDomainID('');
    setProxied(defaultProxied);
  }, [defaultProxied, open]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>添加域名成员</DialogTitle>
          <DialogDescription>
            加入后会创建或接管唯一同名 A 记录；多条同名 A 会拒绝同步。
          </DialogDescription>
        </DialogHeader>
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor='cf-domain'>Zone 域名</FieldLabel>
            <Select value={domainID} onValueChange={setDomainID}>
              <SelectTrigger id='cf-domain' className='w-full'>
                <SelectValue placeholder='选择可用域名' />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {domains.map((domain) => (
                    <SelectItem key={domain.id} value={String(domain.id)}>
                      {domain.domain}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
          <Field orientation='horizontal'>
            <FieldLabel htmlFor='cf-member-proxied'>开启橙云代理</FieldLabel>
            <Switch
              id='cf-member-proxied'
              checked={proxied}
              onCheckedChange={setProxied}
            />
          </Field>
        </FieldGroup>
        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button
            disabled={pending || !domainID}
            onClick={() => onSubmit(Number(domainID), proxied)}
          >
            添加并同步
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
