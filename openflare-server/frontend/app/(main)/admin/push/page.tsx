// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

"use client"

import * as React from "react"
import {motion} from "motion/react"
import {Bell, History, Layers, Settings} from "lucide-react"

import {Tabs, TabsContent, TabsList, TabsTrigger} from "@/components/ui/tabs"
import {EventsTab} from "./components/events-tab"
import {HistoriesTab} from "./components/histories-tab"
import {SettingsTab} from "./components/settings-tab"

export default function PushAdminPage() {
  const [activeTab, setActiveTab] = React.useState("events")

  return (
    <motion.div
      initial={{ opacity: 0, y: 15 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3, ease: "easeOut" }}
      className="w-full py-6 space-y-6"
    >
      <div className="flex items-center gap-2">
        <Bell className="size-5 text-primary" />
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">通知推送管理</h1>
        </div>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
        <TabsList variant="line" className="w-fit inline-flex gap-8">
          <TabsTrigger value="events" className="px-0 pb-2 text-xs font-semibold">
            <Layers className="size-3.5 mr-1" />
            事件管理
          </TabsTrigger>
          <TabsTrigger value="histories" className="px-0 pb-2 text-xs font-semibold">
            <History className="size-3.5 mr-1" />
            通知历史
          </TabsTrigger>
          <TabsTrigger value="settings" className="px-0 pb-2 text-xs font-semibold">
            <Settings className="size-3.5 mr-1" />
            通道管理与设置
          </TabsTrigger>
        </TabsList>

        {/* ==================== 1. 事件管理 TAB ==================== */}
        <TabsContent value="events" className="focus-visible:outline-none">
          <EventsTab />
        </TabsContent>

        {/* ==================== 2. 通知历史 TAB ==================== */}
        <TabsContent value="histories" className="focus-visible:outline-none">
          <HistoriesTab />
        </TabsContent>

        {/* ==================== 3. 通道管理与设置 TAB ==================== */}
        <TabsContent value="settings" className="focus-visible:outline-none">
          <SettingsTab />
        </TabsContent>
      </Tabs>
    </motion.div>
  )
}
