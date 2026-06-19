// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

"use client"

import * as React from "react"
import {useMutation, useQuery, useQueryClient} from "@tanstack/react-query"
import {toast} from "sonner"

import {Edit2, Loader2, Play, Plus, Settings, Trash2,} from "lucide-react"

import {Button} from "@/components/ui/button"
import {Input} from "@/components/ui/input"
import {Label} from "@/components/ui/label"
import {Textarea} from "@/components/ui/textarea"
import {Switch} from "@/components/ui/switch"
import {Badge} from "@/components/ui/badge"
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow,} from "@/components/ui/table"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue,} from "@/components/ui/select"

import {ErrorInline} from "@/components/layout/error"
import {LoadingStateWithBorder} from "@/components/layout/loading"

import type {ChannelDefinition, CreateChannelRequest, PushChannel, UpdateChannelRequest} from "@/lib/services/push"
import {PushService} from "@/lib/services/push"

export function SettingsTab() {
  const queryClient = useQueryClient()

  // --- 获取所有自定义消息通道 ---
  const channelsQuery = useQuery({
    queryKey: ["admin", "push-channels"],
    queryFn: () => PushService.listChannels(),
  })

  // --- 获取动态通道表单字段定义 ---
  const definitionsQuery = useQuery({
    queryKey: ["admin", "push-channels-definitions"],
    queryFn: () => PushService.listChannelDefinitions(),
  })


  // --- 消息通道 CRUD Mutations ---
  const createChannelMutation = useMutation({
    mutationFn: (data: CreateChannelRequest) => PushService.createChannel(data),
    onSuccess: () => {
      toast.success("通道创建成功")
      queryClient.invalidateQueries({ queryKey: ["admin", "push-channels"] })
      setChannelDialogOpen(false)
    },
    onError: (err: unknown) => {
      toast.error("通道创建失败: " + (err as Error).message)
    },
  })

  const updateChannelMutation = useMutation({
    mutationFn: ({ id, data }: { id: number; data: UpdateChannelRequest }) =>
      PushService.updateChannel(id, data),
    onSuccess: () => {
      toast.success("通道更新成功")
      queryClient.invalidateQueries({ queryKey: ["admin", "push-channels"] })
      setChannelDialogOpen(false)
    },
    onError: (err: unknown) => {
      toast.error("通道更新失败: " + (err as Error).message)
    },
  })

  const deleteChannelMutation = useMutation({
    mutationFn: (id: number) => PushService.deleteChannel(id),
    onSuccess: () => {
      toast.success("通道删除成功")
      queryClient.invalidateQueries({ queryKey: ["admin", "push-channels"] })
    },
    onError: (err: unknown) => {
      toast.error("通道删除失败: " + (err as Error).message)
    },
  })

  // --- 消息通道与设置相关 State ---
  const [channelDialogOpen, setChannelDialogOpen] = React.useState(false)
  const [editingChannel, setEditingChannel] = React.useState<PushChannel | null>(null)
  const [channelName, setChannelName] = React.useState("")
  const [channelDescription, setChannelDescription] = React.useState("")
  const [channelType, setChannelType] = React.useState("custom")
  const [channelToken, setChannelToken] = React.useState("")
  const [channelUrl, setChannelUrl] = React.useState("")
  const [channelOther, setChannelOther] = React.useState("")

  const activeDef = React.useMemo<ChannelDefinition | undefined>(() => {
    return (definitionsQuery.data ?? []).find(d => d.type === channelType)
  }, [definitionsQuery.data, channelType])

  const [testChannelOpen, setTestChannelOpen] = React.useState(false)
  const [testChannelName, setTestChannelName] = React.useState("")
  const [testChannelTarget, setTestChannelTarget] = React.useState("")
  const [isTestingChannel, setIsTestingChannel] = React.useState(false)

  const handleChannelTypeChange = (newType: string) => {
    setChannelType(newType)
    setChannelUrl("")
    setChannelToken("")
    if (newType === "custom") {
      setChannelOther(JSON.stringify({
        title: "$title",
        description: "$description",
        content: "$content",
        url: "$url",
        to: "$to"
      }, null, 2))
    } else {
      setChannelOther("")
    }
  }

  const handleCreateChannelClick = () => {
    setEditingChannel(null)
    setChannelName("")
    setChannelDescription("")
    setChannelType("custom")
    setChannelToken("")
    setChannelUrl("")
    setChannelOther(JSON.stringify({
      title: "$title",
      description: "$description",
      content: "$content",
      url: "$url",
      to: "$to"
    }, null, 2))
    setChannelDialogOpen(true)
  }

  const handleEditChannelClick = (channel: PushChannel) => {
    setEditingChannel(channel)
    setChannelName(channel.name)
    setChannelDescription(channel.description ?? "")
    setChannelType(channel.type)
    setChannelToken(channel.token ?? "")
    setChannelUrl(channel.url)
    setChannelOther(channel.other)
    setChannelDialogOpen(true)
  }

  const handleSaveChannel = () => {
    if (!channelName && !editingChannel) {
      toast.error("通道名称不能为空")
      return
    }
    if (!/^[a-zA-Z_0-9]+$/.test(channelName)) {
      toast.error("通道名称只能使用英文字母、数字和下划线")
      return
    }

    if (!activeDef) {
      toast.error("无效的通道类型")
      return
    }

    // 动态字段必填性校验
    for (const field of activeDef.fields) {
      const value = field.key === "url"
        ? channelUrl
        : field.key === "token"
        ? channelToken
        : channelOther;
      if (field.required && !value.trim()) {
        toast.error(`${field.label}不能为空`)
        return
      }
    }

    // 协议安全校验（非邮件服务且配置了地址时，强制 HTTPS 协议）
    if (channelType !== "email") {
      if (channelUrl && !channelUrl.startsWith("https://")) {
        toast.error("地址必须以 https:// 开头以确保安全性")
        return
      }
    }

    // JSON 结构格式校验
    if (channelType === "custom") {
      try {
        JSON.parse(channelOther)
      } catch {
        toast.error("请求体必须是合法的 JSON 格式")
        return
      }
    } else if (channelType === "lark" && channelOther) {
      try {
        JSON.parse(channelOther)
      } catch {
        toast.error("自定义卡片模版必须是合法的 JSON 格式")
        return
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
        }
      })
    } else {
      createChannelMutation.mutate({
        name: channelName,
        description: channelDescription,
        type: channelType,
        token: channelToken || undefined,
        url: channelUrl,
        other: channelOther,
        enabled: true,
      })
    }
  }

  const handleTestChannelClick = (name: string) => {
    setTestChannelName(name)
    setTestChannelTarget("")
    setTestChannelOpen(true)
  }

  const handleSendChannelTest = async () => {
    try {
      setIsTestingChannel(true)
      toast.info("正在发送测试推送...")
      await PushService.testChannel({
        name: testChannelName,
        target: testChannelTarget || undefined,
      })
      toast.success("测试推送发送成功，请前往对应平台确认。")
      setTestChannelOpen(false)
    } catch (err: unknown) {
      toast.error("连通性测试失败: " + (err as Error).message)
    } finally {
      setIsTestingChannel(false)
    }
  }



  return (
    <div className="pt-4 space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-sm font-semibold">自定义推送通道</h2>
          <p className="text-[11px] text-muted-foreground mt-0.5">
            添加、配置及管理用于第三方 Webhook 对接的自定义数据推送通道
          </p>
        </div>
        <Button size="sm" onClick={handleCreateChannelClick} className="text-xs">
          <Plus className="size-3.5 mr-1" />
          新建消息通道
        </Button>
      </div>

      {channelsQuery.isLoading ? (
        <LoadingStateWithBorder icon={Settings} description="加载消息通道中..." />
      ) : channelsQuery.isError ? (
        <div className="p-8 border border-dashed rounded-xl bg-card">
          <ErrorInline error={channelsQuery.error} onRetry={() => channelsQuery.refetch()} className="justify-center" />
        </div>
      ) : (channelsQuery.data ?? []).length === 0 ? (
        <div className="py-12 border border-dashed rounded-lg flex flex-col items-center justify-center text-muted-foreground">
          <Settings className="size-8 mb-2 opacity-30 animate-spin" style={{ animationDuration: '3s' }} />
          <span className="text-xs font-medium">暂无自定义通道配置，请点击右上角新建</span>
        </div>
      ) : (
        <div className="border border-dashed shadow-none rounded-lg overflow-hidden">
          <Table className="w-full caption-bottom text-sm min-w-full">
            <TableHeader className="sticky top-0 z-20 bg-background">
              <TableRow className="border-b border-dashed hover:bg-transparent">
                <TableHead className="w-[120px] whitespace-nowrap py-2 h-8">名称</TableHead>
                <TableHead className="w-[100px] whitespace-nowrap py-2 h-8">类型</TableHead>
                <TableHead className="whitespace-nowrap py-2 h-8">备注</TableHead>
                <TableHead className="w-[80px] text-center whitespace-nowrap py-2 h-8">状态</TableHead>
                <TableHead className="sticky right-0 text-center bg-background z-10 w-[180px] py-2 h-8">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(channelsQuery.data ?? []).map(ch => (
                <TableRow
                  key={ch.id}
                  className="border-dashed hover:bg-muted/30 cursor-pointer group"
                  onClick={() => handleEditChannelClick(ch)}
                >
                  <TableCell className="text-xs font-mono font-bold py-1">
                    {ch.name}
                  </TableCell>
                  <TableCell className="py-1">
                    <Badge variant="outline" className="text-[10px] py-0 px-1.5 h-4.5 whitespace-nowrap">
                      {(definitionsQuery.data ?? []).find(d => d.type === ch.type)?.name ?? ch.type}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground max-w-[200px] truncate py-1">
                    {ch.description || <span className="italic">无备注</span>}
                  </TableCell>
                  <TableCell className="text-center py-1" onClick={(e) => e.stopPropagation()}>
                    <Switch
                      checked={ch.enabled}
                      onCheckedChange={checked => {
                        updateChannelMutation.mutate({
                          id: ch.id,
                          data: {
                            description: ch.description,
                            type: ch.type,
                            token: ch.token,
                            url: ch.url,
                            other: ch.other,
                            enabled: checked,
                          }
                        })
                      }}
                      className="scale-75 data-[state=checked]:bg-green-600 h-4 w-7"
                    />
                  </TableCell>
                  <TableCell className="sticky right-0 text-center bg-background z-10 py-1" onClick={(e) => e.stopPropagation()}>
                    <div className="flex items-center justify-center gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleTestChannelClick(ch.name)}
                        className="h-6 px-2 text-[10px] text-primary hover:text-primary hover:bg-primary/10"
                      >
                        <Play className="size-2.5 mr-1" />
                        测试
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleEditChannelClick(ch)}
                        className="h-6 px-2 text-[10px] text-muted-foreground hover:text-foreground"
                      >
                        <Edit2 className="size-2.5 mr-1" />
                        编辑
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={deleteChannelMutation.isPending}
                        onClick={() => {
                          if (confirm(`确定要删除通道 "${ch.name}" 吗？`)) {
                            deleteChannelMutation.mutate(ch.id)
                          }
                        }}
                        className="h-6 px-2 text-[10px] text-destructive hover:text-destructive hover:bg-destructive/10"
                      >
                        <Trash2 className="size-2.5 mr-1" />
                        删除
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
        <DialogContent className="sm:max-w-[600px] max-h-[85vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{editingChannel ? "编辑消息通道" : "新建消息通道"}</DialogTitle>
            <DialogDescription>
              配置自定义通知推送通道，支持以 POST 请求方式向第三方 Webhook 或推送服务投递数据
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-4">
            <div className="space-y-1.5">
              <Label className="text-xs font-semibold">名称</Label>
              <Input
                type="text"
                placeholder="请输入通道名称，请仅使用英文字母和下划线，该名称必须唯一"
                value={channelName}
                onChange={e => setChannelName(e.target.value)}
                disabled={!!editingChannel}
                className="text-xs h-9 font-mono"
              />
              <p className="text-[10px] text-muted-foreground">通道唯一标识，创建后不可修改</p>
            </div>

            <div className="space-y-1.5">
              <Label className="text-xs font-semibold">备注</Label>
              <Input
                type="text"
                placeholder="请输入备注信息"
                value={channelDescription}
                onChange={e => setChannelDescription(e.target.value)}
                className="text-xs h-9"
              />
            </div>

             <div className="space-y-1.5">
              <Label className="text-xs font-semibold">通道类型</Label>
              <Select value={channelType} onValueChange={handleChannelTypeChange}>
                <SelectTrigger className="text-xs h-9">
                  <SelectValue placeholder="选择通道类型" />
                </SelectTrigger>
                <SelectContent>
                  {(definitionsQuery.data ?? []).map(d => (
                    <SelectItem key={d.type} value={d.type} className="text-xs">
                      {d.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {activeDef && (
              <>
                <div className="p-3.5 border rounded-lg bg-muted/20 space-y-1.5">
                  <div className="text-xs font-semibold">{activeDef.name}配置说明</div>
                  <p className="text-[11px] text-muted-foreground leading-relaxed">
                    {activeDef.description}
                  </p>
                </div>

                {activeDef.fields.map(field => {
                  const value = field.key === "url"
                    ? channelUrl
                    : field.key === "token"
                    ? channelToken
                    : channelOther;
                  const onChange = (val: string) => {
                    if (field.key === "url") setChannelUrl(val);
                    else if (field.key === "token") setChannelToken(val);
                    else setChannelOther(val);
                  };

                  return (
                    <div key={field.key} className="space-y-1.5">
                      <Label className="text-xs font-semibold">
                        {field.label}
                        {field.required && <span className="text-destructive ml-0.5">*</span>}
                      </Label>

                      {field.type === "textarea" ? (
                        <Textarea
                          placeholder={field.placeholder}
                          value={value}
                          onChange={e => onChange(e.target.value)}
                          rows={field.key === "other" && channelType === "custom" ? 6 : 4}
                          className="text-xs font-mono"
                        />
                      ) : (
                        <Input
                          type={field.type}
                          placeholder={field.placeholder}
                          value={value}
                          onChange={e => onChange(e.target.value)}
                          className="text-xs h-9 font-mono"
                        />
                      )}
                      {field.description && (
                        <p className="text-[10px] text-muted-foreground">
                          {field.description}
                        </p>
                      )}
                    </div>
                  );
                })}

                {/* Custom post helper templates card */}
                {channelType === "custom" && (
                  <div className="p-3.5 border rounded-lg bg-muted/20 space-y-2.5">
                    <Label className="text-[11px] font-semibold">快捷加载常用模版实例：</Label>
                    <div className="flex flex-wrap gap-1.5">
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-7 text-[10px] px-2 py-0"
                        type="button"
                        onClick={() => {
                          setChannelUrl("https://open.feishu.cn/open-apis/bot/v2/hook/YOUR_TOKEN");
                          setChannelOther(JSON.stringify({
                            msg_type: "text",
                            content: {
                              text: "$title\n$description\n$content\n$url"
                            }
                          }, null, 2));
                          toast.success("已加载飞书 Webhook 模版");
                        }}
                      >
                        飞书 Webhook
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-7 text-[10px] px-2 py-0"
                        type="button"
                        onClick={() => {
                          setChannelUrl("https://oapi.dingtalk.com/robot/send?access_token=YOUR_TOKEN");
                          setChannelOther(JSON.stringify({
                            msgtype: "markdown",
                            markdown: {
                              title: "$title",
                              text: "### $title\n$content\n\n[查看详情]($url)"
                            }
                          }, null, 2));
                          toast.success("已加载钉钉机器人模版");
                        }}
                      >
                        钉钉群机器人
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-7 text-[10px] px-2 py-0"
                        type="button"
                        onClick={() => {
                          setChannelUrl("https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=YOUR_KEY");
                          setChannelOther(JSON.stringify({
                            msgtype: "markdown",
                            markdown: {
                              content: "### $title\n$content\n\n[查看详情]($url)"
                            }
                          }, null, 2));
                          toast.success("已加载企业微信机器人模版");
                        }}
                      >
                        企业微信群机器人
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-7 text-[10px] px-2 py-0"
                        type="button"
                        onClick={() => {
                          setChannelUrl("https://api.day.app/push");
                          setChannelOther(JSON.stringify({
                            device_key: "$to",
                            title: "$title",
                            body: "$content",
                            url: "$url"
                          }, null, 2));
                          toast.success("已加载 Bark App 模版");
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
            <Button variant="outline" size="sm" onClick={() => setChannelDialogOpen(false)} className="h-9 text-xs">
              取消
            </Button>
            <Button
              variant="default"
              size="sm"
              disabled={createChannelMutation.isPending || updateChannelMutation.isPending}
              onClick={handleSaveChannel}
              className="h-9 px-5 text-xs"
            >
              {(createChannelMutation.isPending || updateChannelMutation.isPending) && <Loader2 className="size-3 animate-spin mr-1" />}
              确定
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* ==================== 对话框：测试渠道连通性 ==================== */}
      <Dialog open={testChannelOpen} onOpenChange={setTestChannelOpen}>
        <DialogContent className="sm:max-w-[450px]">
          <DialogHeader>
            <DialogTitle>发送测试通知</DialogTitle>
            <DialogDescription>
              请输入消息测试的接收目标，点击发送后系统将通过此通道执行连通性推送测试
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-3">
            <div className="space-y-1.5">
              <Label className="text-xs">推送通道名称</Label>
              <Input
                type="text"
                value={testChannelName}
                disabled
                className="text-xs h-9 bg-muted font-mono"
              />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs">测试推送目标 (对应模板变量 $to)</Label>
              <Input
                type="text"
                placeholder="请输入测试推送接收人/目标标识，如 Bark Token、邮箱等"
                value={testChannelTarget}
                onChange={e => setTestChannelTarget(e.target.value)}
                className="text-xs h-9"
              />
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" size="sm" onClick={() => setTestChannelOpen(false)} className="h-9 text-xs">
              取消
            </Button>
            <Button
              variant="default"
              size="sm"
              disabled={isTestingChannel}
              onClick={handleSendChannelTest}
              className="h-9 px-5 text-xs"
            >
              {isTestingChannel && <Loader2 className="size-3 animate-spin mr-1" />}
              发送测试
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
