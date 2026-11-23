'use client';

import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft } from 'lucide-react';
import { useTranslations } from 'next-intl';
import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { Suspense } from 'react';

import { ErrorInline } from '@/components/layout/error';
import { LoadingState } from '@/components/layout/loading';
import { Button } from '@/components/ui/button';
import {
  attackGroupsQueryKey,
  WafService,
} from '@/lib/services/openflare/waf.service';

import { AttackGroupEditor } from './attack-group-editor';

export default function AttackProtectionEditorPage() {
  const t = useTranslations('attackProtection');

  return (
    <div className='flex w-full flex-col gap-4 px-1 py-6'>
      <Button variant='ghost' size='sm' className='w-fit' asChild>
        <Link href='/attack-protection'>
          <ArrowLeft data-icon='inline-start' />
          {t('back')}
        </Link>
      </Button>
      <Suspense fallback={<LoadingState />}>
        <Editor />
      </Suspense>
    </div>
  );
}

function Editor() {
  const t = useTranslations('attackProtection');
  const router = useRouter();
  const client = useQueryClient();
  const id = useSearchParams().get('id');
  const groups = useQuery({
    queryKey: attackGroupsQueryKey,
    queryFn: () => WafService.listAttackGroups(),
    enabled: id !== null,
  });
  const group = groups.data?.find((item) => String(item.id) === id);

  if (id !== null && groups.isPending) return <LoadingState />;
  if (id !== null && groups.isLoadingError) {
    return (
      <ErrorInline
        message={t('loadFailed')}
        onRetry={() => void groups.refetch()}
      />
    );
  }
  if (id !== null && !group)
    return <ErrorInline message={t('groupNotFound')} />;

  return (
    <AttackGroupEditor
      key={id ?? 'new'}
      group={group}
      onSaved={async () => {
        await client.invalidateQueries({ queryKey: attackGroupsQueryKey });
        router.replace('/attack-protection');
      }}
    />
  );
}
