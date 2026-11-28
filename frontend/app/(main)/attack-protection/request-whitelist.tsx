import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Pencil, Plus, Trash2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useState, type ReactNode } from 'react';

import {
  ListActionButton,
  ListActions,
} from '@/components/common/list-actions';
import { ErrorInline } from '@/components/layout/error';
import { LoadingState } from '@/components/layout/loading';
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { ApiErrorBase } from '@/lib/services/core';
import {
  whitelistPathMatches,
  type RequestWhitelistEntry,
  type WhitelistPathMatch,
} from '@/lib/services/openflare/attack-types';
import {
  requestWhitelistQueryKey,
  WafService,
} from '@/lib/services/openflare/waf.service';

export function RequestWhitelist() {
  const t = useTranslations('attackProtection');
  const entries = useQuery({
    queryKey: requestWhitelistQueryKey,
    queryFn: () => WafService.listRequestWhitelist(),
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('whitelist.title')}</CardTitle>
        <CardDescription>{t('whitelist.description')}</CardDescription>
        <CardAction>
          <WhitelistEditor>
            <Button variant='outline' size='sm'>
              <Plus data-icon='inline-start' />
              {t('whitelist.add')}
            </Button>
          </WhitelistEditor>
        </CardAction>
      </CardHeader>
      <CardContent>
        {entries.isPending && <LoadingState />}
        {entries.isError && (
          <ErrorInline
            message={t('whitelist.loadFailed')}
            onRetry={() => void entries.refetch()}
          />
        )}
        {entries.data?.length === 0 && (
          <p className='text-sm text-muted-foreground'>
            {t('whitelist.empty')}
          </p>
        )}
        {entries.data && entries.data.length > 0 && (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('whitelist.domain')}</TableHead>
                <TableHead>{t('whitelist.pathMatch')}</TableHead>
                <TableHead>{t('whitelist.path')}</TableHead>
                <TableHead className='text-center'>
                  <span className='sr-only'>{t('whitelist.actions')}</span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {entries.data.map((entry) => (
                <TableRow key={entry.id}>
                  <TableCell className='min-w-32 whitespace-normal wrap-anywhere'>
                    {entry.domain}
                  </TableCell>
                  <TableCell>
                    {t(`whitelist.modes.${entry.path_match}`)}
                  </TableCell>
                  <TableCell className='min-w-32 whitespace-normal wrap-anywhere font-mono'>
                    {entry.path}
                  </TableCell>
                  <TableCell className='text-center'>
                    <ListActions>
                      <WhitelistEditor
                        entry={entry}
                        trigger={(openDialog) => (
                          <ListActionButton
                            icon={Pencil}
                            label={t('edit')}
                            onClick={openDialog}
                          />
                        )}
                      />
                      <DeleteWhitelistEntry entry={entry} />
                    </ListActions>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

function WhitelistEditor({
  entry,
  children,
  trigger,
}: {
  entry?: RequestWhitelistEntry;
  children?: ReactNode;
  trigger?: (openDialog: () => void) => ReactNode;
}) {
  const [open, setOpen] = useState(false);
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      {trigger ? (
        trigger(() => setOpen(true))
      ) : children ? (
        <DialogTrigger asChild>{children}</DialogTrigger>
      ) : null}
      <DialogContent>
        <WhitelistForm entry={entry} onClose={() => setOpen(false)} />
      </DialogContent>
    </Dialog>
  );
}

function WhitelistForm({
  entry,
  onClose,
}: {
  entry?: RequestWhitelistEntry;
  onClose: () => void;
}) {
  const t = useTranslations('attackProtection');
  const client = useQueryClient();
  const [domain, setDomain] = useState(entry?.domain ?? '');
  const [pathMatch, setPathMatch] = useState<WhitelistPathMatch>(
    entry?.path_match ?? 'exact',
  );
  const [path, setPath] = useState(entry?.path ?? '');
  const save = useMutation({
    mutationFn: () =>
      WafService.saveRequestWhitelistEntry(
        { domain: domain.trim(), path_match: pathMatch, path: path.trim() },
        entry?.id,
      ),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: requestWhitelistQueryKey });
      onClose();
    },
  });

  return (
    <>
      <DialogHeader>
        <DialogTitle>
          {t(entry ? 'whitelist.edit' : 'whitelist.add')}
        </DialogTitle>
        <DialogDescription>{t('whitelist.description')}</DialogDescription>
      </DialogHeader>
      <form
        className='flex flex-col gap-4'
        onSubmit={(event) => {
          event.preventDefault();
          save.mutate();
        }}
      >
        <FieldGroup className='gap-4'>
          <Field>
            <FieldLabel htmlFor='whitelist-domain'>
              {t('whitelist.domain')}
            </FieldLabel>
            <Input
              id='whitelist-domain'
              placeholder='*.example.com'
              value={domain}
              onChange={(event) => setDomain(event.target.value)}
              maxLength={253}
              disabled={save.isPending}
              required
            />
            <FieldDescription>{t('whitelist.domainHelp')}</FieldDescription>
          </Field>
          <Field>
            <FieldLabel htmlFor='whitelist-path-match'>
              {t('whitelist.pathMatch')}
            </FieldLabel>
            <Select
              value={pathMatch}
              onValueChange={(value: WhitelistPathMatch) => setPathMatch(value)}
              disabled={save.isPending}
            >
              <SelectTrigger id='whitelist-path-match'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {whitelistPathMatches.map((mode) => (
                    <SelectItem key={mode} value={mode}>
                      {t(`whitelist.modes.${mode}`)}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
          <Field>
            <FieldLabel htmlFor='whitelist-path'>
              {t('whitelist.path')}
            </FieldLabel>
            <Input
              id='whitelist-path'
              value={path}
              onChange={(event) => setPath(event.target.value)}
              maxLength={1024}
              disabled={save.isPending}
              required
            />
            <FieldDescription>
              {t(`whitelist.pathHelp.${pathMatch}`)}
            </FieldDescription>
          </Field>
        </FieldGroup>
        {save.isError && (
          <p role='alert' className='text-sm text-destructive'>
            {t(
              save.error instanceof ApiErrorBase &&
                save.error.statusCode === 400
                ? 'whitelist.invalidEntry'
                : 'whitelist.saveFailed',
            )}
          </p>
        )}
        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={onClose}
            disabled={save.isPending}
          >
            {t('cancel')}
          </Button>
          <Button
            type='submit'
            disabled={!domain.trim() || !path.trim() || save.isPending}
          >
            {t('save')}
          </Button>
        </DialogFooter>
      </form>
    </>
  );
}

function DeleteWhitelistEntry({ entry }: { entry: RequestWhitelistEntry }) {
  const t = useTranslations('attackProtection');
  const client = useQueryClient();
  const [open, setOpen] = useState(false);
  const remove = useMutation({
    mutationFn: () => WafService.deleteRequestWhitelistEntry(entry.id),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: requestWhitelistQueryKey });
      setOpen(false);
    },
  });

  return (
    <>
      <ListActionButton
        icon={Trash2}
        label={t('whitelist.delete')}
        tone='destructive'
        onClick={() => setOpen(true)}
      />
      <AlertDialog
        open={open}
        onOpenChange={(next) => {
          if (!next) remove.reset();
          setOpen(next);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('whitelist.deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('whitelist.deleteDescription', {
                domain: entry.domain,
                path: entry.path,
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          {remove.isError && (
            <p role='alert' className='text-sm text-destructive'>
              {t('whitelist.deleteFailed')}
            </p>
          )}
          <AlertDialogFooter>
            <AlertDialogCancel disabled={remove.isPending}>
              {t('cancel')}
            </AlertDialogCancel>
            <Button
              variant='destructive'
              onClick={() => remove.mutate()}
              disabled={remove.isPending}
            >
              {t('whitelist.delete')}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
