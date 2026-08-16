'use client';

import { useEffect, useState } from 'react';
import { toast } from 'sonner';
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
import services from '@/lib/services';
import type { AuthSource, AuthSourceRequest } from '@/lib/services/admin';

const emptyForm: AuthSourceRequest = {
  name: '',
  type: 'oidc',
  display_name: '',
  is_active: false,
  client_id: '',
  client_secret: '',
  openid_discovery_url: '',
  scopes: 'openid profile email',
  icon_url: '',
};

export function AuthSourceModal({
  isOpen,
  source,
  onClose,
  onChanged,
}: {
  isOpen: boolean;
  source: AuthSource | null;
  onClose: () => void;
  onChanged: () => Promise<void>;
}) {
  const t = useTranslations('settings.authSourceModal');
  const [form, setForm] = useState<AuthSourceRequest>(emptyForm);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (isOpen) {
      if (source) {
        setForm({
          name: source.name,
          type: source.type,
          display_name: source.display_name,
          is_active: source.is_active,
          client_id: source.client_id,
          client_secret: '',
          openid_discovery_url: source.openid_discovery_url,
          scopes: source.scopes || 'openid profile email',
          icon_url: source.icon_url,
        });
      } else {
        setForm(emptyForm);
      }
    } else {
      setForm(emptyForm);
      setSaving(false);
    }
  }, [isOpen, source]);

  const saveSource = async () => {
    setSaving(true);
    try {
      if (source) {
        await services.adminAuthSource.updateAuthSource(source.id, form);
      } else {
        await services.adminAuthSource.createAuthSource(form);
      }
      await onChanged();
      toast.success(t('authSourceSaved'));
      onClose();
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('saveAuthSourceFailed'),
      );
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className='max-w-xl'>
        <DialogHeader>
          <DialogTitle>
            {source ? t('editAuthSource') : t('addAuthSource')}
          </DialogTitle>
          <DialogDescription>
            {source ? t('editAuthSourceDesc') : t('addAuthSourceDesc')}
          </DialogDescription>
        </DialogHeader>

        <div className='grid gap-4 md:grid-cols-2 pt-2'>
          <div className='space-y-2'>
            <Label>{t('identifier')}</Label>
            <Input
              value={form.name}
              disabled={!!source}
              onChange={(e) =>
                setForm((prev) => ({ ...prev, name: e.target.value }))
              }
              placeholder='例如: github'
            />
          </div>
          <div className='space-y-2'>
            <Label>{t('displayName')}</Label>
            <Input
              value={form.display_name}
              onChange={(e) =>
                setForm((prev) => ({ ...prev, display_name: e.target.value }))
              }
              placeholder='例如: GitHub 登录'
            />
          </div>
          <div className='space-y-2'>
            <Label>Client ID</Label>
            <Input
              value={form.client_id}
              onChange={(e) =>
                setForm((prev) => ({ ...prev, client_id: e.target.value }))
              }
            />
          </div>
          <div className='space-y-2'>
            <Label>Client Secret</Label>
            <Input
              value={form.client_secret}
              onChange={(e) =>
                setForm((prev) => ({ ...prev, client_secret: e.target.value }))
              }
              placeholder={source ? t('keepOriginalIfEmpty') : ''}
            />
          </div>
          <div className='space-y-2 md:col-span-2'>
            <Label>{t('discoveryURL')}</Label>
            <Input
              value={form.openid_discovery_url}
              onChange={(e) =>
                setForm((prev) => ({
                  ...prev,
                  openid_discovery_url: e.target.value,
                }))
              }
              placeholder='https://...'
            />
          </div>
          <div className='space-y-2'>
            <Label>Scopes</Label>
            <Input
              value={form.scopes}
              onChange={(e) =>
                setForm((prev) => ({ ...prev, scopes: e.target.value }))
              }
            />
          </div>
          <div className='space-y-2'>
            <Label>{t('iconURL')}</Label>
            <Input
              value={form.icon_url}
              onChange={(e) =>
                setForm((prev) => ({ ...prev, icon_url: e.target.value }))
              }
              placeholder='https://... 或留空'
            />
          </div>
          <div className='flex items-center justify-between rounded-xl border border-dashed p-3 md:col-span-2 bg-muted/10'>
            <div>
              <div className='font-medium text-sm'>{t('enableAuthSource')}</div>
              <div className='text-xs text-muted-foreground'>
                {t('enableAuthSourceDesc')}
              </div>
            </div>
            <Switch
              aria-label={t('enableAuthSource')}
              checked={form.is_active}
              onCheckedChange={(checked) =>
                setForm((prev) => ({ ...prev, is_active: checked }))
              }
            />
          </div>
          <div className='md:col-span-2 flex justify-end gap-2 pt-2 border-t mt-2'>
            <Button variant='outline' type='button' onClick={onClose}>
              {t('cancel')}
            </Button>
            <Button
              type='button'
              onClick={saveSource}
              disabled={saving}
              variant='secondary'
            >
              {saving ? t('saving') : t('save')}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
