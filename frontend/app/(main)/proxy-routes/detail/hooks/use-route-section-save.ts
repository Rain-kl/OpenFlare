'use client';

import { useCallback, useState } from 'react';
import { toast } from 'sonner';

import { useTranslations } from 'next-intl';

import type {
  ProxyRouteItem,
  ProxyRouteMutationPayload,
} from '@/lib/services/openflare';
import { ProxyRouteService } from '@/lib/services/openflare';

import {
  buildPayloadFromRoute,
  getErrorMessage,
} from '../../components/helpers';

export function useRouteSectionSave(
  route: ProxyRouteItem,
  onRouteUpdate: (route: ProxyRouteItem) => void,
  onSavingChange?: (saving: boolean) => void,
) {
  const t = useTranslations('proxyRoutes');
  const [saving, setSaving] = useState(false);

  const save = useCallback(
    async (overrides: Partial<ProxyRouteMutationPayload>, message: string) => {
      setSaving(true);
      onSavingChange?.(true);
      try {
        const updated = await ProxyRouteService.update(
          route.id,
          buildPayloadFromRoute(route, overrides),
        );
        onRouteUpdate(updated);
        toast.success(message);
      } catch (error) {
        toast.error(t('saveFailed'), {
          description: getErrorMessage(error, t('requestFailed')),
        });
      } finally {
        setSaving(false);
        onSavingChange?.(false);
      }
    },
    [onRouteUpdate, onSavingChange, route, t],
  );

  return { saving, save };
}
