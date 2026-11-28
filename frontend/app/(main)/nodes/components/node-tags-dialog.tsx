'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Pencil, Plus, Tags, Trash2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useState } from 'react';
import { toast } from 'sonner';

import {
  ListActionButton,
  ListActions,
} from '@/components/common/list-actions';
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
import { Badge } from '@/components/ui/badge';
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import {
  NodeTagService,
  nodeTagQueryKey,
  type NodeTagItem,
} from '@/lib/services/openflare';
import { getErrorMessage } from './node-utils';

const TAG_COLORS = [
  'blue',
  'emerald',
  'amber',
  'rose',
  'violet',
  'cyan',
  'indigo',
  'orange',
  'teal',
  'slate',
] as const;

export function NodeTagsDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useTranslations('nodes.tags');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();

  const [editing, setEditing] = useState<NodeTagItem | null>(null);
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [color, setColor] = useState<string>('blue');
  const [defaultWeight, setDefaultWeight] = useState('1.0');
  const [description, setDescription] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<NodeTagItem | null>(null);

  const tagsQuery = useQuery({
    queryKey: nodeTagQueryKey,
    queryFn: () => NodeTagService.list(),
    enabled: open,
  });

  const resetForm = () => {
    setEditing(null);
    setName('');
    setSlug('');
    setColor('blue');
    setDefaultWeight('1.0');
    setDescription('');
  };

  const invalidateAll = async () => {
    await queryClient.invalidateQueries({ queryKey: nodeTagQueryKey });
    await queryClient.invalidateQueries({ queryKey: ['openflare', 'nodes'] });
  };

  const saveMutation = useMutation({
    mutationFn: async () => {
      const weight = Number(defaultWeight);
      const payload = {
        name: name.trim(),
        slug: slug.trim().toLowerCase(),
        color,
        description: description.trim(),
        default_weight: Number.isFinite(weight) && weight > 0 ? weight : 1.0,
      };
      if (editing) {
        return NodeTagService.update(editing.id, payload);
      }
      return NodeTagService.create(payload);
    },
    onSuccess: async () => {
      toast.success(editing ? t('updated') : t('created'));
      resetForm();
      await invalidateAll();
    },
    onError: (err) => toast.error(getErrorMessage(err, t('saveFailed'))),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => NodeTagService.deleteById(id),
    onSuccess: async () => {
      toast.success(t('deleted'));
      if (editing && deleteTarget?.id === editing.id) {
        resetForm();
      }
      setDeleteTarget(null);
      await invalidateAll();
    },
    onError: (err) => toast.error(getErrorMessage(err, t('deleteFailed'))),
  });

  const startEdit = (item: NodeTagItem) => {
    setEditing(item);
    setName(item.name);
    setSlug(item.slug);
    setColor(item.color || 'blue');
    setDefaultWeight(String(item.default_weight ?? 1.0));
    setDescription(item.description ?? '');
  };

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) {
      resetForm();
    }
    onOpenChange(nextOpen);
  };

  return (
    <>
      <Dialog open={open} onOpenChange={handleOpenChange}>
        <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              <Tags className='size-4 text-primary' />
              {t('title')}
            </DialogTitle>
            <DialogDescription>{t('description')}</DialogDescription>
          </DialogHeader>

          <div className='space-y-5'>
            <div className='rounded-lg border border-dashed p-4 space-y-3'>
              <div className='flex items-center justify-between'>
                <h3 className='text-sm font-medium'>
                  {editing ? t('editHeading') : t('createHeading')}
                </h3>
                {editing ? (
                  <Button
                    type='button'
                    variant='ghost'
                    size='sm'
                    onClick={resetForm}
                  >
                    {t('cancelEdit')}
                  </Button>
                ) : null}
              </div>

              <div className='grid gap-3 sm:grid-cols-2'>
                <div className='space-y-1.5'>
                  <Label htmlFor='node-tag-name'>{t('name')}</Label>
                  <Input
                    id='node-tag-name'
                    value={name}
                    placeholder={t('namePlaceholder')}
                    onChange={(e) => {
                      const nextName = e.target.value;
                      setName(nextName);
                      if (!editing && !slug.trim()) {
                        const autoSlug = nextName
                          .trim()
                          .toLowerCase()
                          .replace(/[^a-z0-9-]+/g, '-')
                          .replace(/^-+|-+$/g, '');
                        if (autoSlug) setSlug(autoSlug);
                      }
                    }}
                  />
                </div>
                <div className='space-y-1.5'>
                  <Label htmlFor='node-tag-slug'>{t('slug')}</Label>
                  <Input
                    id='node-tag-slug'
                    value={slug}
                    placeholder={t('slugPlaceholder')}
                    onChange={(e) => setSlug(e.target.value)}
                  />
                </div>
                <div className='space-y-1.5'>
                  <Label htmlFor='node-tag-color'>{t('color')}</Label>
                  <Select value={color} onValueChange={setColor}>
                    <SelectTrigger id='node-tag-color' className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {TAG_COLORS.map((col) => (
                        <SelectItem key={col} value={col}>
                          {col}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className='space-y-1.5'>
                  <Label htmlFor='node-tag-weight'>{t('defaultWeight')}</Label>
                  <Input
                    id='node-tag-weight'
                    type='number'
                    step='0.05'
                    min='0.01'
                    max='100'
                    value={defaultWeight}
                    onChange={(e) => setDefaultWeight(e.target.value)}
                  />
                </div>
                <div className='space-y-1.5 sm:col-span-2'>
                  <Label htmlFor='node-tag-desc'>{t('descLabel')}</Label>
                  <Input
                    id='node-tag-desc'
                    value={description}
                    placeholder={t('descPlaceholder')}
                    onChange={(e) => setDescription(e.target.value)}
                  />
                </div>
              </div>

              <div className='flex justify-end'>
                <Button
                  type='button'
                  size='sm'
                  disabled={
                    saveMutation.isPending || !name.trim() || !slug.trim()
                  }
                  onClick={() => saveMutation.mutate()}
                >
                  <Plus data-icon='inline-start' />
                  {editing ? t('saveEdit') : t('createAction')}
                </Button>
              </div>
            </div>

            <div className='space-y-2'>
              <h3 className='text-sm font-medium'>{t('listTitle')}</h3>
              {tagsQuery.isLoading ? (
                <LoadingStateWithBorder
                  icon={Tags}
                  description={t('loading')}
                />
              ) : tagsQuery.isError ? (
                <ErrorInline
                  message={getErrorMessage(tagsQuery.error, t('loadFailed'))}
                  onRetry={() => void tagsQuery.refetch()}
                />
              ) : (tagsQuery.data ?? []).length === 0 ? (
                <div className='rounded-lg border border-dashed py-6 text-center text-sm text-muted-foreground'>
                  {t('empty')}
                </div>
              ) : (
                <div className='space-y-2'>
                  {(tagsQuery.data ?? []).map((item) => (
                    <div
                      key={item.id}
                      className='flex items-center justify-between gap-3 rounded-lg border border-dashed px-3 py-2'
                    >
                      <div className='min-w-0 flex-1 space-y-0.5'>
                        <div className='flex flex-wrap items-center gap-2'>
                          <span className='font-medium text-sm'>
                            {item.name}
                          </span>
                          <Badge
                            variant='outline'
                            className='font-mono text-xs'
                          >
                            {item.slug}
                          </Badge>
                          <Badge variant='secondary' className='text-xs'>
                            {t('weightValue', {
                              weight: item.default_weight.toFixed(2),
                            })}
                          </Badge>
                          <Badge variant='outline' className='text-xs'>
                            {item.color}
                          </Badge>
                        </div>
                        {item.description ? (
                          <p className='truncate text-xs text-muted-foreground'>
                            {item.description}
                          </p>
                        ) : null}
                      </div>
                      <ListActions align='end'>
                        <ListActionButton
                          icon={Pencil}
                          label={t('edit')}
                          onClick={() => startEdit(item)}
                        />
                        <ListActionButton
                          icon={Trash2}
                          label={t('delete')}
                          tone='destructive'
                          onClick={() => setDeleteTarget(item)}
                        />
                      </ListActions>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(nextOpen) => !nextOpen && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('deleteDesc', { name: deleteTarget?.name ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tc('cancel')}</AlertDialogCancel>
            <AlertDialogAction
              className='bg-destructive text-white hover:bg-destructive/90'
              disabled={deleteMutation.isPending}
              onClick={() =>
                deleteTarget && deleteMutation.mutate(deleteTarget.id)
              }
            >
              {t('confirmDelete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
