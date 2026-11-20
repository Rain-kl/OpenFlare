import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import { useTranslations } from 'next-intl';

import { Button } from '@/components/ui/button';
import { Field, FieldDescription, FieldLabel } from '@/components/ui/field';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { WAFRuleNode } from '@/lib/services/openflare';
import {
  attackGroupsQueryKey,
  WafService,
} from '@/lib/services/openflare/waf.service';

export function AttackGroupSelect({
  node,
  onChange,
}: {
  node: Extract<WAFRuleNode, { type: 'security_check' }>;
  onChange: (node: WAFRuleNode) => void;
}) {
  const t = useTranslations('attackProtection');
  const query = useQuery({
    queryKey: attackGroupsQueryKey,
    queryFn: () => WafService.listAttackGroups(),
  });
  const id = node.config.attack_group_id;
  const groups = query.data ?? [];
  return (
    <Field>
      <FieldLabel htmlFor={`${node.id}-attack-group`}>{t('group')}</FieldLabel>
      <Select
        value={id ? String(id) : ''}
        onValueChange={(value) =>
          onChange({ ...node, config: { attack_group_id: Number(value) } })
        }
        disabled={groups.length === 0}
      >
        <SelectTrigger id={`${node.id}-attack-group`} className='w-full'>
          <SelectValue placeholder={t('selectGroup')} />
        </SelectTrigger>
        <SelectContent>
          {groups.map((group) => (
            <SelectItem key={group.id} value={String(group.id)}>
              {group.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {query.isError && (
        <FieldDescription role='alert'>{t('loadFailed')}</FieldDescription>
      )}
      <FieldDescription>
        {id === undefined ? t('legacy') : t('branches')}
      </FieldDescription>
      <Button variant='link' className='h-auto self-start p-0' asChild>
        <Link href='/attack-protection'>{t('manage')}</Link>
      </Button>
    </Field>
  );
}
