// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { useTranslations } from 'next-intl';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

import { Edit2, Loader2, Play, Plus, Settings, Trash2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Switch } from '@/components/ui/switch';
import { Badge } from '@/components/ui/badge';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';

import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';

import type {
  ChannelDefinition,
  CreateChannelRequest,
  PushChannel,
  UpdateChannelRequest,
} from '@/lib/services/push';
import { PushService } from '@/lib/services/push';

export function SettingsTab() {
  const t = useTranslations('admin.push.settings');
  const queryClient = useQueryClient();
  const [deleteTarget, setDeleteTarget] = React.useState<PushChannel | null>(
    null,
  );

  // --- 获取所有自定义消息通道 ---
  const channelsQuery = useQuery({
    queryKey: ['admin', 'push-channels'],
    queryFn: () => PushService.listChannels(),
  });

  // --- 获取动态通道表单字段定义 ---
  const definitionsQuery = useQuery({
    queryKey: ['admin', 'push-channels-definitions'],
    queryFn: () => PushService.listChannelDefinitions(),
  });

  // --- 消息通道 CRUD Mutations ---
  const createChannelMutation = useMutation({
    mutationFn: (data: CreateChannelRequest) => PushService.createChannel(data),
    onSuccess: () => {
      toast.success(t('channelCreateSuccess'));
      queryClient.invalidateQueries({ queryKey: ['admin', 'push-channels'] });
      setChannelDialogOpen(false);
    },
    onError: (err: unknown) => {
      toast.error(t('channelCreateFailed') + ': ' + (err as Error).message);
    },
  });

  const updateChannelMutation = useMutation({
    mutationFn: ({ id, data }: { id: number; data: UpdateChannelRequest }) =>
      PushService.updateChannel(id, data),
    onSuccess: () => {
      toast.success(t('channelUpdateSuccess'));
      queryClient.invalidateQueries({ queryKey: ['admin', 'push-channels'] });
      setChannelDialogOpen(false);
    },
    onError: (err: unknown) => {
      toast.error(t('channelUpdateFailed') + ': ' + (err as Error).message);
    },
  });

  const deleteChannelMutation = useMutation({
    mutationFn: (id: number) => PushService.deleteChannel(id),
    onSuccess: () => {
      setDeleteTarget(null);
      toast.success(t('channelDeleteSuccess'));
      queryClient.invalidateQueries({ queryKey: ['admin', 'push-channels'] });
    },
    onError: (err: unknown) => {
      toast.error(t('channelDeleteFailed') + ': ' + (err as Error).message);
    },
  });

  // --- 消息通道与设置相关 State ---
  const [channelDialogOpen, setChannelDialogOpen] = React.useState(false);
  const [editingChannel, setEditingChannel] =
    React.useState<PushChannel | null>(null);
  const [channelName, setChannelName] = React.useState('');
  const [channelDescription, setChannelDescription] = React.useState('');
  const [channelType, setChannelType] = React.useState('custom');
  const [channelToken, setChannelToken] = React.useState('');
  const [channelUrl, setChannelUrl] = React.useState('');
  const [channelOther, setChannelOther] = React.useState('');

  const activeDef = React.useMemo<ChannelDefinition | undefined>(() => {
    return (definitionsQuery.data ?? []).find((d) => d.type === channelType);
  }, [definitionsQuery.data, channelType]);

  const [testChannelOpen, setTestChannelOpen] = React.useState(false);
  const [testChannelName, setTestChannelName] = React.useState('');
  const [testChannelTarget, setTestChannelTarget] = React.useState('');
  const [isTestingChannel, setIsTestingChannel] = React.useState(false);

  const handleChannelTypeChange = (newType: string) => {
    setChannelType(newType);
    setChannelUrl('');
    setChannelToken('');
    if (newType === 'custom') {
      setChannelOther(
        JSON.stringify(
          {
            title: '$title',
            description: '$description',
            content: '$content',
            url: '$url',
            to: '$to',
          },
          null,
          2,
        ),
      );
    } else {
      setChannelOther('');
    }
  };

  const handleCreateChannelClick = () => {
    setEditingChannel(null);
    setChannelName('');
    setChannelDescription('');
    setChannelType('custom');
    setChannelToken('');
    setChannelUrl('');
    setChannelOther(
      JSON.stringify(
        {
          title: '$title',
          description: '$description',
          content: '$content',
          url: '$url',
          to: '$to',
        },
        null,
        2,
      ),
    );
    setChannelDialogOpen(true);
  };

  const handleEditChannelClick = (channel: PushChannel) => {
    setEditingChannel(channel);
    setChannelName(channel.name);
    setChannelDescription(channel.description ?? '');
    setChannelType(channel.type);
    setChannelToken(channel.token ?? '');
    setChannelUrl(channel.url);
    setChannelOther(channel.other);
    setChannelDialogOpen(true);
  };

  const handleSaveChannel = () => {
    if (!channelName && !editingChannel) {
      toast.error(t('channelNameRequired'));
      return;
    }
    if (!/^[a-zA-Z_0-9]+$/.test(channelName)) {
      toast.error(t('channelNameFormat'));
      return;
    }

    if (!activeDef) {
      toast.error(t('invalidChannelType'));
      return;
    }

    // 动态字段必填性校验
    for (const field of activeDef.fields) {
      const value =
        field.key === 'url'
          ? channelUrl
          : field.key === 'token'
            ? channelToken
            : channelOther;
      if (field.required && !value.trim()) {
        toast.error(t('fieldRequired', { label: field.label }));
        return;
      }
    }

    // 协议安全校验（非邮件服务且配置了地址时，强制 HTTPS 协议）
    if (channelType !== 'email') {
      if (channelUrl && !channelUrl.startsWith('https://')) {
        toast.error(t('httpsRequired'));
        return;
      }
    }

    // JSON 结构格式校验
    if (channelType === 'custom') {
      try {
        JSON.parse(channelOther);
      } catch {
        toast.error(t('jsonFormatRequired'));
        return;
      }
    } else if (channelType === 'lark' && channelOther) {
      try {
        JSON.parse(channelOther);
      } catch {
        toast.error(t('larkTemplateFormat'));
        return;
      }
    }

    if (editingChannel) {
      updateChannelMutation.mutate({
        id: editingChannel.id,
        data: {
          description: channelDescription,
          type: channelType,
          token: channelToken || undefined,
          url: channelUrl,
          other: channelOther,
          enabled: editingChannel.enabled,
        },
      });
    } else {
      createChannelMutation.mutate({
        name: channelName,
        description: channelDescription,
        type: channelType,
        token: channelToken || undefined,
        url: channelUrl,
        other: channelOther,
        enabled: true,
      });
    }
  };

  const handleTestChannelClick = (name: string) => {
    setTestChannelName(name);
    setTestChannelTarget('');
    setTestChannelOpen(true);
  };

  const handleSendChannelTest = async () => {
    try {
      setIsTestingChannel(true);
      toast.info(t('sendingTestPush'));
      await PushService.testChannel({
        name: testChannelName,
        target: testChannelTarget || undefined,
      });
      toast.success(t('testPushSuccess'));
      setTestChannelOpen(false);
    } catch (err: unknown) {
      toast.error(t('connectivityTestFailed') + ': ' + (err as Error).message);
    } finally {
      setIsTestingChannel(false);
    }
  };

  return (
    <div className='pt-4 space-y-4'>
      <div className='flex items-center justify-between'>
        <div>
          <h2 className='text-sm font-semibold'>{t('customPushChannels')}</h2>
          <p className='text-[11px] text-muted-foreground mt-0.5'>
            {t('customPushChannelsDesc')}
          </p>
        </div>
        <Button
          size='sm'
          onClick={handleCreateChannelClick}
          className='text-xs'
        >
          <Plus className='size-3.5 mr-1' />
          {t('createChannel')}
        </Button>
      </div>

      {channelsQuery.isLoading ? (
        <LoadingStateWithBorder
          icon={Settings}
          description={t('loadingChannels')}
        />
      ) : channelsQuery.isError ? (
        <div className='p-8 border border-dashed rounded-xl bg-card'>
          <ErrorInline
            error={channelsQuery.error}
            onRetry={() => channelsQuery.refetch()}
            className='justify-center'
          />
        </div>
      ) : (channelsQuery.data ?? []).length === 0 ? (
        <div className='py-12 border border-dashed rounded-lg flex flex-col items-center justify-center text-muted-foreground'>
          <Settings
            className='size-8 mb-2 opacity-30 animate-spin'
            style={{ animationDuration: '3s' }}
          />
          <span className='text-xs font-medium'>{t('noCustomChannels')}</span>
        </div>
      ) : (
        <div className='border border-dashed shadow-none rounded-lg overflow-hidden'>
          <Table className='w-full caption-bottom text-sm min-w-full'>
            <TableHeader className='sticky top-0 z-20 bg-background'>
              <TableRow className='border-b border-dashed hover:bg-transparent'>
                <TableHead className='w-[120px] whitespace-nowrap py-2 h-8'>
                  {t('colName')}
                </TableHead>
                <TableHead className='w-[100px] whitespace-nowrap py-2 h-8'>
                  {t('colType')}
                </TableHead>
                <TableHead className='whitespace-nowrap py-2 h-8'>
                  {t('colRemark')}
                </TableHead>
                <TableHead className='w-[80px] text-center whitespace-nowrap py-2 h-8'>
                  {t('colStatus')}
                </TableHead>
                <TableHead className='sticky right-0 text-center bg-background z-10 w-[180px] py-2 h-8'>
                  {t('colActions')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(channelsQuery.data ?? []).map((ch) => (
                <TableRow
                  key={ch.id}
                  className='border-dashed hover:bg-muted/30 cursor-pointer group'
                  onClick={() => handleEditChannelClick(ch)}
                >
                  <TableCell className='text-xs font-mono font-bold py-1'>
                    {ch.name}
                  </TableCell>
                  <TableCell className='py-1'>
                    <Badge
                      variant='outline'
                      className='text-[10px] py-0 px-1.5 h-4.5 whitespace-nowrap'
                    >
                      {(definitionsQuery.data ?? []).find(
                        (d) => d.type === ch.type,
                      )?.name ?? ch.type}
                    </Badge>
                  </TableCell>
                  <TableCell className='text-xs text-muted-foreground max-w-[200px] truncate py-1'>
                    {ch.description || (
                      <span className='italic'>{t('noRemark')}</span>
                    )}
                  </TableCell>
                  <TableCell
                    className='text-center py-1'
                    onClick={(e) => e.stopPropagation()}
                  >
                    <Switch
                      checked={ch.enabled}
                      onCheckedChange={(checked) => {
                        updateChannelMutation.mutate({
                          id: ch.id,
                          data: {
                            description: ch.description,
                            type: ch.type,
                            token: ch.token,
                            url: ch.url,
                            other: ch.other,
                            enabled: checked,
                          },
                        });
                      }}
                      className='scale-75'
                    />
                  </TableCell>
                  <TableCell
                    className='sticky right-0 text-center bg-background z-10 py-1'
                    onClick={(e) => e.stopPropagation()}
                  >
                    <div className='flex items-center justify-center gap-1'>
                      <Button
                        variant='ghost'
                        size='sm'
                        onClick={() => handleTestChannelClick(ch.name)}
                        className='h-6 px-2 text-[10px] text-primary hover:text-primary hover:bg-primary/10'
                      >
                        <Play className='size-2.5 mr-1' />
                        {t('test')}
                      </Button>
                      <Button
                        variant='ghost'
                        size='sm'
                        onClick={() => handleEditChannelClick(ch)}
                        className='h-6 px-2 text-[10px] text-muted-foreground hover:text-foreground'
                      >
                        <Edit2 className='size-2.5 mr-1' />
                        {t('edit')}
                      </Button>
                      <Button
                        variant='ghost'
                        size='sm'
                        disabled={deleteChannelMutation.isPending}
                        onClick={() => setDeleteTarget(ch)}
                        className='h-6 px-2 text-[10px] text-destructive hover:text-destructive hover:bg-destructive/10'
                      >
                        <Trash2 className='size-2.5 mr-1' />
                        {t('delete')}
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {/* ==================== 对话框：新增/编辑消息通道 ==================== */}
      <Dialog open={channelDialogOpen} onOpenChange={setChannelDialogOpen}>
        <DialogContent className='sm:max-w-[600px] max-h-[85vh] overflow-y-auto'>
          <DialogHeader>
            <DialogTitle>
              {editingChannel
                ? t('editChannelDialogTitle')
                : t('createChannelDialogTitle')}
            </DialogTitle>
            <DialogDescription>{t('channelDialogDesc')}</DialogDescription>
          </DialogHeader>

          <div className='space-y-4 py-4'>
            <div className='space-y-1.5'>
              <Label className='text-xs font-semibold'>
                {t('channelName')}
              </Label>
              <Input
                type='text'
                placeholder={t('channelNamePlaceholder')}
                value={channelName}
                onChange={(e) => setChannelName(e.target.value)}
                disabled={!!editingChannel}
                className='text-xs h-9 font-mono'
              />
              <p className='text-[10px] text-muted-foreground'>
                {t('channelNameHint')}
              </p>
            </div>

            <div className='space-y-1.5'>
              <Label className='text-xs font-semibold'>
                {t('channelRemark')}
              </Label>
              <Input
                type='text'
                placeholder={t('channelRemarkPlaceholder')}
                value={channelDescription}
                onChange={(e) => setChannelDescription(e.target.value)}
                className='text-xs h-9'
              />
            </div>

            <div className='space-y-1.5'>
              <Label className='text-xs font-semibold'>
                {t('channelType')}
              </Label>
              <Select
                value={channelType}
                onValueChange={handleChannelTypeChange}
              >
                <SelectTrigger className='text-xs h-9'>
                  <SelectValue placeholder={t('selectChannelType')} />
                </SelectTrigger>
                <SelectContent>
                  {(definitionsQuery.data ?? []).map((d) => (
                    <SelectItem key={d.type} value={d.type} className='text-xs'>
                      {d.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {activeDef && (
              <>
                <div className='p-3.5 border rounded-lg bg-muted/20 space-y-1.5'>
                  <div className='text-xs font-semibold'>
                    {t('configHelp', { name: activeDef.name })}
                  </div>
                  <p className='text-[11px] text-muted-foreground leading-relaxed'>
                    {activeDef.description}
                  </p>
                </div>

                {activeDef.fields.map((field) => {
                  const value =
                    field.key === 'url'
                      ? channelUrl
                      : field.key === 'token'
                        ? channelToken
                        : channelOther;
                  const onChange = (val: string) => {
                    if (field.key === 'url') setChannelUrl(val);
                    else if (field.key === 'token') setChannelToken(val);
                    else setChannelOther(val);
                  };

                  return (
                    <div key={field.key} className='space-y-1.5'>
                      <Label className='text-xs font-semibold'>
                        {field.label}
                        {field.required && (
                          <span className='text-destructive ml-0.5'>*</span>
                        )}
                      </Label>

                      {field.type === 'textarea' ? (
                        <Textarea
                          placeholder={field.placeholder}
                          value={value}
                          onChange={(e) => onChange(e.target.value)}
                          rows={
                            field.key === 'other' && channelType === 'custom'
                              ? 6
                              : 4
                          }
                          className='text-xs font-mono'
                        />
                      ) : (
                        <Input
                          type={field.type}
                          placeholder={field.placeholder}
                          value={value}
                          onChange={(e) => onChange(e.target.value)}
                          className='text-xs h-9 font-mono'
                        />
                      )}
                      {field.description && (
                        <p className='text-[10px] text-muted-foreground'>
                          {field.description}
                        </p>
                      )}
                    </div>
                  );
                })}

                {/* Custom post helper templates card */}
                {channelType === 'custom' && (
                  <div className='p-3.5 border rounded-lg bg-muted/20 space-y-2.5'>
                    <Label className='text-[11px] font-semibold'>
                      {t('quickLoadTemplates')}
                    </Label>
                    <div className='flex flex-wrap gap-1.5'>
                      <Button
                        variant='outline'
                        size='sm'
                        className='h-7 text-[10px] px-2 py-0'
                        type='button'
                        onClick={() => {
                          setChannelUrl(
                            'https://open.feishu.cn/open-apis/bot/v2/hook/YOUR_TOKEN',
                          );
                          setChannelOther(
                            JSON.stringify(
                              {
                                msg_type: 'text',
                                content: {
                                  text: '$title\n$description\n$content\n$url',
                                },
                              },
                              null,
                              2,
                            ),
                          );
                          toast.success(t('loadedFeishuTemplate'));
                        }}
                      >
                        飞书 Webhook
                      </Button>
                      <Button
                        variant='outline'
                        size='sm'
                        className='h-7 text-[10px] px-2 py-0'
                        type='button'
                        onClick={() => {
                          setChannelUrl(
                            'https://oapi.dingtalk.com/robot/send?access_token=YOUR_TOKEN',
                          );
                          setChannelOther(
                            JSON.stringify(
                              {
                                msgtype: 'markdown',
                                markdown: {
                                  title: '$title',
                                  text: '### $title\n$content\n\n[查看详情]($url)',
                                },
                              },
                              null,
                              2,
                            ),
                          );
                          toast.success(t('loadedDingTalkTemplate'));
                        }}
                      >
                        钉钉群机器人
                      </Button>
                      <Button
                        variant='outline'
                        size='sm'
                        className='h-7 text-[10px] px-2 py-0'
                        type='button'
                        onClick={() => {
                          setChannelUrl(
                            'https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY',
                          );
                          setChannelOther(
                            JSON.stringify(
                              {
                                msgtype: 'markdown',
                                markdown: {
                                  content:
                                    '### $title\n$content\n\n[查看详情]($url)',
                                },
                              },
                              null,
                              2,
                            ),
                          );
                          toast.success(t('loadedWeChatTemplate'));
                        }}
                      >
                        企业微信群机器人
                      </Button>
                      <Button
                        variant='outline'
                        size='sm'
                        className='h-7 text-[10px] px-2 py-0'
                        type='button'
                        onClick={() => {
                          setChannelUrl('https://api.day.app/push');
                          setChannelOther(
                            JSON.stringify(
                              {
                                device_key: '$to',
                                title: '$title',
                                body: '$content',
                                url: '$url',
                              },
                              null,
                              2,
                            ),
                          );
                          toast.success(t('loadedBarkTemplate'));
                        }}
                      >
                        Bark App
                      </Button>
                    </div>
                  </div>
                )}
              </>
            )}
          </div>

          <DialogFooter>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setChannelDialogOpen(false)}
              className='h-9 text-xs'
            >
              {t('cancel')}
            </Button>
            <Button
              variant='default'
              size='sm'
              disabled={
                createChannelMutation.isPending ||
                updateChannelMutation.isPending
              }
              onClick={handleSaveChannel}
              className='h-9 px-5 text-xs'
            >
              {(createChannelMutation.isPending ||
                updateChannelMutation.isPending) && (
                <Loader2 className='size-3 animate-spin mr-1' />
              )}
              {t('confirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* ==================== 对话框：测试渠道连通性 ==================== */}
      <Dialog open={testChannelOpen} onOpenChange={setTestChannelOpen}>
        <DialogContent className='sm:max-w-[450px]'>
          <DialogHeader>
            <DialogTitle>{t('sendTestNotification')}</DialogTitle>
            <DialogDescription>
              {t('sendTestNotificationDesc')}
            </DialogDescription>
          </DialogHeader>

          <div className='space-y-4 py-3'>
            <div className='space-y-1.5'>
              <Label className='text-xs'>{t('pushChannelName')}</Label>
              <Input
                type='text'
                value={testChannelName}
                disabled
                className='text-xs h-9 bg-muted font-mono'
              />
            </div>
            <div className='space-y-1.5'>
              <Label className='text-xs'>{t('testPushTarget')}</Label>
              <Input
                type='text'
                placeholder={t('testPushTargetPlaceholder')}
                value={testChannelTarget}
                onChange={(e) => setTestChannelTarget(e.target.value)}
                className='text-xs h-9'
              />
            </div>
          </div>

          <DialogFooter>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setTestChannelOpen(false)}
              className='h-9 text-xs'
            >
              {t('cancel')}
            </Button>
            <Button
              variant='default'
              size='sm'
              disabled={isTestingChannel}
              onClick={handleSendChannelTest}
              className='h-9 px-5 text-xs'
            >
              {isTestingChannel && (
                <Loader2 className='size-3 animate-spin mr-1' />
              )}
              {t('sendTest')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('deleteChannelTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('deleteChannelConfirm', { name: deleteTarget?.name ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteChannelMutation.isPending}>
              {t('cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={deleteChannelMutation.isPending}
              onClick={() =>
                deleteTarget && deleteChannelMutation.mutate(deleteTarget.id)
              }
            >
              {deleteChannelMutation.isPending
                ? t('deleting')
                : t('confirmDelete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
