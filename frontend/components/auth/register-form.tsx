'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useRouter, useSearchParams } from 'next/navigation';
import { toast } from 'sonner';
import { useTranslations } from 'next-intl';
import Link from 'next/link';

import { useAuth } from '@/components/providers/auth-provider';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Spinner } from '@/components/ui/spinner';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { AuthHeading } from '@/components/auth/auth-shell';
import { CapWidget } from '@/components/auth/cap-widget';
import { AuthService } from '@/lib/services/auth';
import { ConfigService } from '@/lib/services/config';
import type { RegisterRequest } from '@/lib/services/auth/types';
import { safeRedirectTarget } from '@/lib/utils';

function getRedirectTarget(searchParams: ReturnType<typeof useSearchParams>) {
  const callbackUrl = searchParams.get('callbackUrl');
  const storedRedirect =
    typeof window === 'undefined'
      ? null
      : sessionStorage.getItem('redirect_after_login');
  return safeRedirectTarget(callbackUrl || storedRedirect || '/');
}

function configBool(value: string | undefined, fallback: boolean) {
  if (value === undefined) return fallback;
  return value === 'true';
}

export function RegisterForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { setUser } = useAuth();
  const t = useTranslations('auth.register');
  const tCommon = useTranslations('common');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [nickname, setNickname] = useState('');
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const [registerCooldown, setRegisterCooldown] = useState(0);
  const [errorMessage, setErrorMessage] = useState('');

  useEffect(() => {
    if (registerCooldown > 0) {
      const timer = setTimeout(
        () => setRegisterCooldown(registerCooldown - 1),
        1000,
      );
      return () => clearTimeout(timer);
    }
  }, [registerCooldown]);

  const publicConfigQuery = useQuery({
    queryKey: ['public-config'],
    queryFn: () => ConfigService.getPublicConfig(),
  });

  const redirectTarget = useMemo(
    () => getRedirectTarget(searchParams),
    [searchParams],
  );

  const registrationEnabled =
    configBool(publicConfigQuery.data?.registration_enabled, false) &&
    configBool(publicConfigQuery.data?.password_register_enabled, false);

  const emailRegisterEnabled = configBool(
    publicConfigQuery.data?.email_register_verification_enabled,
    false,
  );

  const capEnabled = configBool(
    publicConfigQuery.data?.cap_login_enabled,
    false,
  );
  const capAutoSolve = configBool(publicConfigQuery.data?.cap_auto_solve, true);

  const [capAfterEmailCode, setCapAfterEmailCode] = useState(false);
  const capScope: 'send_email_code' | 'register' =
    emailRegisterEnabled && !capAfterEmailCode ? 'send_email_code' : 'register';

  const capTokenRef = useRef<string | null>(null);
  const [capReady, setCapReady] = useState(false);
  const [capError, setCapError] = useState(false);
  const [capResetKey, setCapResetKey] = useState(0);

  const handleCapToken = (token: string) => {
    capTokenRef.current = token;
    setCapReady(true);
    setCapError(false);
  };

  const handleCapError = () => {
    capTokenRef.current = null;
    setCapReady(false);
    setCapError(true);
  };

  // Redirect to login if registration is closed
  useEffect(() => {
    if (publicConfigQuery.isSuccess && !registrationEnabled) {
      toast.error(t('closed'));
      router.replace('/login');
    }
  }, [publicConfigQuery.isSuccess, registrationEnabled, router, t]);

  const registerMutation = useMutation({
    mutationFn: (req: RegisterRequest) => {
      const headers: Record<string, string> = {};
      if (capEnabled && capTokenRef.current) {
        headers['X-Cap-Token'] = capTokenRef.current;
        capTokenRef.current = null;
        setCapReady(false);
      }
      return AuthService.register(
        req,
        Object.keys(headers).length ? headers : undefined,
      );
    },
    onSuccess: (user) => {
      setUser(user);
      if (typeof window !== 'undefined') {
        sessionStorage.removeItem('redirect_after_login');
      }
      router.replace(redirectTarget);
      toast.success(t('success'));
    },
    onError: (error: Error) => {
      setErrorMessage(error.message || t('failed'));
      if (capEnabled) {
        capTokenRef.current = null;
        setCapReady(false);
        setCapResetKey((key) => key + 1);
      }
    },
  });

  const sendRegisterCodeMutation = useMutation({
    mutationFn: (targetEmail: string) => {
      const headers: Record<string, string> = {};
      if (capEnabled && capTokenRef.current) {
        headers['X-Cap-Token'] = capTokenRef.current;
        capTokenRef.current = null;
        setCapReady(false);
      }
      return AuthService.sendEmailCode(
        targetEmail,
        'register',
        Object.keys(headers).length ? headers : undefined,
      );
    },
    onSuccess: () => {
      setRegisterCooldown(60);
      toast.success(t('codeSent'));
      if (capEnabled) {
        setCapAfterEmailCode(true);
        capTokenRef.current = null;
        setCapReady(false);
        setCapResetKey((key) => key + 1);
      }
    },
    onError: (error: Error) => {
      toast.error(error.message || t('sendCodeFailed'));
      if (capEnabled) {
        capTokenRef.current = null;
        setCapReady(false);
        setCapResetKey((key) => key + 1);
      }
    },
  });

  const registerDisabled =
    registerMutation.isPending ||
    (capEnabled &&
      capScope === 'register' &&
      capAutoSolve &&
      !capReady &&
      !capError);

  const sendCodeDisabled =
    registerCooldown > 0 ||
    sendRegisterCodeMutation.isPending ||
    (capEnabled &&
      capScope === 'send_email_code' &&
      capAutoSolve &&
      !capReady &&
      !capError);

  const handleSendRegisterCode = () => {
    const trimmedEmail = email.trim();
    if (!trimmedEmail) {
      toast.error(t('emailRequired'));
      return;
    }
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    if (!emailRegex.test(trimmedEmail)) {
      toast.error(t('emailInvalid'));
      return;
    }
    if (capEnabled && capScope === 'send_email_code' && !capReady) {
      toast.error(capAutoSolve ? t('capPending') : t('capRequired'));
      return;
    }
    sendRegisterCodeMutation.mutate(trimmedEmail);
  };

  const handleRegister = () => {
    setErrorMessage('');
    if (!username.trim() || !password) {
      toast.error(t('usernamePasswordRequired'));
      return;
    }
    if (password.length < 8) {
      toast.error(t('passwordTooShort'));
      return;
    }
    const trimmedEmail = email.trim();
    if (!trimmedEmail) {
      toast.error(t('emailEmpty'));
      return;
    }
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    if (!emailRegex.test(trimmedEmail)) {
      toast.error(t('emailInvalid'));
      return;
    }
    if (emailRegisterEnabled && !code.trim()) {
      toast.error(t('codeRequired'));
      return;
    }
    if (capEnabled && capScope === 'register' && !capReady) {
      toast.error(capAutoSolve ? t('capPending') : t('capRequired'));
      return;
    }
    registerMutation.mutate({
      username: username.trim(),
      password,
      nickname: nickname.trim() || undefined,
      email: trimmedEmail,
      code: code.trim() || undefined,
    });
  };

  if (publicConfigQuery.isPending) {
    return (
      <div className='flex items-center justify-center py-24'>
        <Spinner />
      </div>
    );
  }

  if (!registrationEnabled) {
    return null;
  }

  return (
    <div className='flex flex-col gap-6 [@media(max-height:700px)]:gap-4'>
      <AuthHeading
        siteName={publicConfigQuery.data?.site_name}
        title={t('title')}
        description={t('description')}
      />

      <div className='flex flex-col gap-5 [@media(max-height:700px)]:gap-3'>
        <FieldGroup className='gap-4 [@media(min-width:500px)]:grid [@media(min-width:500px)]:grid-cols-2 [@media(max-height:700px)]:gap-3'>
          <Field className='gap-1.5'>
            <FieldLabel htmlFor='username'>
              {t('username')} <span className='text-destructive'>*</span>
            </FieldLabel>
            <Input
              id='username'
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder={t('usernamePlaceholder')}
              autoComplete='username'
              className='h-10 text-sm [@media(max-height:700px)]:h-9'
              onKeyDown={(e) => e.key === 'Enter' && handleRegister()}
            />
          </Field>
          <Field className='gap-1.5'>
            <FieldLabel htmlFor='nickname'>
              {t('nickname')}
              <span className='ml-1 text-xs font-normal text-muted-foreground'>
                {tCommon('optional')}
              </span>
            </FieldLabel>
            <Input
              id='nickname'
              value={nickname}
              onChange={(e) => setNickname(e.target.value)}
              placeholder={t('nicknamePlaceholder')}
              autoComplete='nickname'
              className='h-10 text-sm [@media(max-height:700px)]:h-9'
              onKeyDown={(e) => e.key === 'Enter' && handleRegister()}
            />
          </Field>
          <Field className='gap-1.5'>
            <FieldLabel htmlFor='email'>
              {t('email')} <span className='text-destructive'>*</span>
            </FieldLabel>
            <Input
              id='email'
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder={t('emailPlaceholder')}
              autoComplete='email'
              className='h-10 text-sm [@media(max-height:700px)]:h-9'
              onKeyDown={(e) => e.key === 'Enter' && handleRegister()}
            />
          </Field>
          <Field className='gap-1.5'>
            <FieldLabel htmlFor='password'>
              {t('password')} <span className='text-destructive'>*</span>
            </FieldLabel>
            <Input
              id='password'
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              type='password'
              placeholder={t('passwordPlaceholder')}
              autoComplete='new-password'
              className='h-10 text-sm [@media(max-height:700px)]:h-9'
              onKeyDown={(e) => e.key === 'Enter' && handleRegister()}
            />
          </Field>
          {emailRegisterEnabled && (
            <Field className='gap-1.5 [@media(min-width:500px)]:col-span-2'>
              <FieldLabel htmlFor='code'>
                {t('emailCode')} <span className='text-destructive'>*</span>
              </FieldLabel>
              <div className='flex gap-2'>
                <Input
                  id='code'
                  value={code}
                  onChange={(e) => setCode(e.target.value)}
                  placeholder={t('emailCodePlaceholder')}
                  maxLength={6}
                  className='h-10 flex-1 text-sm [@media(max-height:700px)]:h-9'
                  onKeyDown={(e) => e.key === 'Enter' && handleRegister()}
                />
                <Button
                  type='button'
                  variant='outline'
                  onClick={handleSendRegisterCode}
                  disabled={sendCodeDisabled}
                  className='h-10 w-[120px] text-xs [@media(max-height:700px)]:h-9'
                >
                  {registerCooldown > 0
                    ? t('resendIn', { seconds: registerCooldown })
                    : t('getCode')}
                </Button>
              </div>
            </Field>
          )}
        </FieldGroup>

        {capEnabled && (
          <CapWidget
            key={`${capResetKey}-${capScope}`}
            scope={capScope}
            onToken={handleCapToken}
            onError={handleCapError}
            autoStart={capAutoSolve}
          />
        )}

        {errorMessage ? (
          <div className='rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive'>
            {errorMessage}
          </div>
        ) : null}

        <Button
          type='button'
          className='h-10 w-full [@media(max-height:700px)]:h-9'
          variant='auth'
          onClick={handleRegister}
          disabled={registerDisabled}
        >
          {registerMutation.isPending ? (
            <>
              <Spinner />
              {t('submitting')}
            </>
          ) : (
            t('submit')
          )}
        </Button>
      </div>

      <div className='text-center text-sm text-muted-foreground'>
        {t('hasAccount')}{' '}
        <Link
          href='/login'
          className='font-medium text-foreground underline underline-offset-4'
        >
          {t('backToLogin')}
        </Link>
      </div>
    </div>
  );
}
