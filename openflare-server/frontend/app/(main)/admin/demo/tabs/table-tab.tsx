// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

"use client"

import * as React from "react"
import {Code, Edit, ListFilter, Plus, Search, Trash2} from "lucide-react"
import {Button} from "@/components/ui/button"
import {Input} from "@/components/ui/input"
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow} from "@/components/ui/table"
import {Badge} from "@/components/ui/badge"
import {Card, CardContent, CardHeader, CardTitle} from "@/components/ui/card"

export function TableTab() {
  const mockTableData = [
    { id: "TX-1002", name: "用户信息批量同步导出", type: "数据同步", status: "success", duration: "1.2s", time: "2026-06-15 14:22" },
    { id: "TX-1003", name: "系统数据库每周自动备份归档", type: "数据库运维", status: "running", duration: "在执行中", time: "2026-06-15 15:00" },
    { id: "TX-1004", name: "远端S3过期缓存对象自动清理任务", type: "磁盘清理", status: "failed", duration: "0.4s", time: "2026-06-15 10:05" },
  ]

  return (
    <div className="space-y-6">
      {/* 表格控制工具栏 */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
        <div className="flex items-center gap-2">
          <div className="relative w-64">
            <Search className="absolute left-2.5 top-2.5 size-3 text-muted-foreground" />
            <Input placeholder="输入任务名称搜索..." className="h-8 pl-8 text-xs w-full shadow-none border-dashed bg-background" />
          </div>
          <Button variant="outline" size="sm" className="h-8 border-dashed text-xs shadow-none">
            <ListFilter className="size-3 mr-1" />
            过滤
          </Button>
        </div>
        <Button size="sm" className="h-8 text-xs shadow-none">
          <Plus className="size-3.5 mr-1" />
          新建任务
        </Button>
      </div>

      {/* 数据表格本身 */}
      <div className="border border-dashed shadow-none rounded-lg overflow-hidden bg-background">
        <Table className="w-full caption-bottom text-sm min-w-full">
          <TableHeader className="bg-muted/40">
            <TableRow className="border-dashed hover:bg-transparent">
              <TableHead className="w-[100px] text-xs font-semibold">任务编号</TableHead>
              <TableHead className="text-xs font-semibold">执行任务名称</TableHead>
              <TableHead className="w-[120px] text-xs font-semibold">任务类型</TableHead>
              <TableHead className="w-[100px] text-xs font-semibold">状态</TableHead>
              <TableHead className="w-[100px] text-xs font-semibold">执行耗时</TableHead>
              <TableHead className="w-[150px] text-xs font-semibold">触发时间</TableHead>
              <TableHead className="w-[80px] text-xs font-semibold text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {mockTableData.map((row) => (
              <TableRow key={row.id} className="border-dashed hover:bg-muted/10 transition-colors">
                <TableCell className="font-mono text-xs font-semibold">{row.id}</TableCell>
                <TableCell className="text-xs font-medium text-foreground">{row.name}</TableCell>
                <TableCell className="text-xs text-muted-foreground">{row.type}</TableCell>
                <TableCell>
                  {row.status === "success" && (
                    <Badge variant="outline" className="text-[10px] bg-emerald-500/10 border-emerald-500/20 text-emerald-600 rounded-full py-0 px-2 font-medium">
                      <span className="size-1 bg-emerald-500 rounded-full mr-1.5 shrink-0" />
                      成功
                    </Badge>
                  )}
                  {row.status === "running" && (
                    <Badge variant="outline" className="text-[10px] bg-blue-500/10 border-blue-500/20 text-blue-600 rounded-full py-0 px-2 font-medium">
                      <span className="size-1 bg-blue-500 rounded-full mr-1.5 shrink-0 animate-pulse" />
                      运行中
                    </Badge>
                  )}
                  {row.status === "failed" && (
                    <Badge variant="outline" className="text-[10px] bg-destructive/10 border-destructive/20 text-destructive rounded-full py-0 px-2 font-medium">
                      <span className="size-1 bg-destructive rounded-full mr-1.5 shrink-0" />
                      失败
                    </Badge>
                  )}
                </TableCell>
                <TableCell className="text-xs text-muted-foreground font-mono">{row.duration}</TableCell>
                <TableCell className="text-xs text-muted-foreground">{row.time}</TableCell>
                <TableCell className="text-right">
                  <div className="flex items-center justify-end gap-1.5">
                    <Button variant="ghost" size="icon" className="h-6 w-6 rounded hover:bg-muted text-muted-foreground">
                      <Edit className="size-3" />
                    </Button>
                    <Button variant="ghost" size="icon" className="h-6 w-6 rounded hover:bg-destructive/10 text-destructive">
                      <Trash2 className="size-3" />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {/* 表格规范指引 */}
      <Card className="border-dashed shadow-none bg-muted/20">
        <CardHeader className="pb-3">
          <CardTitle className="text-sm font-semibold flex items-center gap-1.5">
            <Code className="size-4 text-primary" />
            表格设计规范与代码模板
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <ul className="list-disc pl-4 text-xs text-muted-foreground space-y-1.5">
            <li>表格容器必须统一使用带有虚线边框的容器，即类名：<code className="bg-muted px-1 py-0.5 rounded font-mono">border border-dashed shadow-none rounded-lg overflow-hidden bg-background</code>。</li>
            <li>表头背景设为 <code className="bg-muted px-1 py-0.5 rounded font-mono">bg-muted/40</code>。表头表项字号统一为小字粗体 <code className="bg-muted px-1 py-0.5 rounded font-mono">text-xs font-semibold</code>。</li>
            <li>单元格行与列之间使用虚线分割（<code className="bg-muted px-1 py-0.5 rounded font-mono">border-dashed</code>）。表格行悬浮交互采用温和过渡色 <code className="bg-muted px-1 py-0.5 rounded font-mono">hover:bg-muted/10 transition-colors</code>。</li>
            <li>编码、主键或唯一 ID 类字段一律采用等宽字体呈现：<code className="bg-muted px-1 py-0.5 rounded font-mono">font-mono text-xs</code>。</li>
            <li>状态标签（Badge）尽量圆角化（<code className="bg-muted px-1 py-0.5 rounded font-mono">rounded-full py-0 px-2</code>），并以浅背景配以各自对应的图标圆点，以实现现代、高对比度的视觉质感。</li>
          </ul>
          <pre className="text-[11px] font-mono text-muted-foreground overflow-x-auto p-3 bg-background rounded border border-border/40 leading-relaxed">
{`<div className="border border-dashed shadow-none rounded-lg overflow-hidden bg-background">
  <Table>
    <TableHeader className="bg-muted/40">
      <TableRow className="border-dashed hover:bg-transparent">
        <TableHead className="text-xs font-semibold">编号</TableHead>
        <TableHead className="text-xs font-semibold">状态</TableHead>
      </TableRow>
    </TableHeader>
    <TableBody>
      <TableRow className="border-dashed hover:bg-muted/10 transition-colors">
        <TableCell className="font-mono text-xs">ID-1001</TableCell>
        <TableCell>
          <Badge variant="outline" className="text-[10px] bg-emerald-500/10 border-emerald-500/20 text-emerald-600 rounded-full py-0 px-2">
            <span className="size-1 bg-emerald-500 rounded-full mr-1.5 shrink-0" />
            成功
          </Badge>
        </TableCell>
      </TableRow>
    </TableBody>
  </Table>
</div>`}
          </pre>
        </CardContent>
      </Card>
    </div>
  )
}
