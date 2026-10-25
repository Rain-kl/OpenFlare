'use client';

import { useTranslations } from 'next-intl';
import * as React from 'react';
import { useState } from 'react';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  InputOTP,
  InputOTPGroup,
  InputOTPSlot,
} from '@/components/ui/input-otp';
import { Button } from '@/components/ui/button';
import { Spinner } from '@/components/ui/spinner';

interface PasswordDialogProps {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: (password: string) => void;
  loading?: boolean;
  title?: string;
  description?: string;
}

/**
 * 密码输入对话框
 *
 * 用于敏感操作前的二次确认
 */
export function PasswordDialog({
  isOpen,
  onOpenChange,
  onConfirm,
  loading = false,
  title = undefined,
  description = undefined,
}: PasswordDialogProps) {
  const rt = useTranslations('runtime');
  title ??= rt('enter_password_728a7b60');
  description ??= rt('for_your_account_security_please_enter_the_28fc0bbd');

  const [password, setPassword] = useState('');

  const handleConfirm = () => {
    if (password.length === 6) {
      onConfirm(password);
    }
  };

  /* 当对话框关闭时重置密码*/
  React.useEffect(() => {
    if (!isOpen) {
      setPassword('');
    }
  }, [isOpen]);

  return (
    <Dialog open={isOpen} onOpenChange={onOpenChange}>
      <DialogContent className='w-xs sm:w-md'>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>

        <div className='py-2 flex justify-center'>
          <InputOTP
            maxLength={6}
            value={password}
            onChange={setPassword}
            disabled={loading}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && password.length === 6 && !loading) {
                handleConfirm();
              }
            }}
          >
            <InputOTPGroup className='gap-2'>
              <InputOTPSlot index={0} className='w-10 h-10 text-base' />
              <InputOTPSlot index={1} className='w-10 h-10 text-base' />
              <InputOTPSlot index={2} className='w-10 h-10 text-base' />
              <InputOTPSlot index={3} className='w-10 h-10 text-base' />
              <InputOTPSlot index={4} className='w-10 h-10 text-base' />
              <InputOTPSlot index={5} className='w-10 h-10 text-base' />
            </InputOTPGroup>
          </InputOTP>
        </div>

        <div className='flex justify-end gap-2 pt-2'>
          <Button
            variant='ghost'
            onClick={() => onOpenChange(false)}
            disabled={loading}
            className='h-8 text-xs'
          >
            {rt('cancel_2cd0f3be')}
          </Button>
          <Button
            type='button'
            className='bg-primary hover:bg-primary/90 h-8 text-xs'
            onClick={handleConfirm}
            disabled={password.length !== 6 || loading}
          >
            {loading ? (
              <>
                <Spinner className='mr-2 h-4 w-4' />
                {rt('verifying_6c128e0a')}
              </>
            ) : (
              rt('confirm_36f33ada')
            )}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
