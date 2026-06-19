'use client';

import {useCallback, useState} from 'react';
import {toast} from 'sonner';

import type {ProxyRouteItem, ProxyRouteMutationPayload} from '@/lib/services/openflare';
import {ProxyRouteService} from '@/lib/services/openflare';

import {buildPayloadFromRoute, getErrorMessage} from '../../components/helpers';

export function useRouteSectionSave(
  route: ProxyRouteItem,
  onRouteUpdate: (route: ProxyRouteItem) => void,
  onSavingChange?: (saving: boolean) => void,
) {
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
        toast.error('保存失败', { description: getErrorMessage(error) });
      } finally {
        setSaving(false);
        onSavingChange?.(false);
      }
    },
    [onRouteUpdate, onSavingChange, route],
  );

  return { saving, save };
}