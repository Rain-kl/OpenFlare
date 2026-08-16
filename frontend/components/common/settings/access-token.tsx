'use client';

import * as React from 'react';
import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { motion } from 'motion/react';
import {
  AlertTriangle,
  Check,
  Copy,
  Info,
  Key,
  Loader2,
  Plus,
  RefreshCw,
  Shield,
  Trash2,
} from 'lucide-react';

import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb';
import type { CreateTokenResponse } from '@/lib/services/user';
import { UserService } from '@/lib/services/user';
import { useAuth } from '@/components/providers/auth-provider';
import { toast } from 'sonner';
import { useLocale, useTranslations } from 'next-intl';
import { formatDateTime } from '@/i18n/format';
import type { AppLocale } from '@/i18n/config';

type ConfirmTarget = { id: number; name: string };

export function AccessTokenMain() {
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const t = useTranslations('settings');
  const tCommon = useTranslations('common');
  const ta = useTranslations('settings.accessToken');
  const locale = useLocale() as AppLocale;
  const [createDialogOpen, setCreateDialogOpen] = React.useState(false);
  const [viewDialogOpen, setViewDialogOpen] = React.useState(false);
  const [tokenName, setTokenName] = React.useState('');
  const [tokenIsAdmin, setTokenIsAdmin] = React.useState(false);
  const [copiedId, setCopiedId] = React.useState<number | null>(null);
  const [newCreatedToken, setNewCreatedToken] =
    React.useState<CreateTokenResponse | null>(null);
  const [deleteTarget, setDeleteTarget] = React.useState<ConfirmTarget | null>(
    null,
  );
  const [rotateTarget, setRotateTarget] = React.useState<ConfirmTarget | null>(
    null,
  );

  // 获取 Token 列表
  const accessTokensQuery = useQuery({
    queryKey: ['user', 'access-tokens'],
    queryFn: () => UserService.getAccessTokens(),
  });

  // 创建 Token
  const createTokenMutation = useMutation({
    mutationFn: ({ name, isAdmin }: { name: string; isAdmin: boolean }) =>
      UserService.createAccessToken(name, isAdmin),
    onSuccess: (data) => {
      setNewCreatedToken(data);
      setTokenName('');
      setTokenIsAdmin(false);
      setCreateDialogOpen(false);
      setViewDialogOpen(true);
      void queryClient.invalidateQueries({
        queryKey: ['user', 'access-tokens'],
      });
      toast.success(ta('createSuccess'));
    },
    onError: (error: Error) => {
      toast.error(error.message || ta('createFailed'));
    },
  });

  // 删除 Token
  const deleteTokenMutation = useMutation({
    mutationFn: (id: number) => UserService.deleteAccessToken(id),
    onSuccess: () => {
      setDeleteTarget(null);
      void queryClient.invalidateQueries({
        queryKey: ['user', 'access-tokens'],
      });
      toast.success(ta('deleteSuccess'));
    },
    onError: (error: Error) => {
      toast.error(error.message || ta('deleteFailed'));
    },
  });

  // 轮换 Token
  const rotateTokenMutation = useMutation({
    mutationFn: (id: number) => UserService.rotateAccessToken(id),
    onSuccess: (data) => {
      setRotateTarget(null);
      setNewCreatedToken(data);
      setViewDialogOpen(true);
      void queryClient.invalidateQueries({
        queryKey: ['user', 'access-tokens'],
      });
      toast.success(ta('rotateSuccess'));
    },
    onError: (error: Error) => {
      toast.error(error.message || ta('rotateFailed'));
    },
  });

  const handleCreateToken = (e: React.FormEvent) => {
    e.preventDefault();
    if (!tokenName.trim()) {
      toast.error(ta('nameRequired'));
      return;
    }
    createTokenMutation.mutate({
      name: tokenName.trim(),
      isAdmin: tokenIsAdmin,
    });
  };

  const handleCopyText = async (text: string, id: number) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedId(id);
      toast.success(ta('copySuccess'));
      setTimeout(() => setCopiedId(null), 2000);
    } catch {
      toast.error(ta('copyFailed'));
    }
  };

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return '—';
    return formatDateTime(dateStr, locale);
  };

  return (
    <motion.div
      initial={{ opacity: 0, y: 15 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.35, ease: 'easeOut' }}
      className='py-6 space-y-6 max-w-4xl mx-auto px-4'
    >
      <div className='font-semibold'>
        <Breadcrumb>
          <BreadcrumbList>
            <BreadcrumbItem>
              <BreadcrumbLink asChild>
                <Link href='/settings' className='text-base text-primary'>
                  {t('title')}
                </Link>
              </BreadcrumbLink>
            </BreadcrumbItem>
            <BreadcrumbSeparator />
            <BreadcrumbItem>
              <BreadcrumbPage className='text-base font-semibold'>
                {ta('breadcrumb')}
              </BreadcrumbPage>
            </BreadcrumbItem>
          </BreadcrumbList>
        </Breadcrumb>
      </div>

      <div className='flex flex-col md:flex-row md:items-center md:justify-between gap-4 border-b pb-5'>
        <div className='flex items-center gap-4'>
          <div>
            <h1 className='text-xl font-bold tracking-tight bg-gradient-to-r from-foreground via-foreground/90 to-muted-foreground bg-clip-text text-transparent'>
              {ta('title')}
            </h1>
            <p className='text-sm text-muted-foreground'>{ta('subtitle')}</p>
          </div>
        </div>
        <Button
          type='button'
          onClick={() => setCreateDialogOpen(true)}
          variant={'secondary'}
        >
          <Plus className='mr-1.5 size-4' />
          {ta('generate')}
        </Button>
      </div>

      {/* 安全警告提示 */}
      <div className='rounded-xl border border-amber-500/20 bg-amber-500/5 p-4 flex gap-3 text-amber-600 text-xs leading-relaxed'>
        <AlertTriangle className='size-4 shrink-0 mt-0.5' />
        <div className='space-y-1'>
          <span className='font-bold'>{ta('securityTitle')}</span>
          <p className='text-muted-foreground'>{ta('securityBody')}</p>
        </div>
      </div>

      <Card className='border border-dashed shadow-sm'>
        <CardHeader className='border-b border-dashed pb-4'>
          <CardTitle className='text-base font-semibold'>
            {ta('listTitle')}
          </CardTitle>
          <CardDescription className='text-xs'>
            {ta('listDesc')}
          </CardDescription>
        </CardHeader>
        <CardContent className='pt-6 space-y-4'>
          {accessTokensQuery.isPending ? (
            <div className='flex items-center justify-center py-8'>
              <Loader2 className='size-6 animate-spin text-primary' />
            </div>
          ) : (accessTokensQuery.data ?? []).length > 0 ? (
            <div className='space-y-3'>
              {(accessTokensQuery.data ?? []).map((token) => (
                <div
                  key={token.id}
                  className='flex flex-col sm:flex-row sm:items-center justify-between gap-4 rounded-xl border border-dashed p-4 bg-card hover:bg-muted/10 transition-all duration-300 shadow-sm'
                >
                  <div className='space-y-1'>
                    <div className='flex items-center gap-2'>
                      <span className='font-semibold text-sm text-foreground'>
                        {token.name}
                      </span>
                      {token.is_admin ? (
                        <Badge
                          variant='outline'
                          className='text-[10px] px-1.5 py-0 h-4 border-rose-500/40 text-rose-500 bg-rose-500/5 font-semibold'
                        >
                          <Shield className='size-2.5 mr-0.5' />
                          {ta('adminBadge')}
                        </Badge>
                      ) : (
                        <Badge
                          variant='outline'
                          className='text-[10px] px-1.5 py-0 h-4 border-border/50 text-muted-foreground bg-muted/10 font-semibold'
                        >
                          {ta('userToken')}
                        </Badge>
                      )}
                    </div>
                    <div className='flex flex-col gap-1 text-xs text-muted-foreground'>
                      <div className='font-mono bg-muted/30 px-2 py-0.5 rounded border border-border/50 w-fit select-all'>
                        {token.masked_token}
                      </div>
                      <div className='flex flex-wrap gap-x-4 gap-y-0.5 pt-1'>
                        <span>
                          {ta('createdAt', {
                            date: formatDate(token.created_at),
                          })}
                        </span>
                      </div>
                    </div>
                  </div>
                  <div className='flex items-center gap-2 shrink-0 sm:self-center'>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      className='text-xs border-dashed text-muted-foreground hover:text-primary hover:bg-primary/5 rounded-lg h-8 px-2.5'
                      onClick={() =>
                        setRotateTarget({ id: token.id, name: token.name })
                      }
                      disabled={rotateTokenMutation.isPending}
                    >
                      <RefreshCw
                        className={`size-3.5 mr-1 ${rotateTokenMutation.isPending && rotateTokenMutation.variables === token.id ? 'animate-spin' : ''}`}
                      />
                      {ta('rotate')}
                    </Button>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      className='text-xs border-dashed text-muted-foreground hover:text-rose-500 hover:bg-rose-500/10 hover:border-rose-500/20 rounded-lg h-8 px-2.5'
                      onClick={() =>
                        setDeleteTarget({ id: token.id, name: token.name })
                      }
                      disabled={deleteTokenMutation.isPending}
                    >
                      <Trash2 className='size-3.5 mr-1' />
                      {ta('revoke')}
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className='rounded-xl border border-dashed border-border/50 px-4 py-10 text-center text-xs text-muted-foreground bg-muted/5 flex flex-col items-center justify-center gap-3'>
              <Key className='size-8 text-muted-foreground/30' />
              <span>{ta('empty')}</span>
              <Button
                type='button'
                variant='outline'
                size='sm'
                className='border-dashed'
                onClick={() => setCreateDialogOpen(true)}
              >
                <Plus className='mr-1 size-3.5' />
                {ta('generateFirst')}
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      {/* 创建令牌 Dialog */}
      <Dialog open={createDialogOpen} onOpenChange={setCreateDialogOpen}>
        <DialogContent className='sm:max-w-[425px]'>
          <form onSubmit={handleCreateToken}>
            <DialogHeader>
              <DialogTitle className='text-base font-semibold'>
                {ta('generate')}
              </DialogTitle>
              <DialogDescription className='text-xs text-muted-foreground'>
                {ta('createDesc')}
              </DialogDescription>
            </DialogHeader>
            <div className='space-y-4 py-4'>
              <div className='space-y-2'>
                <Label htmlFor='token-name' className='text-xs font-semibold'>
                  {ta('name')}
                </Label>
                <Input
                  id='token-name'
                  placeholder={ta('namePlaceholder')}
                  value={tokenName}
                  onChange={(e) => setTokenName(e.target.value)}
                  disabled={createTokenMutation.isPending}
                  className='rounded-xl border border-dashed focus:border-primary focus:ring-0 focus-visible:ring-0'
                />
              </div>
              {user?.is_admin && (
                <div className='flex items-center justify-between rounded-xl border border-dashed p-3 bg-muted/5'>
                  <div className='space-y-0.5'>
                    <Label
                      htmlFor='token-admin'
                      className='text-xs font-semibold flex items-center gap-1.5'
                    >
                      <Shield className='size-3.5 text-rose-500' />
                      {ta('adminPermission')}
                    </Label>
                    <p className='text-[11px] text-muted-foreground leading-normal'>
                      {ta('adminHint')}
                    </p>
                  </div>
                  <Switch
                    id='token-admin'
                    checked={tokenIsAdmin}
                    onCheckedChange={setTokenIsAdmin}
                    disabled={createTokenMutation.isPending}
                  />
                </div>
              )}
            </div>
            <DialogFooter className='gap-2'>
              <Button
                type='button'
                variant='outline'
                onClick={() => setCreateDialogOpen(false)}
                disabled={createTokenMutation.isPending}
                className='rounded-xl text-xs h-9 border-dashed'
              >
                {tCommon('cancel')}
              </Button>
              <Button
                type='submit'
                disabled={createTokenMutation.isPending}
                variant={'secondary'}
              >
                {createTokenMutation.isPending ? (
                  <>
                    <Loader2 className='mr-1.5 size-3.5 animate-spin' />
                    {ta('generating')}
                  </>
                ) : (
                  ta('generateAction')
                )}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* 明文 Token 显示 Dialog (仅显示一次) */}
      <Dialog
        open={viewDialogOpen}
        onOpenChange={(open) => {
          if (!open) {
            setNewCreatedToken(null);
            setViewDialogOpen(false);
          }
        }}
      >
        <DialogContent className='sm:max-w-[500px]'>
          <DialogHeader>
            <DialogTitle className='text-base font-bold text-foreground flex items-center gap-1.5'>
              <Check className='size-5 text-emerald-500 border border-emerald-500 rounded-full p-0.5' />
              {ta('readyTitle')}
            </DialogTitle>
          </DialogHeader>

          {newCreatedToken && (
            <div className='space-y-4 py-2'>
              {/* 明文 Token 文本框 */}
              <div className='flex items-center gap-2 rounded-xl bg-primary/5 border border-dashed border-primary/30 p-3'>
                <span className='font-mono text-xs select-all break-all flex-1 text-primary font-semibold leading-relaxed'>
                  {newCreatedToken.token}
                </span>
                <Button
                  type='button'
                  size='icon'
                  variant='outline'
                  className='size-8 rounded-lg shrink-0 border-dashed text-primary hover:bg-primary/10 hover:border-primary/30 transition-colors'
                  onClick={() => handleCopyText(newCreatedToken.token, 9999)}
                >
                  {copiedId === 9999 ? (
                    <Check className='size-3.5 text-emerald-500' />
                  ) : (
                    <Copy className='size-3.5' />
                  )}
                </Button>
              </div>

              {/* 强提示 */}
              <div className='rounded-xl border border-rose-500/20 bg-rose-500/5 p-4 flex gap-3 text-rose-600 text-xs leading-relaxed'>
                <Info className='size-4 shrink-0 mt-0.5' />
                <div className='space-y-1'>
                  <span className='font-bold'>{ta('importantTitle')}</span>
                  <p className='text-muted-foreground'>{ta('readyBody')}</p>
                </div>
              </div>
            </div>
          )}

          <DialogFooter>
            <Button
              type='button'
              onClick={() => {
                setNewCreatedToken(null);
                setViewDialogOpen(false);
              }}
              className='rounded-xl  w-full'
            >
              {ta('savedConfirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{ta('deleteConfirmTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {ta('deleteConfirm', { name: deleteTarget?.name ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteTokenMutation.isPending}>
              {tCommon('cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={deleteTokenMutation.isPending}
              onClick={() =>
                deleteTarget && deleteTokenMutation.mutate(deleteTarget.id)
              }
            >
              {deleteTokenMutation.isPending
                ? ta('revoking')
                : ta('confirmRevoke')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={Boolean(rotateTarget)}
        onOpenChange={(open) => !open && setRotateTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{ta('rotateConfirmTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {ta('rotateConfirm', { name: rotateTarget?.name ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={rotateTokenMutation.isPending}>
              {tCommon('cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={rotateTokenMutation.isPending}
              onClick={() =>
                rotateTarget && rotateTokenMutation.mutate(rotateTarget.id)
              }
            >
              {rotateTokenMutation.isPending
                ? ta('rotating')
                : ta('confirmRotate')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </motion.div>
  );
}
