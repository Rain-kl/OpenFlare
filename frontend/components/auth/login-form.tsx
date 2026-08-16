'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useSearchParams } from 'next/navigation';
import { EyeIcon, EyeOffIcon } from 'lucide-react';
import { toast } from 'sonner';
import Link from 'next/link';
import { useTranslations } from 'next-intl';

import { useAuth } from '@/components/providers/auth-provider';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Separator } from '@/components/ui/separator';
import { Spinner } from '@/components/ui/spinner';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { CapWidget } from '@/components/auth/cap-widget';
import { AuthHeading } from '@/components/auth/auth-shell';
import { OTPForm } from './otp-form';
import { AuthService } from '@/lib/services/auth';
import { ConfigService } from '@/lib/services/config';
import type { LoginRequest } from '@/lib/services/auth/types';
import { safeRedirectTarget } from '@/lib/utils';

function persistRedirectTarget(
  searchParams: ReturnType<typeof useSearchParams>,
) {
  const callbackUrl = searchParams.get('callbackUrl');
  if (callbackUrl && typeof window !== 'undefined') {
    sessionStorage.setItem(
      'redirect_after_login',
      safeRedirectTarget(callbackUrl),
    );
  }
}

function configBool(value: string | undefined, fallback: boolean) {
  if (value === undefined) return fallback;
  return value === 'true';
}

