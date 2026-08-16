'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { FileText, Loader2, Pencil, Plus, Trash2 } from 'lucide-react';

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
import { Textarea } from '@/components/ui/textarea';
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
import services from '@/lib/services';
import type { Template } from '@/lib/services/admin/types';
import { toast } from 'sonner';
import { useTranslations } from 'next-intl';

export function TemplatesManager() {
  const queryClient = useQueryClient();
  const t = useTranslations('settings.templates');
  const [modalOpen, setModalOpen] = useState(false);
  const [selectedTemplate, setSelectedTemplate] = useState<Template | null>(
    null,
  );
  const [deleteTarget, setDeleteTarget] = useState<Template | null>(null);

  // Form states
  const [key, setKey] = useState('');
  const [name, setName] = useState('');
  const [type, setType] = useState('email');
  const [subject, setSubject] = useState('');
  const [content, setContent] = useState('');
  const [description, setDescription] = useState('');

  const templatesQuery = useQuery({
    queryKey: ['admin', 'templates'],
    queryFn: () => services.adminTemplate.listTemplates(),
  });

  const createTemplateMutation = useMutation({
    mutationFn: async () => {
      await services.adminTemplate.createTemplate({
        key,
        name,
        type,
        subject,
        content,
        description,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['admin', 'templates'] });
      toast.success(t('templateCreated'));
      setModalOpen(false);
    },
    onError: (error: Error) => {
      toast.error(error.message || t('createTemplateFailed'));
    },
  });

  const updateTemplateMutation = useMutation({
    mutationFn: async (key: string) => {
      await services.adminTemplate.updateTemplate(key, {
        name,
        type,
        subject,
        content,
        description,
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['admin', 'templates'] });
      toast.success(t('templateSaved'));
      setModalOpen(false);
    },
    onError: (error: Error) => {
      toast.error(error.message || t('updateTemplateFailed'));
    },
  });

  const deleteTemplateMutation = useMutation({
    mutationFn: async (key: string) => {
      await services.adminTemplate.deleteTemplate(key);
    },
    onSuccess: async () => {
      setDeleteTarget(null);
      await queryClient.invalidateQueries({ queryKey: ['admin', 'templates'] });
      toast.success(t('templateDeleted'));
    },
    onError: (error: Error) => {
      toast.error(error.message || t('deleteTemplateFailed'));
    },
  });

  const handleOpenCreate = () => {
    setSelectedTemplate(null);
    setKey('');
    setName('');
    setType('email');
    setSubject('');
    setContent('');
    setDescription('');
    setModalOpen(true);
  };

  const handleOpenEdit = (tmpl: Template) => {
    setSelectedTemplate(tmpl);
    setKey(tmpl.key);
    setName(tmpl.name);
    setType(tmpl.type);
    setSubject(tmpl.subject || '');
    setContent(tmpl.content);
    setDescription(tmpl.description || '');
    setModalOpen(true);
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (selectedTemplate) {
      updateTemplateMutation.mutate(selectedTemplate.key);
    } else {
      createTemplateMutation.mutate();
    }
  };

  const isPending =
    createTemplateMutation.isPending || updateTemplateMutation.isPending;

  return (
    <div className='space-y-6'>
      <Card className='border border-dashed shadow-sm'>
        <CardHeader className='border-b border-dashed pb-4 flex flex-row items-center justify-between gap-4'>
          <div className='flex items-center gap-2'>
            <div className='p-1.5 rounded-lg bg-primary/10 text-primary'>
              <FileText className='size-4' />
            </div>
            <div>
              <CardTitle className='text-base font-semibold'>
                {t('templateManagement')}
              </CardTitle>
              <CardDescription className='text-xs'>
                {t('templateManagementDesc')}
              </CardDescription>
            </div>
          </div>
          <Button
            type='button'
            size='sm'
            onClick={handleOpenCreate}
            variant='secondary'
          >
            <Plus className='mr-1.5 size-3.5' />
            {t('addTemplate')}
          </Button>
        </CardHeader>
        <CardContent className='pt-6 space-y-4'>
          {templatesQuery.isPending ? (
            <div className='flex items-center justify-center p-8'>
              <Loader2 className='size-6 animate-spin text-muted-foreground/50' />
            </div>
          ) : (templatesQuery.data ?? []).length > 0 ? (
            <div className='grid grid-cols-1 gap-4'>
              {(templatesQuery.data ?? []).map((tmpl) => (
                <div
                  key={tmpl.id}
                  className='flex flex-col sm:flex-row sm:items-center justify-between gap-4 rounded-xl border border-dashed p-4 bg-card hover:bg-muted/10 hover:border-primary/30 transition-all duration-300 shadow-sm'
                >
                  <div className='space-y-1.5'>
                    <div className='flex items-center gap-2 flex-wrap'>
                      <span className='font-semibold text-sm text-foreground'>
                        {tmpl.name}
                      </span>
                      <span
                        className={`text-[10px] px-2 py-0.5 rounded-full border font-medium ${
                          tmpl.is_system
                            ? 'bg-primary/10 text-primary border-primary/20'
                            : 'bg-amber-500/10 text-amber-500 border-amber-500/20'
                        }`}
                      >
                        {tmpl.is_system ? t('systemBuiltIn') : t('custom')}
                      </span>
                      <span className='text-[10px] px-2 py-0.5 rounded-full border border-border/50 bg-muted/50 text-muted-foreground font-mono'>
                        {tmpl.type.toUpperCase()}
                      </span>
                    </div>
                    <div className='text-xs text-muted-foreground'>
                      {t('identifier')}:{' '}
                      <span className='font-mono text-primary bg-primary/5 px-1.5 py-0.5 rounded'>
                        {tmpl.key}
                      </span>
                      {tmpl.subject && ` · ${t('subject')}: ${tmpl.subject}`}
                    </div>
                    {tmpl.description && (
                      <p className='text-xs text-muted-foreground/80 leading-relaxed max-w-xl'>
                        {tmpl.description}
                      </p>
                    )}
                  </div>
                  <div className='flex items-center justify-end gap-2 shrink-0'>
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      className='size-8 text-muted-foreground hover:text-primary hover:bg-primary/10 rounded-lg transition-colors'
                      onClick={() => handleOpenEdit(tmpl)}
                    >
                      <Pencil className='size-4' />
                    </Button>
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      className='size-8 text-muted-foreground hover:text-rose-500 hover:bg-rose-500/10 rounded-lg transition-colors'
                      disabled={
                        tmpl.is_system || deleteTemplateMutation.isPending
                      }
                      onClick={() => setDeleteTarget(tmpl)}
                    >
                      <Trash2 className='size-4' />
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className='rounded-xl border border-dashed border-border/50 px-4 py-8 text-center text-xs text-muted-foreground bg-muted/5 flex flex-col items-center justify-center gap-3'>
              <span>{t('noTemplates')}</span>
              <Button
                type='button'
                size='sm'
                variant='outline'
                onClick={handleOpenCreate}
                className='border-dashed'
              >
                <Plus className='mr-1.5 size-3.5' />
                {t('addTemplate')}
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      <Dialog open={modalOpen} onOpenChange={setModalOpen}>
        <DialogContent className='max-w-xl border border-dashed'>
          <DialogHeader>
            <DialogTitle className='text-base font-semibold'>
              {selectedTemplate ? t('editTemplate') : t('addTemplateTitle')}
            </DialogTitle>
            <DialogDescription className='text-xs'>
              {t('templateDesc')}
            </DialogDescription>
          </DialogHeader>

          <form onSubmit={handleSubmit} className='space-y-4'>
            <div className='grid grid-cols-1 sm:grid-cols-2 gap-4'>
              <div className='space-y-1.5'>
                <Label htmlFor='tmpl_key' className='text-xs font-semibold'>
                  {t('templateKey')}
                </Label>
                <Input
                  id='tmpl_key'
                  type='text'
                  required
                  value={key}
                  onChange={(e) => setKey(e.target.value)}
                  placeholder='例如: login_email'
                  className='bg-card border-dashed text-xs'
                  disabled={!!selectedTemplate}
                />
                <p className='text-[10px] text-muted-foreground leading-normal'>
                  {t('templateKeyDesc')}
                </p>
              </div>

              <div className='space-y-1.5'>
                <Label htmlFor='tmpl_name' className='text-xs font-semibold'>
                  {t('templateName')}
                </Label>
                <Input
                  id='tmpl_name'
                  type='text'
                  required
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder='例如: 登录验证码邮件'
                  className='bg-card border-dashed text-xs'
                />
                <p className='text-[10px] text-muted-foreground leading-normal'>
                  {t('templateNameDesc')}
                </p>
              </div>

              <div className='space-y-1.5'>
                <Label htmlFor='tmpl_type' className='text-xs font-semibold'>
                  {t('templateType')}
                </Label>
                <Input
                  id='tmpl_type'
                  type='text'
                  required
                  value={type}
                  onChange={(e) => setType(e.target.value)}
                  placeholder='email'
                  className='bg-card border-dashed text-xs'
                />
                <p className='text-[10px] text-muted-foreground leading-normal'>
                  {t('templateTypeDesc')}
                </p>
              </div>

              <div className='space-y-1.5'>
                <Label htmlFor='tmpl_subject' className='text-xs font-semibold'>
                  {t('templateSubject')}
                </Label>
                <Input
                  id='tmpl_subject'
                  type='text'
                  value={subject}
                  onChange={(e) => setSubject(e.target.value)}
                  placeholder='例如: OpenFlare 登录验证码'
                  className='bg-card border-dashed text-xs'
                />
                <p className='text-[10px] text-muted-foreground leading-normal'>
                  {t('templateSubjectDesc')}
                </p>
              </div>
            </div>

            <div className='space-y-1.5'>
              <Label
                htmlFor='tmpl_description'
                className='text-xs font-semibold'
              >
                {t('templateDescription')}
              </Label>
              <Input
                id='tmpl_description'
                type='text'
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder='例如: 包含变量：{{.Code}}，5分钟内有效'
                className='bg-card border-dashed text-xs'
              />
            </div>

            <div className='space-y-1.5'>
              <Label htmlFor='tmpl_content' className='text-xs font-semibold'>
                {t('templateContent')}
              </Label>
              <Textarea
                id='tmpl_content'
                required
                value={content}
                onChange={(e) => setContent(e.target.value)}
                placeholder='<h3>正文标题</h3><p>内容段落，可用变量 {{.Code}}</p>'
                rows={8}
                className='bg-card border-dashed text-xs font-mono'
              />
            </div>

            <DialogFooter className='gap-2 sm:gap-0 border-t border-dashed pt-4'>
              <Button
                type='button'
                variant='ghost'
                size='sm'
                onClick={() => setModalOpen(false)}
                disabled={isPending}
              >
                {t('cancel')}
              </Button>
              <Button type='submit' size='sm' disabled={isPending}>
                {isPending ? (
                  <>
                    <Loader2 className='mr-1.5 size-3.5 animate-spin' />
                    {t('saving')}
                  </>
                ) : (
                  t('saveConfig')
                )}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('deleteTemplateTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('deleteTemplateConfirm', { name: deleteTarget?.name ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteTemplateMutation.isPending}>
              {t('cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={deleteTemplateMutation.isPending}
              onClick={() =>
                deleteTarget && deleteTemplateMutation.mutate(deleteTarget.key)
              }
            >
              {deleteTemplateMutation.isPending
                ? t('deleting')
                : t('confirmDelete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
