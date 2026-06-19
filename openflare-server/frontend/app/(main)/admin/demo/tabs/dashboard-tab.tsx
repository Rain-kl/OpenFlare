// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

"use client"

import * as React from "react"
import {Code, Cpu, Database, HardDrive} from "lucide-react"
import {Card, CardContent, CardHeader, CardTitle} from "@/components/ui/card"
import {Progress} from "@/components/ui/progress"

export function DashboardTab() {
  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {/* CPU */}
        <Card className="border-dashed shadow-none">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <span className="text-xs font-medium text-muted-foreground">容器 CPU 使用率</span>
            <Cpu className="size-4 text-primary" />
          </CardHeader>
          <CardContent className="space-y-2">
            <div className="text-2xl font-semibold tracking-tight">42.8 %</div>
            <Progress value={42.8} className="h-1.5" />
            <p className="text-[10px] text-muted-foreground flex items-center gap-1">
              <span className="size-1.5 rounded-full bg-emerald-500 inline-block animate-pulse" />
              容器运行正常，负载平稳
            </p>
          </CardContent>
        </Card>

        {/* Storage */}
        <Card className="border-dashed shadow-none">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <span className="text-xs font-medium text-muted-foreground">存储已用容量</span>
            <HardDrive className="size-4 text-primary" />
          </CardHeader>
          <CardContent className="space-y-2">
            <div className="text-2xl font-semibold tracking-tight">72.4 GB <span className="text-xs text-muted-foreground">/ 100 GB</span></div>
            <Progress value={72.4} className="h-1.5" />
            <p className="text-[10px] text-muted-foreground">
              数据已同步远端 S3 存储桶，支持自动过期
            </p>
          </CardContent>
        </Card>

        {/* DB Overview */}
        <Card className="border-dashed shadow-none">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <span className="text-xs font-medium text-muted-foreground">物理数据库</span>
            <Database className="size-4 text-primary" />
          </CardHeader>
          <CardContent className="space-y-2">
            <div className="text-2xl font-semibold tracking-tight">PostgreSQL <span className="text-xs font-normal text-muted-foreground">(v16.2)</span></div>
            <div className="flex items-center gap-3 text-xs text-muted-foreground">
              <div>活跃连接: <span className="font-semibold text-foreground">12</span></div>
              <div>总表数量: <span className="font-semibold text-foreground">34</span></div>
            </div>
            <p className="text-[10px] text-muted-foreground">
              链接池自动维护中，最大连接限制为 100
            </p>
          </CardContent>
        </Card>
      </div>

      {/* 指标卡片规范与代码 */}
      <Card className="border-dashed shadow-none bg-muted/20">
        <CardHeader className="pb-3">
          <CardTitle className="text-sm font-semibold flex items-center gap-1.5">
            <Code className="size-4 text-primary" />
            指标卡片 & 仪表盘设计规范
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <ul className="list-disc pl-4 text-xs text-muted-foreground space-y-1.5">
            <li>指标卡片统一采用 <code className="bg-muted px-1 py-0.5 rounded font-mono">border-dashed shadow-none</code> 作为边框和投影。</li>
            <li>头部使用 <code className="bg-muted px-1 py-0.5 rounded font-mono">flex flex-row items-center justify-between pb-2</code> 布局，左侧为标题小字，右侧为直接呈现的 Lucide 图标。</li>
            <li>数值强调使用且仅使用 <code className="bg-muted px-1 py-0.5 rounded font-mono">text-2xl font-semibold tracking-tight</code> 展示。</li>
            <li>进度条使用 <code className="bg-muted px-1 py-0.5 rounded font-mono">h-1.5</code> 细条高度，不喧宾夺主。</li>
            <li>底部的次要说明信息必须统一使用 <code className="bg-muted px-1 py-0.5 rounded font-mono">text-[10px] text-muted-foreground</code> 等辅助类。</li>
          </ul>
          <pre className="text-[11px] font-mono text-muted-foreground overflow-x-auto p-3 bg-background rounded border border-border/40 leading-relaxed">
{`<Card className="border-dashed shadow-none">
  <CardHeader className="flex flex-row items-center justify-between pb-2">
    <span className="text-xs font-medium text-muted-foreground">指标标题</span>
    <Cpu className="size-4 text-primary" />
  </CardHeader>
  <CardContent className="space-y-2">
    <div className="text-2xl font-semibold tracking-tight">42.8 %</div>
    <Progress value={42.8} className="h-1.5" />
    <p className="text-[10px] text-muted-foreground">状态说明小字</p>
  </CardContent>
</Card>`}
          </pre>
        </CardContent>
      </Card>
    </div>
  )
}