export function LoginForm({
  onOTPStateChange,
}: {
  onOTPStateChange?: (show: boolean) => void;
}) {
  const searchParams = useSearchParams();
  const { setUser } = useAuth();
  const t = useTranslations('auth.login');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [code, setCode] = useState('');
  const [showLoginCodeInput, setShowLoginCodeInput] = useState(false);
  const [loginCooldown, setLoginCooldown] = useState(0);
  const [errorMessage, setErrorMessage] = useState('');
  const [loginCodeTip, setLoginCodeTip] = useState<ReactNode>(null);

  useEffect(() => {
    onOTPStateChange?.(showLoginCodeInput);
  }, [showLoginCodeInput, onOTPStateChange]);

  useEffect(() => {
    if (loginCooldown > 0) {
      const timer = setTimeout(() => setLoginCooldown(loginCooldown - 1), 1000);
      return () => clearTimeout(timer);
    }
  }, [loginCooldown]);

  // Cap token management — ref to hold latest token without triggering re-render
  const capTokenRef = useRef<string | null>(null);
  const [capReady, setCapReady] = useState(false);
  const [capError, setCapError] = useState(false);
  const [capResetKey, setCapResetKey] = useState(0);

  const publicConfigQuery = useQuery({
    queryKey: ['public-config'],
    queryFn: () => ConfigService.getPublicConfig(),
  });

  const authSourcesQuery = useQuery({
    queryKey: ['auth-sources'],
    queryFn: () => AuthService.getAuthSources(),
  });

  const capEnabled = configBool(
    publicConfigQuery.data?.cap_login_enabled,
    false,
  );
  const capAutoSolve = configBool(publicConfigQuery.data?.cap_auto_solve, true);

  const loginMutation = useMutation({
    mutationFn: (req: LoginRequest) => {
      const headers: Record<string, string> = {};
      if (capEnabled && capTokenRef.current) {
        headers['X-Cap-Token'] = capTokenRef.current;
        // Consume the token — next login attempt will need a new one
        capTokenRef.current = null;
        setCapReady(false);
      }
      return AuthService.login(
        req,
        Object.keys(headers).length ? headers : undefined,
      );
    },
    onSuccess: (user) => {
      setUser(user);
      toast.success(t('success'));
    },
    onError: (error: Error) => {
      const errorMsg = error.message || '';
      if (errorMsg.startsWith('need_email_code:')) {
        const emailMasked = errorMsg.substring('need_email_code:'.length);
        setLoginCodeTip(t('codeTipMasked', { email: emailMasked }));
        setShowLoginCodeInput(true);
        setLoginCooldown(60);
        toast.success(t('codeSent'));
        if (capEnabled) {
          capTokenRef.current = null;
          setCapReady(false);
          setCapResetKey((key) => key + 1);
        }
        return;
      }

      if (errorMsg.startsWith('smtp_invalid:')) {
        const tip = errorMsg.substring('smtp_invalid:'.length);
        setLoginCodeTip(t('codeTipGeneric'));
        setShowLoginCodeInput(true);
        setLoginCooldown(0);
        toast.warning(tip);
        if (capEnabled) {
          capTokenRef.current = null;
          setCapReady(false);
          setCapResetKey((key) => key + 1);
        }
        return;
      }

      setErrorMessage(errorMsg || t('failed'));
      if (capEnabled) {
        capTokenRef.current = null;
        setCapReady(false);
        setCapResetKey((key) => key + 1);
      }
    },
  });

  const handlePasswordLogin = () => {
    setErrorMessage('');
    const trimmedUsername = username.trim();
    if (!trimmedUsername || !password) {
      toast.error(t('incomplete'), {
        description: t('incompleteDesc'),
      });
      return;
    }
    if (capEnabled && !capReady) {
      toast.error(capAutoSolve ? t('capPending') : t('capRequired'));
      return;
    }
    loginMutation.mutate({
      username: trimmedUsername,
      password,
      code: showLoginCodeInput ? code.trim() : undefined,
    });
  };

  const handleResendLoginCode = () => {
    setCode('');
    loginMutation.mutate({
      username: username.trim(),
      password,
    });
  };

  const handleOAuthLogin = async (sourceName: string) => {
    try {
      setErrorMessage('');
      persistRedirectTarget(searchParams);
      const { authorize_url } = await AuthService.getAuthorizeUrl(sourceName);
      window.location.href = authorize_url;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('oauthFailed'));
    }
  };

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

  const registrationEnabled =
    configBool(publicConfigQuery.data?.registration_enabled, false) &&
    configBool(publicConfigQuery.data?.password_register_enabled, false);

  const passwordLoginEnabled = configBool(
    publicConfigQuery.data?.password_login_enabled,
    true,
  );
  const oidcLoginEnabled = configBool(
    publicConfigQuery.data?.oidc_login_enabled,
    true,
  );
  const authSources = oidcLoginEnabled ? (authSourcesQuery.data ?? []) : [];

  const loginDisabled =
    !passwordLoginEnabled ||
    loginMutation.isPending ||
    (capEnabled && capAutoSolve && !capReady && !capError);

  if (publicConfigQuery.isPending) {
    return (
      <div className='flex items-center justify-center py-24'>
        <Spinner />
      </div>
    );
  }

  if (showLoginCodeInput) {
    return (
      <OTPForm
        code={code}
        setCode={setCode}
        loginCodeTip={loginCodeTip}
        loginCooldown={loginCooldown}
        isPending={loginMutation.isPending}
        onResend={handleResendLoginCode}
        onSubmit={handlePasswordLogin}
      />
    );
  }

  return (
    <div className='flex flex-col gap-6 [@media(max-height:700px)]:gap-4'>
      <AuthHeading
        siteName={publicConfigQuery.data?.site_name}
        title={t('title')}
        description={t('description')}
      />

      <div className='flex flex-col gap-5 [@media(max-height:700px)]:gap-3'>
        <FieldGroup className='gap-4 [@media(max-height:700px)]:gap-3'>
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
              onKeyDown={(e) => e.key === 'Enter' && handlePasswordLogin()}
            />
          </Field>
          <Field className='gap-1.5'>
            <FieldLabel htmlFor='password'>
              {t('password')} <span className='text-destructive'>*</span>
            </FieldLabel>
            <div className='relative'>
              <Input
                id='password'
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                type={showPassword ? 'text' : 'password'}
                placeholder={t('passwordPlaceholder')}
                autoComplete='current-password'
                className='h-10 pr-11 text-sm [@media(max-height:700px)]:h-9'
                onKeyDown={(e) => e.key === 'Enter' && handlePasswordLogin()}
              />
              <Button
                type='button'
                variant='ghost'
                size='icon-sm'
                aria-label={
                  showPassword ? t('hidePassword') : t('showPassword')
                }
                className='absolute right-1.5 top-1/2 -translate-y-1/2 text-muted-foreground'
                onClick={() => setShowPassword((visible) => !visible)}
              >
                {showPassword ? <EyeOffIcon /> : <EyeIcon />}
              </Button>
            </div>
          </Field>
        </FieldGroup>

        {/* Cap 人机验证 */}
        {capEnabled && (
          <CapWidget
            key={capResetKey}
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
          onClick={handlePasswordLogin}
          disabled={loginDisabled}
        >
          {loginMutation.isPending ? (
            <>
              <Spinner />
              {t('submitting')}
            </>
          ) : (
            t('submit')
          )}
        </Button>
      </div>

      {authSources.length > 0 ? (
        <div className='flex flex-col gap-3'>
          <div className='flex items-center gap-3'>
            <Separator className='flex-1' />
            <span className='text-xs text-muted-foreground'>{t('orUse')}</span>
            <Separator className='flex-1' />
          </div>
          <div className='grid gap-2'>
            {authSources.map((source) => (
              <Button
                key={source.id}
                type='button'
                variant='outline'
                className='h-10 [@media(max-height:700px)]:h-9'
                onClick={() => void handleOAuthLogin(source.name)}
              >
                {t('oauthLogin', { name: source.display_name || source.name })}
              </Button>
            ))}
          </div>
        </div>
      ) : null}

      {registrationEnabled && (
        <div className='text-center text-sm text-muted-foreground'>
          {t('noAccount')}{' '}
          <Link
            href='/register'
            className='font-medium text-foreground underline underline-offset-4'
          >
            {t('registerNow')}
          </Link>
        </div>
      )}
    </div>
  );
}
