'use client';

import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { useAdminUsers } from '@/contexts/admin-users-context';
import type { CreateUserRequest } from '@/lib/services/admin';

const emptyForm: CreateUserRequest = {
  username: '',
  password: '',
  nickname: '',
  email: '',
  is_active: true,
  is_admin: false,
};

export function CreateUserModal({
  isOpen,
  onClose,
}: {
  isOpen: boolean;
  onClose: () => void;
}) {
  const t = useTranslations('admin.users');
  const { createUser } = useAdminUsers();
  const [form, setForm] = useState<CreateUserRequest>(emptyForm);
  const [saving, setSaving] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    if (isOpen) {
      setForm(emptyForm);
      setErrors({});
    } else {
      setSaving(false);
    }
  }, [isOpen]);

  const validate = () => {
    const newErrors: Record<string, string> = {};
    if (!form.username.trim()) {
      newErrors.username = t('validation.usernameRequired');
    } else if (form.username.trim().length < 3) {
      newErrors.username = t('validation.usernameTooShort');
    }

    if (!form.email.trim()) {
      newErrors.email = t('validation.emailRequired');
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email.trim())) {
      newErrors.email = t('validation.emailInvalid');
    }

    if (!form.password) {
      newErrors.password = t('validation.passwordRequired');
    } else if (form.password.length < 8) {
      newErrors.password = t('validation.passwordTooShort');
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!validate()) return;

    setSaving(true);
    try {
      await createUser({
        ...form,
        username: form.username.trim(),
        nickname: form.nickname?.trim() || undefined,
        email: form.email.trim(),
      });
      onClose();
    } catch {
      // Errors are already handled by context toast notifications
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className='max-w-md'>
        <DialogHeader>
          <DialogTitle>{t('createTitle')}</DialogTitle>
          <DialogDescription>{t('createDesc')}</DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSave} className='space-y-4 pt-2'>
          <div className='space-y-1.5'>
            <Label htmlFor='username'>{t('username')}</Label>
            <Input
              id='username'
              value={form.username}
              onChange={(e) =>
                setForm((prev) => ({ ...prev, username: e.target.value }))
              }
              placeholder={t('usernamePlaceholder')}
            />
            {errors.username && (
              <p className='text-xs text-destructive'>{errors.username}</p>
            )}
          </div>

          <div className='space-y-1.5'>
            <Label htmlFor='nickname'>{t('nicknameOptional')}</Label>
            <Input
              id='nickname'
              value={form.nickname}
              onChange={(e) =>
                setForm((prev) => ({ ...prev, nickname: e.target.value }))
              }
              placeholder={t('nicknamePlaceholder')}
            />
          </div>

          <div className='space-y-1.5'>
            <Label htmlFor='email'>{t('email')}</Label>
            <Input
              id='email'
              type='email'
              value={form.email}
              onChange={(e) =>
                setForm((prev) => ({ ...prev, email: e.target.value }))
              }
              placeholder={t('emailPlaceholder')}
            />
            {errors.email && (
              <p className='text-xs text-destructive'>{errors.email}</p>
            )}
          </div>

          <div className='space-y-1.5'>
            <Label htmlFor='password'>{t('password')}</Label>
            <Input
              id='password'
              type='password'
              value={form.password}
              onChange={(e) =>
                setForm((prev) => ({ ...prev, password: e.target.value }))
              }
              placeholder={t('passwordPlaceholder')}
            />
            {errors.password && (
              <p className='text-xs text-destructive'>{errors.password}</p>
            )}
          </div>

          <div className='flex items-center justify-between rounded-lg border border-dashed p-3 bg-muted/10'>
            <div>
              <div className='font-medium text-sm'>
                {t('enableAccountLabel')}
              </div>
              <div className='text-xs text-muted-foreground'>
                {t('enableAccountDesc')}
              </div>
            </div>
            <Switch
              checked={form.is_active}
              onCheckedChange={(checked) =>
                setForm((prev) => ({ ...prev, is_active: checked }))
              }
            />
          </div>

          <div className='flex items-center justify-between rounded-lg border border-dashed p-3 bg-muted/10'>
            <div>
              <div className='font-medium text-sm'>{t('adminPermission')}</div>
              <div className='text-xs text-muted-foreground'>
                {t('adminPermissionDesc')}
              </div>
            </div>
            <Switch
              checked={form.is_admin}
              onCheckedChange={(checked) =>
                setForm((prev) => ({ ...prev, is_admin: checked }))
              }
            />
          </div>

          <div className='flex justify-end gap-2 pt-2 border-t mt-2'>
            <Button variant='outline' type='button' onClick={onClose}>
              {t('cancel')}
            </Button>
            <Button type='submit' disabled={saving} variant='secondary'>
              {saving ? t('creating') : t('create')}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
