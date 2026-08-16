'use client';

import * as React from 'react';
import { useTranslations } from 'next-intl';
import { RefreshCwIcon } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Field, FieldLabel } from '@/components/ui/field';
import {
  InputOTP,
  InputOTPGroup,
  InputOTPSeparator,
  InputOTPSlot,
} from '@/components/ui/input-otp';
import { AuthHeading } from '@/components/auth/auth-shell';
import { cn } from '@/lib/utils';

interface OTPFormProps {
  code: string;
  setCode: (val: string) => void;
  loginCodeTip: React.ReactNode;
  loginCooldown: number;
  isPending: boolean;
  onResend: () => void;
  onSubmit: () => void;
}

export function OTPForm({
  code,
  setCode,
  loginCodeTip,
  loginCooldown,
  isPending,
  onResend,
  onSubmit,
}: OTPFormProps) {
  const t = useTranslations('auth.otp');

  return (
    <div className='flex flex-col gap-6 [@media(max-height:700px)]:gap-4'>
      <AuthHeading title={t('title')} description={t('description')} />
      {loginCodeTip ? (
        <p className='text-sm leading-6 text-muted-foreground'>
          {loginCodeTip}
        </p>
      ) : null}
      <div className='flex flex-col gap-5 [@media(max-height:700px)]:gap-3'>
        <Field className='gap-3'>
          <div className='flex items-center justify-between'>
            <FieldLabel
              htmlFor='otp-verification'
              className='text-sm font-medium'
            >
              {t('code')}
            </FieldLabel>
            <Button
              variant='outline'
              size='sm'
              type='button'
              onClick={onResend}
              disabled={loginCooldown > 0 || isPending}
              className='h-8 text-xs'
            >
              <RefreshCwIcon className={cn(isPending && 'animate-spin')} />
              {loginCooldown > 0
                ? t('resendIn', { seconds: loginCooldown })
                : t('resend')}
            </Button>
          </div>
          <div className='flex justify-start'>
            <InputOTP
              maxLength={6}
              id='otp-verification'
              required
              value={code}
              onChange={setCode}
              onComplete={onSubmit}
              disabled={isPending}
            >
              <InputOTPGroup className='*:data-[slot=input-otp-slot]:h-12 *:data-[slot=input-otp-slot]:w-11 *:data-[slot=input-otp-slot]:text-xl'>
                <InputOTPSlot index={0} />
                <InputOTPSlot index={1} />
                <InputOTPSlot index={2} />
              </InputOTPGroup>
              <InputOTPSeparator className='mx-2' />
              <InputOTPGroup className='*:data-[slot=input-otp-slot]:h-12 *:data-[slot=input-otp-slot]:w-11 *:data-[slot=input-otp-slot]:text-xl'>
                <InputOTPSlot index={3} />
                <InputOTPSlot index={4} />
                <InputOTPSlot index={5} />
              </InputOTPGroup>
            </InputOTP>
          </div>
        </Field>
        <Button
          type='button'
          className='h-10 w-full [@media(max-height:700px)]:h-9'
          variant='auth'
          onClick={onSubmit}
          disabled={isPending || code.length < 6}
        >
          {isPending ? t('submitting') : t('submit')}
        </Button>
        <div className='text-center text-sm text-muted-foreground'>
          {t('help')}{' '}
          <a
            href='#'
            className='underline underline-offset-4 transition-colors hover:text-primary'
          >
            {t('contactSupport')}
          </a>
        </div>
      </div>
    </div>
  );
}
