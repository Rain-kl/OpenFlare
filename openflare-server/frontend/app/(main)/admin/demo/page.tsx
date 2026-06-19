// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

"use client"

import * as React from "react"
import {Code} from "lucide-react"

import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card"
import {Tabs, TabsContent, TabsList, TabsTrigger} from "@/components/ui/tabs"

// 引入模块化拆分后的 Tab 子组件
import {DashboardTab} from "./tabs/dashboard-tab"
import {TableTab} from "./tabs/table-tab"
import {ControlsTab} from "./tabs/controls-tab"

export default function HeaderDemoPage() {
  const [activeTab, setActiveTab] = React.useState("dashboard")

  return (
    <div className="py-6 px-1 space-y-6">
      {/* 1. 标准页面标题 */}
      <div className="flex items-center gap-2">
        <Code className="size-5 text-primary" />
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">开发规范示例</h1>
        </div>
      </div>

      {/* 说明区域 */}
      <Card className="border-dashed shadow-none">
        <CardHeader className="pb-3">
          <div className="flex items-center gap-2">
            <Code className="size-4 text-primary" />
            <CardTitle className="text-base font-semibold">规范说明</CardTitle>
          </div>
          <CardDescription>
            此页面展示了后台 Sidebar 关联页面的标准标题栏布局，供开发参考。
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">核心规范细节与编写原则：</h3>
            <ul className="list-disc pl-4 text-xs text-muted-foreground space-y-1.5">
              <li><strong>结构专注</strong>：页面标题区必须保持绝对干净。请不要在标题下方放置任何描述段落或小字副标题，让页面在视线进入时保持极简与信息纯粹。</li>
              <li><strong>容器与上边距对齐</strong>：使用 <code className="bg-muted px-1 py-0.5 rounded font-mono">flex items-center gap-2</code> 作为标题行基础容器（不含下边框），外层统一使用 <code className="bg-muted px-1 py-0.5 rounded font-mono">py-6 px-1</code> (或 <code className="bg-muted px-1 py-0.5 rounded font-mono">py-6</code>) 进行上边距的视觉对齐，确保切换菜单时顶部高度感一致。</li>
              <li><strong>图标呈现</strong>：直接将 Lucide 图标组件嵌套于标题容器中，应用 <code className="bg-muted px-1 py-0.5 rounded font-mono">size-5 text-primary</code> 样式。禁止为图标包裹任何背景色小卡片、圆角或装饰性边框。</li>
              <li><strong>文本样式标准</strong>：文本使用且仅使用 <code className="bg-muted px-1 py-0.5 rounded font-mono">{'h1 className="text-2xl font-semibold tracking-tight"'}</code>，禁用字重加粗（如 <code className="bg-muted px-1 py-0.5 rounded font-mono">font-bold</code>）或任何渐变色，保证全站字体一致性。</li>
              <li><strong>Tabs 模块化拆分</strong>：凡是带有多个 Tab 页切换的页面，**禁止**将所有 Tab 的渲染代码堆积在同一个物理页面主文件内。每个 Tab 页对应的 Content 必须单独拆分为独立组件文件（如 <code className="bg-muted px-1 py-0.5 rounded font-mono">tabs/events-tab.tsx</code> 或就近放在 <code className="bg-muted px-1 py-0.5 rounded font-mono">components/</code> 下），主页面文件只负责维护当前 Tab 的激活状态及外层骨架布局。</li>
              <li><strong>复杂度驱动的区块拆分</strong>：不仅是 Tabs 切换，当一个页面代码行数超过 600 行时，必须将高密度区块拆分为子组件。路由专属、不复用的子组件应放在对应路由的特征目录中（如 <code className="bg-muted px-1 py-0.5 rounded font-mono">app/(main)/admin/database/components/</code>），只有真正跨页面复用的组件才进入全局 <code className="bg-muted px-1 py-0.5 rounded font-mono">components/common/</code>。典型标杆案例参考“数据管理” (<code className="bg-muted px-1 py-0.5 rounded font-mono">/admin/database</code>) 的 <code className="bg-muted px-1 py-0.5 rounded font-mono">table-browser.tsx</code>、<code className="bg-muted px-1 py-0.5 rounded font-mono">cache-manager.tsx</code> 与 <code className="bg-muted px-1 py-0.5 rounded font-mono">sql-console.tsx</code>。</li>
            </ul>
          </div>
        </CardContent>
      </Card>

      {/* Tabs 切换 */}
      <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
        <TabsList variant="line" className="w-fit inline-flex gap-8 mb-6">
          <TabsTrigger value="dashboard" className="px-0 pb-2 text-xs font-semibold">
            仪表盘与指标卡片
          </TabsTrigger>
          <TabsTrigger value="table" className="px-0 pb-2 text-xs font-semibold">
            数据表格规范
          </TabsTrigger>
          <TabsTrigger value="controls" className="px-0 pb-2 text-xs font-semibold">
            输入控件与表单
          </TabsTrigger>
        </TabsList>

        <TabsContent value="dashboard" className="focus-visible:outline-none">
          <DashboardTab />
        </TabsContent>
        <TabsContent value="table" className="focus-visible:outline-none">
          <TableTab />
        </TabsContent>
        <TabsContent value="controls" className="focus-visible:outline-none">
          <ControlsTab />
        </TabsContent>
      </Tabs>
    </div>
  )
}
