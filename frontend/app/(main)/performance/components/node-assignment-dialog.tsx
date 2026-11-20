import { useTranslations } from 'next-intl';
import { useState } from 'react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  PerformanceService,
  type PerformanceNode,
  type PerformancePolicy,
} from '@/lib/services/openflare';

interface Props {
  policy: PerformancePolicy;
  policies: PerformancePolicy[];
  nodes: PerformanceNode[];
  onClose: () => void;
  onSaved: () => void;
}

export function NodeAssignmentDialog({
  policy,
  policies,
  nodes,
  onClose,
  onSaved,
}: Props) {
  const t = useTranslations('performancePolicies');
  const [search, setSearch] = useState('');
  const [selected, setSelected] = useState(
    () =>
      new Set(
        nodes
          .filter(({ policy_id }) => policy_id === policy.id)
          .map(({ node_id }) => node_id),
      ),
  );
  const [saving, setSaving] = useState(false);
  const names = new Map(policies.map(({ id, name }) => [id, name]));
  const keyword = search.trim().toLowerCase();
  const visible = nodes.filter(
    ({ name, node_id }) =>
      name.toLowerCase().includes(keyword) ||
      node_id.toLowerCase().includes(keyword),
  );

  function setChecked(ids: string[], checked: boolean) {
    setSelected((current) => {
      const next = new Set(current);
      for (const id of ids) {
        if (checked) next.add(id);
        else next.delete(id);
      }
      return next;
    });
  }

  async function save() {
    setSaving(true);
    try {
      await PerformanceService.assign(policy.id, Array.from(selected));
      toast.success(t('assigned'));
      onSaved();
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('operationFailed'),
      );
    } finally {
      setSaving(false);
    }
  }

  const visibleSelected = visible.filter(({ node_id }) =>
    selected.has(node_id),
  ).length;

  return (
    <Dialog open onOpenChange={(open) => !open && !saving && onClose()}>
      <DialogContent className='flex max-h-[90dvh] flex-col sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>{t('assignTitle', { name: policy.name })}</DialogTitle>
          <DialogDescription>{t('assignDescription')}</DialogDescription>
        </DialogHeader>
        <Field>
          <FieldLabel htmlFor='performance-node-search' className='sr-only'>
            {t('search')}
          </FieldLabel>
          <Input
            id='performance-node-search'
            placeholder={t('search')}
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
        </Field>
        {visible.length > 0 && (
          <Field orientation='horizontal'>
            <Checkbox
              id='performance-select-all'
              checked={
                visibleSelected > 0 && visibleSelected < visible.length
                  ? 'indeterminate'
                  : visibleSelected === visible.length
              }
              onCheckedChange={(checked) =>
                setChecked(
                  visible.map(({ node_id }) => node_id),
                  checked === true,
                )
              }
            />
            <FieldLabel htmlFor='performance-select-all'>
              {t('selectAll')}
            </FieldLabel>
          </Field>
        )}
        <FieldGroup className='min-h-0 gap-2 overflow-y-auto p-1'>
          {visible.length === 0 && (
            <p className='text-sm text-muted-foreground'>{t('noNodes')}</p>
          )}
          {visible.map((node, index) => (
            <Field
              key={node.node_id}
              orientation='horizontal'
              className='rounded-md border border-dashed p-3'
            >
              <Checkbox
                id={`performance-node-${index}`}
                checked={selected.has(node.node_id)}
                onCheckedChange={(checked) =>
                  setChecked([node.node_id], checked === true)
                }
              />
              <div className='min-w-0'>
                <FieldLabel
                  htmlFor={`performance-node-${index}`}
                  className='break-all'
                >
                  {node.name}
                </FieldLabel>
                <p className='break-all text-xs text-muted-foreground'>
                  {node.node_id} ·{' '}
                  {node.policy_id === null
                    ? t('defaultPolicy')
                    : names.get(node.policy_id)}
                </p>
              </div>
            </Field>
          ))}
        </FieldGroup>
        <DialogFooter>
          <Button variant='outline' disabled={saving} onClick={onClose}>
            {t('cancel')}
          </Button>
          <Button disabled={saving} onClick={() => void save()}>
            {t('applyCount', { count: selected.size })}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
