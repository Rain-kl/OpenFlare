// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

"use client"

import * as React from "react"
import {useMutation, useQuery, useQueryClient} from "@tanstack/react-query"
import {toast} from "sonner"
import {ChevronDown, Edit2, Layers, Loader2, Plus, Trash2,} from "lucide-react"

import {Button} from "@/components/ui/button"
import {Input} from "@/components/ui/input"
import {Label} from "@/components/ui/label"
import {Textarea} from "@/components/ui/textarea"
import {Switch} from "@/components/ui/switch"
import {Badge} from "@/components/ui/badge"
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow} from "@/components/ui/table"
import {Tooltip, TooltipContent, TooltipProvider, TooltipTrigger} from "@/components/ui/tooltip"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue,} from "@/components/ui/select"
import {Checkbox} from "@/components/ui/checkbox"
import {Popover, PopoverContent, PopoverTrigger} from "@/components/ui/popover"

import {ErrorInline} from "@/components/layout/error"
import {LoadingStateWithBorder} from "@/components/layout/loading"
import {EmptyStateWithBorder} from "@/components/layout/empty"


import services from "@/lib/services"
import type {CreatePushEventRequest, PushEvent, UpdatePushEventRequest,} from "@/lib/services/push"
import {PushService} from "@/lib/services/push"

