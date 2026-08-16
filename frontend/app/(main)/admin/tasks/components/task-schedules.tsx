'use client';

import { useCallback, useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Switch } from '@/components/ui/switch';
import { Spinner } from '@/components/ui/spinner';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Clock, Edit2, Info, Plus, RefreshCw, Trash2 } from 'lucide-react';

import type {
  CreateScheduleRequest,
  Schedule,
  TaskMeta,
  UpdateScheduleRequest,
} from '@/lib/services/admin';
import services from '@/lib/services';
import { buildTaskPayload } from '@/lib/task-param-utils';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { EmptyStateWithBorder } from '@/components/layout/empty';

export function TaskSchedulesManager() {
  const t = useTranslations('admin.tasks');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const [schedules, setSchedules] = useState<Schedule[]>([]);
  const [taskTypes, setTaskTypes] = useState<TaskMeta[]>([]);

  // Modal States
  const [dialogOpen, setDialogOpen] = useState(false);
  const [submitLoading, setSubmitLoading] = useState(false);
  const [editingSchedule, setEditingSchedule] = useState<Schedule | null>(null);

  // Delete States
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);

  // Form Fields
  const [name, setName] = useState('');
  const [selectedTaskType, setSelectedTaskType] = useState('');
  const [cron, setCron] = useState('');
  const [isActive, setIsActive] = useState(true);
  const [paramValues, setParamValues] = useState<Record<string, string>>({});

  // Fetch Schedules & Task Types
  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const [schedulesData, taskTypesData] = await Promise.all([
        services.adminTask.listSchedules(),
        services.adminTask.getTaskTypes(),
      ]);
      setSchedules(schedulesData || []);
      setTaskTypes(taskTypesData || []);
    } catch (err) {
      setError(
        err instanceof Error ? err : new Error(t('loadSchedulesFailed')),
      );
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Track task type selection to initialize parameters
  useEffect(() => {
    if (selectedTaskType) {
      const targetTask = taskTypes.find((t) => t.type === selectedTaskType);
      if (editingSchedule && editingSchedule.task_type === selectedTaskType) {
        // Parse existing payload when editing the same task type
        try {
          const parsed = JSON.parse(editingSchedule.payload || '{}');
          const values: Record<string, string> = {};
          targetTask?.params?.forEach((p) => {
            values[p.name] = String(parsed[p.name] ?? '');
          });
          setParamValues(values);
        } catch {
          const initialValues: Record<string, string> = {};
          targetTask?.params?.forEach((p) => {
            initialValues[p.name] = '';
          });
          setParamValues(initialValues);
        }
      } else {
        const initialValues: Record<string, string> = {};
        targetTask?.params?.forEach((p) => {
          initialValues[p.name] = '';
        });
        setParamValues(initialValues);
      }
    } else {
      setParamValues({});
    }
  }, [selectedTaskType, taskTypes, editingSchedule]);

  const handleToggleActive = async (schedule: Schedule) => {
    try {
      const updated = await services.adminTask.updateSchedule(schedule.id, {
        name: schedule.name,
        task_type: schedule.task_type,
        cron: schedule.cron,
        payload: schedule.payload,
        is_active: !schedule.is_active,
      });

      setSchedules((prev) =>
        prev.map((s) => (s.id === schedule.id ? updated : s)),
      );
      toast.success(
        updated.is_active
          ? t('scheduleEnabled', { name: updated.name })
          : t('scheduleDisabled', { name: updated.name }),
      );
    } catch (err) {
      toast.error(t('changeScheduleStatusFailed'), {
        description: err instanceof Error ? err.message : t('unknownError'),
      });
    }
  };

  const openCreateDialog = () => {
    setEditingSchedule(null);
    setName('');
    setSelectedTaskType(taskTypes[0]?.type || '');
    setCron('0 */2 * * *');
    setIsActive(true);
    setParamValues({});
    setDialogOpen(true);
  };

  const openEditDialog = (schedule: Schedule) => {
    setEditingSchedule(schedule);
    setName(schedule.name);
    setSelectedTaskType(schedule.task_type);
    setCron(schedule.cron);
    setIsActive(schedule.is_active);
    setDialogOpen(true);
  };

  const handleSubmit = async () => {
    if (!name.trim()) {
      toast.error(t('scheduleNameRequired'));
      return;
    }
    if (!cron.trim()) {
      toast.error(t('cronRequired'));
      return;
    }

    try {
      setSubmitLoading(true);

      const targetTask = taskTypes.find((t) => t.type === selectedTaskType);
      let payloadStr = '{}';

      if (targetTask?.params && targetTask.params.length > 0) {
        const { payload, error } = buildTaskPayload(
          targetTask.params,
          paramValues,
        );
        if (error) {
          toast.error(error);
          setSubmitLoading(false);
          return;
        }
        payloadStr = payload ?? '{}';
      }

      if (editingSchedule) {
        const req: UpdateScheduleRequest = {
          name,
          task_type: selectedTaskType,
          cron,
          payload: payloadStr,
          is_active: isActive,
        };
        const updated = await services.adminTask.updateSchedule(
          editingSchedule.id,
          req,
        );
        setSchedules((prev) =>
          prev.map((s) => (s.id === editingSchedule.id ? updated : s)),
        );
        toast.success(t('scheduleUpdateSuccess'));
      } else {
        const req: CreateScheduleRequest = {
          name,
          task_type: selectedTaskType,
          cron,
          payload: payloadStr,
          is_active: isActive,
        };
        const created = await services.adminTask.createSchedule(req);
        setSchedules((prev) => [created, ...prev]);
        toast.success(t('scheduleCreateSuccess'));
      }

      setDialogOpen(false);
    } catch (err) {
      toast.error(t('submitFailed'), {
        description: err instanceof Error ? err.message : t('unknownError'),
      });
    } finally {
      setSubmitLoading(false);
    }
  };

  const openDeleteDialog = (id: string) => {
    setDeletingId(id);
    setDeleteOpen(true);
  };

  const handleDelete = async () => {
    if (!deletingId) return;
    try {
      setDeleteLoading(true);
      await services.adminTask.deleteSchedule(deletingId);
      setSchedules((prev) => prev.filter((s) => s.id !== deletingId));
      toast.success(t('scheduleDeleteSuccess'));
      setDeleteOpen(false);
    } catch (err) {
      toast.error(t('deleteFailed'), {
        description: err instanceof Error ? err.message : t('unknownError'),
      });
    } finally {
      setDeleteLoading(false);
    }
  };

  const getSelectedTaskMeta = () => {
    return taskTypes.find((t) => t.type === selectedTaskType);
  };

  const getTaskName = (type: string) => {
    const meta = taskTypes.find((t) => t.type === type);
    return meta ? meta.name : type;
  };

  return (
    <div className='space-y-6'>
      <br />
      <div className='flex items-center justify-between'>
        <div className='flex items-center gap-2'>
          <Button
            variant='outline'
            size='sm'
            onClick={fetchData}
            disabled={loading}
          >
            {loading ? (
              <Spinner className='size-4' />
            ) : (
              <RefreshCw className='size-4' />
            )}
            {t('refresh')}
          </Button>
          <Button
            size='sm'
            onClick={openCreateDialog}
            disabled={taskTypes.length === 0}
            variant={'secondary'}
          >
            <Plus className='size-4 mr-1' />
            {t('addSchedule')}
          </Button>
        </div>
      </div>

      {error ? (
        <div className='p-8 border border-dashed rounded-lg bg-card'>
          <ErrorInline
            error={error}
            onRetry={fetchData}
            className='justify-center'
          />
        </div>
      ) : loading && schedules.length === 0 ? (
        <LoadingStateWithBorder
          icon={Clock}
          description={t('loadingSchedules')}
        />
      ) : schedules.length === 0 ? (
        <EmptyStateWithBorder icon={Clock} description={t('noSchedules')} />
      ) : (
        <div className='rounded-lg border bg-card'>
          <Table className='min-w-[800px]'>
            <TableHeader>
              <TableRow className='hover:bg-transparent'>
                <TableHead className='w-[180px]'>
                  {t('colScheduleName')}
                </TableHead>
                <TableHead className='w-[160px]'>
                  {t('colAssociatedTask')}
                </TableHead>
                <TableHead className='w-[120px]'>{t('colCron')}</TableHead>
                <TableHead className='min-w-[200px]'>
                  {t('colPayload')}
                </TableHead>
                <TableHead className='w-[100px] text-center'>
                  {t('colEnabled')}
                </TableHead>
                <TableHead className='w-[120px] text-right'>
                  {t('colActions')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {schedules.map((schedule) => (
                <TableRow key={schedule.id}>
                  <TableCell className='font-medium'>{schedule.name}</TableCell>
                  <TableCell>
                    <div className='flex flex-col gap-0.5'>
                      <span className='text-xs font-semibold'>
                        {getTaskName(schedule.task_type)}
                      </span>
                      <span className='font-mono text-[10px] text-muted-foreground'>
                        {schedule.task_type}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell className='font-mono text-xs text-blue-600 dark:text-blue-400 font-semibold'>
                    {schedule.cron}
                  </TableCell>
                  <TableCell className='max-w-[300px] truncate'>
                    <code className='text-xs bg-muted/60 px-1 py-0.5 rounded font-mono'>
                      {schedule.payload || '{}'}
                    </code>
                  </TableCell>
                  <TableCell className='text-center'>
                    <div className='flex justify-center'>
                      <Switch
                        checked={schedule.is_active}
                        onCheckedChange={() => handleToggleActive(schedule)}
                      />
                    </div>
                  </TableCell>
                  <TableCell className='text-right'>
                    <div className='flex items-center justify-end gap-1.5'>
                      <Button
                        variant='ghost'
                        size='icon'
                        className='h-7 w-7 text-muted-foreground hover:text-foreground'
                        onClick={() => openEditDialog(schedule)}
                      >
                        <Edit2 className='size-3.5' />
                      </Button>
                      <Button
                        variant='ghost'
                        size='icon'
                        className='h-7 w-7 text-destructive hover:text-destructive/90'
                        onClick={() => openDeleteDialog(schedule.id)}
                      >
                        <Trash2 className='size-3.5' />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {/* Add / Edit Dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className='sm:max-w-[500px]'>
          <DialogHeader>
            <DialogTitle>
              {editingSchedule ? t('editSchedule') : t('addSchedule')}
            </DialogTitle>
            <DialogDescription>{t('scheduleDialogDesc')}</DialogDescription>
          </DialogHeader>

          <div className='grid gap-4 py-4'>
            <div className='grid gap-2'>
              <Label htmlFor='sched-name'>{t('scheduleName')}</Label>
              <Input
                id='sched-name'
                placeholder={t('scheduleNamePlaceholder')}
                value={name}
                onChange={(e) => setName(e.target.value)}
                className='text-xs'
              />
            </div>

            <div className='grid gap-2'>
              <Label htmlFor='sched-type'>{t('associatedTask')}</Label>
              <select
                id='sched-type'
                value={selectedTaskType}
                onChange={(e) => setSelectedTaskType(e.target.value)}
                disabled={!!editingSchedule}
                className='flex h-8 w-full rounded-md border border-input bg-background px-3 py-1.5 text-xs shadow-sm transition-colors file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50'
              >
                {taskTypes.map((t) => (
                  <option key={t.type} value={t.type}>
                    {t.name} ({t.type})
                  </option>
                ))}
              </select>
            </div>

            <div className='grid gap-2'>
              <Label htmlFor='sched-cron'>{t('cronExpression')}</Label>
              <Input
                id='sched-cron'
                placeholder={t('cronPlaceholder')}
                value={cron}
                onChange={(e) => setCron(e.target.value)}
                className='text-xs font-mono'
              />
              <p className='text-[10px] text-muted-foreground'>
                {t('cronHint')}
              </p>
            </div>

            <div className='flex items-center justify-between rounded-lg border p-3'>
              <div className='space-y-0.5'>
                <Label htmlFor='sched-active'>{t('scheduleActive')}</Label>
                <p className='text-[10px] text-muted-foreground'>
                  {t('scheduleActiveDesc')}
                </p>
              </div>
              <Switch
                id='sched-active'
                checked={isActive}
                onCheckedChange={setIsActive}
              />
            </div>

            {(() => {
              const targetTask = getSelectedTaskMeta();
              if (
                !targetTask ||
                !targetTask.params ||
                targetTask.params.length === 0
              ) {
                return (
                  <div className='flex items-center gap-2 text-xs text-muted-foreground bg-muted/40 p-3 rounded-md border border-dashed'>
                    <Info className='h-4 w-4 shrink-0' />
                    <span>{t('noParamsRequired')}</span>
                  </div>
                );
              }
              return (
                <div className='space-y-4 pt-2 border-t'>
                  <Label className='text-xs font-semibold'>
                    {t('payloadConfig')}
                  </Label>
                  {targetTask.params.map((param) => (
                    <div
                      key={param.name}
                      className='grid gap-2 pl-2 border-l-2 border-muted'
                    >
                      <Label
                        htmlFor={`param-${param.name}`}
                        className='flex items-center gap-1 text-xs'
                      >
                        {param.label}
                        {param.required && (
                          <span className='text-destructive font-bold'>*</span>
                        )}
                      </Label>
                      {param.type === 'text' ? (
                        <Textarea
                          id={`param-${param.name}`}
                          placeholder={param.placeholder}
                          className='text-xs min-h-[70px]'
                          value={paramValues[param.name] || ''}
                          onChange={(e) =>
                            setParamValues((prev) => ({
                              ...prev,
                              [param.name]: e.target.value,
                            }))
                          }
                        />
                      ) : param.type === 'boolean' ? (
                        <div className='flex items-center gap-2 pt-1 h-9'>
                          <Switch
                            id={`param-${param.name}`}
                            checked={paramValues[param.name] === 'true'}
                            onCheckedChange={(checked) =>
                              setParamValues((prev) => ({
                                ...prev,
                                [param.name]: checked ? 'true' : 'false',
                              }))
                            }
                          />
                          <span className='text-xs text-muted-foreground'>
                            {paramValues[param.name] === 'true'
                              ? t('paramOn')
                              : t('paramOff')}
                          </span>
                        </div>
                      ) : (
                        <Input
                          id={`param-${param.name}`}
                          type={param.type === 'number' ? 'number' : 'text'}
                          placeholder={param.placeholder}
                          className='text-xs'
                          value={paramValues[param.name] || ''}
                          onChange={(e) =>
                            setParamValues((prev) => ({
                              ...prev,
                              [param.name]: e.target.value,
                            }))
                          }
                        />
                      )}
                      {param.description && (
                        <p className='text-[10px] text-muted-foreground'>
                          {param.description}
                        </p>
                      )}
                    </div>
                  ))}
                </div>
              );
            })()}
          </div>

          <DialogFooter>
            <Button
              variant='ghost'
              onClick={() => setDialogOpen(false)}
              disabled={submitLoading}
              className='h-8 text-xs'
            >
              {t('cancel')}
            </Button>
            <Button
              onClick={handleSubmit}
              disabled={submitLoading}
              className='h-8 text-xs'
            >
              {submitLoading && <Spinner className='size-3 mr-1' />}
              {editingSchedule ? t('saveChanges') : t('confirmCreate')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation */}
      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent className='sm:max-w-[400px]'>
          <DialogHeader>
            <DialogTitle>{t('deleteScheduleTitle')}</DialogTitle>
            <DialogDescription>{t('deleteScheduleDesc')}</DialogDescription>
          </DialogHeader>
          <DialogFooter className='gap-2 sm:gap-0'>
            <Button
              variant='ghost'
              onClick={() => setDeleteOpen(false)}
              disabled={deleteLoading}
              className='h-8 text-xs'
            >
              {t('cancel')}
            </Button>
            <Button
              variant='destructive'
              onClick={handleDelete}
              disabled={deleteLoading}
              className='h-8 text-xs'
            >
              {deleteLoading && <Spinner className='size-3 mr-1' />}
              {t('confirmDelete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
