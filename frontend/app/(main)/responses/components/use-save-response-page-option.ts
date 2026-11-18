'use client';

import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

import { OptionService, type OptionItem } from '@/lib/services/openflare';

import { invalidateResponseQueries, OPTIONS_QUERY_KEY } from './shared';

export function useSaveResponsePageOption(key: string, savedMessage: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (update: string | ((latest: string) => string)) => {
      // For options that combine HTML and settings, merge into the latest value.
      const value =
        typeof update === 'string'
          ? update
          : update(
              (await OptionService.list()).find((item) => item.key === key)
                ?.value ?? '',
            );
      await OptionService.updateBatch([{ key, value }]);
      return value;
    },
    onSuccess: async (value) => {
      queryClient.setQueryData<OptionItem[]>(OPTIONS_QUERY_KEY, (options) => [
        ...(options ?? []).filter((item) => item.key !== key),
        { key, value },
      ]);
      toast.success(savedMessage);
      await invalidateResponseQueries(queryClient);
    },
    onError: (error) => toast.error(error.message),
  });
}