export function EventsTab() {
  const queryClient = useQueryClient()

  // --- 获取所有自定义消息通道 ---
  const channelsQuery = useQuery({
    queryKey: ["admin", "push-channels"],
    queryFn: () => PushService.listChannels(),
  })

  const availableChannels = React.useMemo(() => {
    const customChannels = (channelsQuery.data ?? [])
      .filter(c => c.enabled)
      .map(c => c.name)
    return ["email", ...customChannels]
  }, [channelsQuery.data])

  // --- 获取通知事件 ---
  const eventsQuery = useQuery({
    queryKey: ["admin", "push-events"],
    queryFn: () => PushService.listEvents(),
  })

  const builtInEventsQuery = useQuery({
    queryKey: ["admin", "push-builtin-events"],
    queryFn: () => PushService.listBuiltInEvents(),
  })

  // --- 获取所有系统可调度任务类型 ---
  const taskTypesQuery = useQuery({
    queryKey: ["admin", "task-types"],
    queryFn: () => services.adminTask.getTaskTypes(),
  })

  // --- 修改保存事件 Mutation ---
  const updateEventMutation = useMutation({
    mutationFn: ({ id, data }: { id: number; data: UpdatePushEventRequest }) => PushService.updateEvent(id, data),
    onSuccess: () => {
      toast.success("事件更新成功")
      queryClient.invalidateQueries({ queryKey: ["admin", "push-events"] })
      setEditEventOpen(false)
    },
    onError: (err: unknown) => {
      toast.error("事件更新失败: " + (err as Error).message)
    },
  })

  const toggleEventMutation = useMutation({
    mutationFn: (id: number) => PushService.toggleEvent(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin", "push-events"] })
    },
    onError: (err: unknown) => {
      toast.error("操作失败: " + (err as Error).message)
    },
  })

  const createEventMutation = useMutation({
    mutationFn: (data: CreatePushEventRequest) => PushService.createEvent(data),
    onSuccess: () => {
      toast.success("事件创建成功")
      queryClient.invalidateQueries({ queryKey: ["admin", "push-events"] })
      setCreateEventOpen(false)
      setNewEventKey("")
      setNewEventType("builtin")
      setNewEventTaskType("")
      setNewEventChannels([])
      setNewEventEnabled(true)
    },
    onError: (err: unknown) => {
      toast.error("事件创建失败: " + (err as Error).message)
    },
  })

  const deleteEventMutation = useMutation({
    mutationFn: (id: number) => PushService.deleteEvent(id),
    onSuccess: () => {
      toast.success("配置删除成功")
      queryClient.invalidateQueries({ queryKey: ["admin", "push-events"] })
    },
    onError: (err: unknown) => {
      toast.error("配置删除失败: " + (err as Error).message)
    },
  })

  // --- 事件编辑对话框状态 ---
  const [editEventOpen, setEditEventOpen] = React.useState(false)
  const [selectedEvent, setSelectedEvent] = React.useState<PushEvent | null>(null)
  const [eventChannels, setEventChannels] = React.useState<string[]>([])
  const [eventTargets, setEventTargets] = React.useState("")
  const [eventTemplate, setEventTemplate] = React.useState("")

  // 事件创建对话框状态
  const [createEventOpen, setCreateEventOpen] = React.useState(false)
  const [newEventType, setNewEventType] = React.useState<"builtin" | "task">("builtin")
  const [newEventKey, setNewEventKey] = React.useState("")
  const [newEventTaskType, setNewEventTaskType] = React.useState("")
  const [newEventChannels, setNewEventChannels] = React.useState<string[]>([])
  const [newEventTargets, setNewEventTargets] = React.useState("")
  const [newEventTemplate, setNewEventTemplate] = React.useState("")
  const [newEventEnabled, setNewEventEnabled] = React.useState(true)

  const availableBuiltInEvents = React.useMemo(() => {
    const configuredKeys = new Set((eventsQuery.data ?? []).map(e => e.event_key))
    return (builtInEventsQuery.data ?? []).filter(e => !configuredKeys.has(e.key))
  }, [builtInEventsQuery.data, eventsQuery.data])

  const handleEditEventClick = (event: PushEvent) => {
    setSelectedEvent(event)
    setEventChannels(event.channels)
    setEventTargets((event.targets ?? []).join(", "))
    setEventTemplate(event.template)
    setEditEventOpen(true)
  }

  const handleSaveEvent = () => {
    if (!selectedEvent) return

    try {
      JSON.parse(eventTemplate)
    } catch {
      toast.error("消息模板不是合法的 JSON 格式")
      return
    }

    const targets = eventTargets
      .split(",")
      .map(t => t.trim())
      .filter(t => t !== "")

    updateEventMutation.mutate({
      id: selectedEvent.id,
      data: {
        channels: eventChannels,
        targets,
        template: eventTemplate,
        enabled: selectedEvent.enabled,
      },
    })
  }

  const handleCreateEventClick = () => {
    setNewEventKey("")
    setNewEventType("builtin")
    setNewEventTaskType("")
    setNewEventChannels([])
    setNewEventTargets("")
    setNewEventTemplate("")
    setNewEventEnabled(true)
    setCreateEventOpen(true)
  }

  const handleNewEventKeyChange = (key: string) => {
    setNewEventKey(key)
    const ev = availableBuiltInEvents.find(e => e.key === key)
    if (ev) {
      setNewEventTemplate(JSON.stringify(ev.default_template, null, 2))
    } else {
      setNewEventTemplate("")
    }
  }

  const handleNewEventTaskTypeChange = (taskType: string) => {
    setNewEventTaskType(taskType)
    const taskMeta = (taskTypesQuery.data ?? []).find(t => t.asynq_task === taskType)
    if (taskMeta) {
      const defaultTemplate = {
        title: `任务完成: ${taskMeta.name}`,
        content: `异步任务 {{task_name}} 已完成。状态: {{task_status}}，耗时: {{task_duration}} ms。`,
        level: "INFO",
      }
      setNewEventTemplate(JSON.stringify(defaultTemplate, null, 2))
    } else {
      setNewEventTemplate("")
    }
  }

  const handleCreateEvent = () => {
    if (newEventType === "builtin" && !newEventKey) {
      toast.error("请选择系统事件")
      return
    }
    if (newEventType === "task" && !newEventTaskType) {
      toast.error("请选择异步任务")
      return
    }

    if (newEventTemplate) {
      try {
        JSON.parse(newEventTemplate)
      } catch {
        toast.error("内容渲染模板不是合法的 JSON 格式")
        return
      }
    }

    const targets = newEventTargets
      .split(",")
      .map(t => t.trim())
      .filter(t => t !== "")

    createEventMutation.mutate({
      event_key: newEventType === "builtin" ? newEventKey : undefined,
      task_type: newEventType === "task" ? newEventTaskType : undefined,
      channels: newEventChannels,
      targets: targets.length > 0 ? targets : undefined,
      template: newEventTemplate || undefined,
      enabled: newEventEnabled,
    })
  }

  return (
    <div className="pt-4 space-y-4">
      <div className="flex justify-end">
        <Button size="sm" onClick={handleCreateEventClick} className="text-xs">
          <Plus className="size-3.5 mr-1" />
          新增通知事件
        </Button>
      </div>
      {eventsQuery.isLoading ? (
        <LoadingStateWithBorder icon={Layers} description="加载通知事件中..." />
      ) : eventsQuery.isError ? (
        <div className="p-8 border border-dashed rounded-xl bg-card">
          <ErrorInline error={eventsQuery.error} onRetry={() => eventsQuery.refetch()} className="justify-center" />
        </div>
      ) : (eventsQuery.data ?? []).length === 0 ? (
        <EmptyStateWithBorder icon={Layers} description="暂无通知配置事件，请点击右上角新增" />
      ) : (
        <div className="border border-dashed shadow-none rounded-lg overflow-hidden">
          <Table className="w-full caption-bottom text-sm min-w-full">
            <TableHeader className="sticky top-0 z-20 bg-background">
              <TableRow className="border-b border-dashed hover:bg-transparent">
                <TableHead className="w-[80px] whitespace-nowrap py-2 h-8">ID</TableHead>
                <TableHead className="w-[180px] whitespace-nowrap py-2 h-8">通知事件</TableHead>
                <TableHead className="w-[200px] whitespace-nowrap py-2 h-8">关联渠道</TableHead>
                <TableHead className="whitespace-nowrap py-2 h-8">推送目标</TableHead>
                <TableHead className="w-[80px] text-center whitespace-nowrap py-2 h-8">状态</TableHead>
                <TableHead className="sticky right-0 text-center bg-background z-10 w-[110px] py-2 h-8">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(eventsQuery.data ?? []).map((event) => (
                <TableRow
                  key={event.id}
                  className="border-dashed hover:bg-muted/30 cursor-pointer group"
                  onClick={() => handleEditEventClick(event)}
                >
                  <TableCell className="font-mono text-[11px] text-muted-foreground py-1">{event.id}</TableCell>
                  <TableCell className="py-1">
                    <div className="flex flex-col gap-0.5">
                      <div className="flex items-center gap-1.5">
                        <span className="font-medium text-[11px] leading-tight" title={event.name}>{event.name}</span>
                        {event.task_type && (
                          <Badge variant="outline" className="text-[8px] h-3.5 px-1 bg-blue-50/50 text-blue-600 border-blue-200">
                            任务
                          </Badge>
                        )}
                      </div>
                      <span className="text-[10px] text-muted-foreground font-mono leading-tight">{event.event_key}</span>
                    </div>
                  </TableCell>
                  <TableCell className="py-1">
                    <div className="flex flex-wrap gap-1">
                      {(event.channels ?? []).length === 0 ? (
                        <span className="text-xs text-muted-foreground italic">未指定渠道</span>
                      ) : (
                        event.channels.map(ch => (
                          <Badge key={ch} variant="secondary" className="text-[10px] py-0 px-1.5 h-4.5">
                            {ch === "email" ? "邮件" : ch}
                          </Badge>
                        ))
                      )}
                    </div>
                  </TableCell>
                  <TableCell className="py-1">
                    <div className="flex flex-wrap gap-1">
                      {(!event.targets || event.targets.length === 0) ? (
                        <span className="text-muted-foreground text-[10px] font-mono">-</span>
                      ) : (
                        event.targets.map(t => (
                          <Badge key={t} variant="outline" className="text-[10px] max-w-[150px] truncate py-0 px-1.5 h-4.5">
                            {t}
                          </Badge>
                        ))
                      )}
                    </div>
                  </TableCell>
                  <TableCell className="text-center py-1" onClick={(e) => e.stopPropagation()}>
                    <Switch
                      checked={event.enabled}
                      onCheckedChange={() => toggleEventMutation.mutate(event.id)}
                      className="scale-75 data-[state=checked]:bg-green-600 h-4 w-7"
                    />
                  </TableCell>
                  <TableCell className="sticky right-0 text-center bg-background z-10 py-1" onClick={(e) => e.stopPropagation()}>
                    <div className="flex items-center justify-center gap-0.5">
                      <TooltipProvider delayDuration={0}>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              variant="ghost"
                              size="icon"
                              className="h-6 w-6 text-muted-foreground hover:text-foreground"
                              onClick={() => handleEditEventClick(event)}
                            >
                              <Edit2 className="size-3" />
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent side="top" className="text-xs">
                            配置
                          </TooltipContent>
                        </Tooltip>
                      </TooltipProvider>

                      <TooltipProvider delayDuration={0}>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              variant="ghost"
                              size="icon"
                              className="h-6 w-6 text-muted-foreground hover:text-destructive hover:bg-destructive/10"
                              disabled={deleteEventMutation.isPending}
                              onClick={() => {
                                if (confirm("确定要删除该通知事件配置吗？")) {
                                  deleteEventMutation.mutate(event.id)
                                }
                              }}
                            >
                              <Trash2 className="size-3" />
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent side="top" className="text-xs">
                            删除
                          </TooltipContent>
                        </Tooltip>
                      </TooltipProvider>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {/* ==================== 对话框：新增事件 ==================== */}
      <Dialog open={createEventOpen} onOpenChange={setCreateEventOpen}>
        <DialogContent className="sm:max-w-[550px] max-h-[85vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>新增通知事件</DialogTitle>
            <DialogDescription>
              选择系统内置的事件并绑定推送渠道，创建后即可开始生效
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-4">
            <div className="space-y-1.5">
              <Label className="text-xs font-semibold">事件类型</Label>
              <div className="flex gap-4 p-1.5 border rounded-md bg-muted/20">
                <label className="flex items-center gap-1.5 text-xs cursor-pointer font-medium">
                  <input
                    type="radio"
                    name="eventType"
                    checked={newEventType === "builtin"}
                    onChange={() => {
                      setNewEventType("builtin")
                      setNewEventTaskType("")
                      setNewEventTemplate("")
                    }}
                    className="scale-90"
                  />
                  <span>系统内置事件</span>
                </label>
                <label className="flex items-center gap-1.5 text-xs cursor-pointer font-medium">
                  <input
                    type="radio"
                    name="eventType"
                    checked={newEventType === "task"}
                    onChange={() => {
                      setNewEventType("task")
                      setNewEventKey("")
                      setNewEventTemplate("")
                    }}
                    className="scale-90"
                  />
                  <span>任务完成事件</span>
                </label>
              </div>
            </div>

            {newEventType === "builtin" ? (
              <div className="space-y-1.5">
                <Label className="text-xs font-semibold">系统事件</Label>
                {builtInEventsQuery.isLoading ? (
                  <div className="flex items-center gap-2 text-xs text-muted-foreground">
                    <Loader2 className="size-3.5 animate-spin" />
                    <span>加载系统事件中...</span>
                  </div>
                ) : availableBuiltInEvents.length === 0 ? (
                  <div className="text-xs text-muted-foreground italic border p-2.5 rounded bg-muted/20">
                    所有内置事件都已配置，没有可新增的事件。
                  </div>
                ) : (
                  <Select value={newEventKey} onValueChange={handleNewEventKeyChange}>
                    <SelectTrigger className="text-xs h-9">
                      <SelectValue placeholder="请选择系统事件" />
                    </SelectTrigger>
                    <SelectContent>
                      {availableBuiltInEvents.map(ev => (
                        <SelectItem key={ev.key} value={ev.key} className="text-xs">
                          {ev.name} ({ev.key})
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              </div>
            ) : (
              <div className="space-y-1.5">
                <Label className="text-xs font-semibold">系统异步任务</Label>
                {taskTypesQuery.isLoading ? (
                  <div className="flex items-center gap-2 text-xs text-muted-foreground">
                    <Loader2 className="size-3.5 animate-spin" />
                    <span>加载异步任务中...</span>
                  </div>
                ) : (taskTypesQuery.data ?? []).length === 0 ? (
                  <div className="text-xs text-muted-foreground italic border p-2.5 rounded bg-muted/20">
                    暂无可用的系统任务。
                  </div>
                ) : (
                  <Select value={newEventTaskType} onValueChange={handleNewEventTaskTypeChange}>
                    <SelectTrigger className="text-xs h-9">
                      <SelectValue placeholder="请选择异步任务" />
                    </SelectTrigger>
                    <SelectContent>
                      {(taskTypesQuery.data ?? []).map(taskMeta => (
                        <SelectItem key={taskMeta.asynq_task} value={taskMeta.asynq_task} className="text-xs">
                          {taskMeta.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              </div>
            )}

            {newEventType === "builtin" && newEventKey && (
              <div className="text-[11px] bg-muted/30 p-2.5 rounded border text-muted-foreground space-y-1">
                <span className="font-semibold text-foreground">事件说明：</span>
                <span>
                  {availableBuiltInEvents.find(e => e.key === newEventKey)?.description || "无描述信息"}
                </span>
              </div>
            )}

            {newEventType === "task" && newEventTaskType && (
              <div className="text-[11px] bg-muted/30 p-2.5 rounded border text-muted-foreground space-y-1">
                <span className="font-semibold text-foreground">任务说明：</span>
                <span>
                  {(taskTypesQuery.data ?? []).find(t => t.asynq_task === newEventTaskType)?.description || "无描述信息"}
                </span>
              </div>
            )}

            <div className="space-y-1.5">
              <Label className="text-xs font-semibold">推送渠道 (可多选)</Label>
              <Popover>
                <PopoverTrigger asChild>
                  <Button variant="outline" className="w-full justify-between text-xs h-9 font-normal">
                    {newEventChannels.length > 0
                      ? newEventChannels
                          .map(ch => {
                            if (ch === "email") return "邮件"
                            return ch
                          })
                          .join(", ")
                      : "选择已配置推送渠道"}
                    <ChevronDown className="ml-2 size-4 shrink-0 opacity-50" />
                  </Button>
                </PopoverTrigger>
                <PopoverContent className="w-[var(--radix-popover-trigger-width)] p-3" align="start">
                  <div className="space-y-2 max-h-[200px] overflow-y-auto">
                    {availableChannels.map(ch => (
                      <label key={ch} className="flex items-center gap-2 text-xs font-medium cursor-pointer p-1.5 hover:bg-muted rounded transition-colors">
                        <Checkbox
                          checked={newEventChannels.includes(ch)}
                          onCheckedChange={checked => {
                            if (checked) {
                              setNewEventChannels([...newEventChannels, ch])
                            } else {
                              setNewEventChannels(newEventChannels.filter(c => c !== ch))
                            }
                          }}
                        />
                        <span>
                          {ch === "email" ? "邮件推送" : ch}
                        </span>
                      </label>
                    ))}
                    {availableChannels.length === 0 && (
                      <div className="text-[11px] text-muted-foreground italic p-1">
                        暂无可用渠道，请先在“通道管理与设置”中创建或启用。
                      </div>
                    )}
                  </div>
                </PopoverContent>
              </Popover>
            </div>

            <div className="space-y-1.5">
              <Label className="text-xs font-semibold">推送目标 (仅邮件等渠道需要)</Label>
              <Input
                type="text"
                placeholder="多个目标用英文逗号分隔，例如：user1@test.com, user2@test.com"
                value={newEventTargets}
                onChange={e => setNewEventTargets(e.target.value)}
                className="text-xs h-9"
              />
            </div>

            <div className="space-y-1.5">
              <div className="flex justify-between items-center">
                <Label className="text-xs font-semibold">内容渲染模板 (JSON 格式)</Label>
                <span className="text-[10px] text-muted-foreground font-mono flex items-center">
                  {newEventType === "task"
                    ? "支持变量：{{task_name}}, {{task_status}}, {{task_duration}}, {{user.username}}"
                    : "支持变量：{{user.username}}, {{ip}}, {{time}}"}
                </span>
              </div>
              <Textarea
                value={newEventTemplate}
                onChange={e => setNewEventTemplate(e.target.value)}
                rows={6}
                className="text-xs font-mono"
                placeholder='{"title": "管理员登录提醒", "content": "管理员 {{user.username}} ...", "level": "INFO"}'
              />
            </div>

            <div className="flex items-center justify-between p-3 border rounded-lg bg-muted/10">
              <div className="space-y-0.5">
                <Label className="text-xs font-semibold">启用状态</Label>
                <div className="text-[10px] text-muted-foreground">创建后是否立即开始接收此事件的通知</div>
              </div>
              <Switch
                checked={newEventEnabled}
                onCheckedChange={setNewEventEnabled}
              />
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" size="sm" onClick={() => setCreateEventOpen(false)} className="h-9 text-xs">
              取消
            </Button>
            <Button
              variant="default"
              size="sm"
              disabled={createEventMutation.isPending || (newEventType === "builtin" && availableBuiltInEvents.length === 0)}
              onClick={handleCreateEvent}
              className="h-9 px-5 text-xs"
            >
              {createEventMutation.isPending && <Loader2 className="size-3 animate-spin mr-1" />}
              启用保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* ==================== 对话框：编辑事件 ==================== */}
      <Dialog open={editEventOpen} onOpenChange={setEditEventOpen}>
        <DialogContent className="sm:max-w-[550px] max-h-[85vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>事件通知渠道配置</DialogTitle>
            <DialogDescription>
              自定义本事件触发时，需要异步推送的渠道以及接收人邮箱等配置
            </DialogDescription>
          </DialogHeader>

          {selectedEvent && (
            <div className="space-y-5 py-4">
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-1.5">
                  <Label className="text-xs font-semibold text-muted-foreground">事件名称</Label>
                  <Input value={selectedEvent.name} disabled className="text-xs h-9 bg-muted" />
                </div>
                <div className="space-y-1.5">
                  <Label className="text-xs font-semibold text-muted-foreground">事件键 (Key)</Label>
                  <Input value={selectedEvent.event_key} disabled className="text-xs h-9 font-mono bg-muted" />
                </div>
              </div>

              {selectedEvent.task_type && (
                <div className="space-y-1.5">
                  <Label className="text-xs font-semibold text-muted-foreground">关联异步任务</Label>
                  <Input
                    value={`${(taskTypesQuery.data ?? []).find(t => t.asynq_task === selectedEvent.task_type)?.name || selectedEvent.task_type} (${selectedEvent.task_type})`}
                    disabled
                    className="text-xs h-9 bg-muted"
                  />
                </div>
              )}

              <div className="space-y-1.5">
                <Label className="text-xs font-semibold">推送渠道 (可多选)</Label>
                <Popover>
                  <PopoverTrigger asChild>
                    <Button variant="outline" className="w-full justify-between text-xs h-9 font-normal">
                      {eventChannels.length > 0
                        ? eventChannels
                            .map(ch => {
                              if (ch === "email") return "邮件"
                              return ch
                            })
                            .join(", ")
                        : "选择已配置推送渠道"}
                      <ChevronDown className="ml-2 size-4 shrink-0 opacity-50" />
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent className="w-[var(--radix-popover-trigger-width)] p-3" align="start">
                    <div className="space-y-2 max-h-[200px] overflow-y-auto">
                      {availableChannels.map(ch => (
                        <label key={ch} className="flex items-center gap-2 text-xs font-medium cursor-pointer p-1.5 hover:bg-muted rounded transition-colors">
                          <Checkbox
                            checked={eventChannels.includes(ch)}
                            onCheckedChange={checked => {
                              if (checked) {
                                setEventChannels([...eventChannels, ch])
                              } else {
                                setEventChannels(eventChannels.filter(c => c !== ch))
                              }
                            }}
                          />
                          <span>
                            {ch === "email" ? "邮件推送" : ch}
                          </span>
                        </label>
                      ))}
                      {availableChannels.length === 0 && (
                        <div className="text-[11px] text-muted-foreground italic p-1">
                          暂无可用渠道，请先在“通道管理与设置”中创建并启用。
                        </div>
                      )}
                    </div>
                  </PopoverContent>
                </Popover>
              </div>

              <div className="space-y-1.5">
                <Label className="text-xs font-semibold">推送目标 (仅邮件等渠道需要)</Label>
                <Input
                  type="text"
                  placeholder="多个目标用英文逗号分隔，例如：user1@test.com, user2@test.com"
                  value={eventTargets}
                  onChange={e => setEventTargets(e.target.value)}
                  className="text-xs h-9"
                />
              </div>

              <div className="space-y-1.5">
                <div className="flex justify-between items-center">
                  <Label className="text-xs font-semibold">内容渲染模板 (JSON 格式)</Label>
                  <span className="text-[10px] text-muted-foreground font-mono flex items-center">
                    {selectedEvent.task_type
                      ? "支持变量：{{task_name}}, {{task_status}}, {{task_duration}}, {{user.username}}"
                      : "支持变量：{{user.username}}, {{ip}}, {{time}}"}
                  </span>
                </div>
                <Textarea
                  value={eventTemplate}
                  onChange={e => setEventTemplate(e.target.value)}
                  rows={6}
                  className="text-xs font-mono"
                  placeholder='{"title": "管理员登录提醒", "content": "管理员 {{user.username}} ...", "level": "INFO"}'
                />
              </div>
            </div>
          )}

          <DialogFooter>
            <Button variant="outline" size="sm" onClick={() => setEditEventOpen(false)} className="h-9 text-xs">
              取消
            </Button>
            <Button
              variant="default"
              size="sm"
              disabled={updateEventMutation.isPending}
              onClick={handleSaveEvent}
              className="h-9 px-5 text-xs"
            >
              {updateEventMutation.isPending && <Loader2 className="size-3 animate-spin mr-1" />}
              保存修改
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
