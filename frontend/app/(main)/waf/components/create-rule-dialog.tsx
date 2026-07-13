'use client';

import {useEffect, useState} from 'react';

import {Button} from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {Field, FieldGroup, FieldLabel} from '@/components/ui/field';
import {Input} from '@/components/ui/input';
import {Spinner} from '@/components/ui/spinner';

interface CreateRuleDialogProps {
  open: boolean;
  pending: boolean;
  onOpenChange: (open: boolean) => void;
  onCreate: (name: string) => Promise<void>;
}

export function CreateRuleDialog({
  open,
  pending,
  onOpenChange,
  onCreate,
}: CreateRuleDialogProps) {
  const [name, setName] = useState('');

  useEffect(() => {
    if (!open) setName('');
  }, [open]);

  const trimmedName = name.trim();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form
          className='flex flex-col gap-4'
          onSubmit={async (event) => {
            event.preventDefault();
            if (!trimmedName || pending) return;
            await onCreate(trimmedName);
          }}
        >
          <DialogHeader>
            <DialogTitle>新建 WAF 规则</DialogTitle>
            <DialogDescription>
              创建后将进入编排页面，通过处理单元配置规则执行流程。
            </DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor='waf-rule-name'>规则名称</FieldLabel>
              <Input
                id='waf-rule-name'
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder='例如：入口防护'
                autoComplete='off'
                autoFocus
              />
            </Field>
          </FieldGroup>
          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              disabled={pending}
              onClick={() => onOpenChange(false)}
            >
              取消
            </Button>
            <Button type='submit' disabled={!trimmedName || pending}>
              {pending ? <Spinner data-icon='inline-start' /> : null}
              创建并编排
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
