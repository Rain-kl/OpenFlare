// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

"use client"

import * as React from "react"
import {Code} from "lucide-react"
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card"
import {Label} from "@/components/ui/label"
import {Input} from "@/components/ui/input"
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from "@/components/ui/select"
import {Checkbox} from "@/components/ui/checkbox"
import {Button} from "@/components/ui/button"

export function ControlsTab() {
  const [formData, setFormData] = React.useState({
    name: "",
    category: "database",
    allowNotify: true,
    channel: "all",
  })

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
      {/* 控件展示 */}
      <Card className="border-dashed shadow-none">
        <CardHeader className="pb-3">
          <CardTitle className="text-sm font-semibold">表单与输入框交互演示</CardTitle>
          <CardDescription>包含按钮变体、下拉选择、多选框及基础输入框的标准样式</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* 输入框 */}
          <div className="space-y-1.5">
            <Label htmlFor="task-name" className="text-xs font-medium text-foreground">任务名称</Label>
            <Input
              id="task-name"
              value={formData.name}
              onChange={(e) => setFormData(prev => ({ ...prev, name: e.target.value }))}
              placeholder="输入要创建的任务描述..."
              className="h-8 text-xs shadow-none bg-background focus-visible:ring-1"
            />
            <p className="text-[10px] text-muted-foreground">任务名称将用于后台执行记录的可读性显示</p>
          </div>

          {/* 下拉框 */}
          <div className="space-y-1.5">
            <Label className="text-xs font-medium text-foreground">任务分类</Label>
            <Select
              value={formData.category}
              onValueChange={(val) => setFormData(prev => ({ ...prev, category: val }))}
            >
              <SelectTrigger className="h-8 text-xs shadow-none bg-background focus:ring-1">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="database">数据库运维</SelectItem>
                <SelectItem value="sync">数据批量同步</SelectItem>
                <SelectItem value="cleanup">过期磁盘清理</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {/* 多选框 Checkbox */}
          <div className="flex items-start gap-2.5 p-3 bg-muted/30 border border-dashed rounded-lg">
            <Checkbox
              id="allow-notify"
              checked={formData.allowNotify}
              onCheckedChange={(checked) => setFormData(prev => ({ ...prev, allowNotify: checked === true }))}
              className="mt-0.5"
            />
            <div className="grid gap-1">
              <label
                htmlFor="allow-notify"
                className="text-xs font-medium text-foreground cursor-pointer select-none leading-none"
              >
                允许触发异步通知推送
              </label>
              <p className="text-[10px] text-muted-foreground leading-normal">
                勾选后，在当前后台任务顺利执行结束时，会关联派发多渠道通知推送。
              </p>
            </div>
          </div>

          {/* 按钮变体 */}
          <div className="pt-2">
            <div className="text-[10px] font-semibold text-muted-foreground uppercase tracking-wider mb-2">按钮样式对齐：</div>
            <div className="flex flex-wrap items-center gap-2">
              <Button size="sm" className="h-8 text-xs shadow-none">
                主要操作 (Primary)
              </Button>
              <Button variant="secondary" size="sm" className="h-8 text-xs shadow-none">
                次要操作 (Secondary)
              </Button>
              <Button variant="outline" size="sm" className="h-8 border-dashed text-xs shadow-none">
                辅助线框 (Outline)
              </Button>
              <Button variant="ghost" size="sm" className="h-8 text-xs hover:bg-muted">
                幽灵按钮 (Ghost)
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* 控件设计规范说明 */}
      <Card className="border-dashed shadow-none bg-muted/20">
        <CardHeader className="pb-3">
          <CardTitle className="text-sm font-semibold flex items-center gap-1.5">
            <Code className="size-4 text-primary" />
            表单设计规范
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4 text-xs text-muted-foreground">
          <div className="space-y-2 leading-relaxed">
            <p className="font-semibold text-foreground text-[11px]">1. 统一控件高度与投影：</p>
            <p>
              输入框 (<code className="bg-muted px-1 py-0.5 rounded font-mono">Input</code>) 与下拉选择器 (<code className="bg-muted px-1 py-0.5 rounded font-mono">SelectTrigger</code>) 的高度应统一限制为小号高度 <code className="bg-muted px-1 py-0.5 rounded font-mono">h-8</code>，字体大小使用 <code className="bg-muted px-1 py-0.5 rounded font-mono">text-xs</code>，并使用 <code className="bg-muted px-1 py-0.5 rounded font-mono">shadow-none</code> 去除原生的重叠阴影。
            </p>

            <p className="font-semibold text-foreground text-[11px] pt-1">2. Checkbox 与文本的对齐：</p>
            <p>
              Checkbox 应在左侧垂直偏上对齐，使用容器 <code className="bg-muted px-1 py-0.5 rounded font-mono">flex items-start gap-2.5</code> 排列。Checkbox 组件本身添加 <code className="bg-muted px-1 py-0.5 rounded font-mono">mt-0.5</code>，从而在多行描述文本的情况下依然保持良好的顶对齐，避免出现居中对齐导致的凌乱感。
            </p>

            <p className="font-semibold text-foreground text-[11px] pt-1">3. 按钮使用策略：</p>
            <p>
              普通主表单及新增/保存按钮，一律使用默认 Primary 按钮或次要 <code className="bg-muted px-1 py-0.5 rounded font-mono">{'variant="secondary"'}</code> 按钮，小号高度统一采用 <code className="bg-muted px-1 py-0.5 rounded font-mono">h-8 text-xs</code>。线框按钮必须带有 <code className="bg-muted px-1 py-0.5 rounded font-mono">border-dashed shadow-none</code> 类。
            </p>
          </div>

          <div className="bg-background/80 p-3 rounded border border-border/40 font-mono text-[10px] space-y-2 overflow-x-auto">
            <div>{`// 1. Label 与 Input 对齐`}</div>
            <div>{`<div className="space-y-1.5">
  <Label htmlFor="id" className="text-xs font-medium text-foreground">标题</Label>
  <Input id="id" className="h-8 text-xs shadow-none" />
</div>`}</div>
            <div className="pt-2">{`// 2. Checkbox 顶对齐`}</div>
            <div>{`<div className="flex items-start gap-2.5">
  <Checkbox id="c" className="mt-0.5" />
  <div className="grid gap-1">
    <label htmlFor="c" className="text-xs font-medium leading-none">标题</label>
    <p className="text-[10px] text-muted-foreground">多行说明内容</p>
  </div>
</div>`}</div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
