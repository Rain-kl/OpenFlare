"use client"

import * as React from "react"
import {motion} from "motion/react"
import {FolderOpen} from "lucide-react"

import {Tabs, TabsContent, TabsList, TabsTrigger} from "@/components/ui/tabs"
import {FileStats} from "./components/file-stats"
import {FileList} from "./components/file-list"
import {StorageConfigTab} from "./components/storage-config-tab"

export default function FilesPage() {
  const [activeTab, setActiveTab] = React.useState("stats")

  return (
    <motion.div
      initial={{ opacity: 0, y: 15 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.35, ease: "easeOut" }}
      className="flex w-full flex-col gap-6 py-6"
    >
      {/* 顶部标题区 */}
      <div className="flex items-center gap-2">
        <FolderOpen className="size-5 text-primary" />
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">存储管理</h1>
        </div>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
        <TabsList variant="line" className="w-fit inline-flex gap-8 mb-6">
          <TabsTrigger value="stats" className="px-0 pb-2 text-xs font-semibold">
            文件存储信息
          </TabsTrigger>
          <TabsTrigger value="list" className="px-0 pb-2 text-xs font-semibold">
            文件列表
          </TabsTrigger>
          <TabsTrigger value="storage" className="px-0 pb-2 text-xs font-semibold">
            存储配置
          </TabsTrigger>
        </TabsList>

        {/* ──────── TAB 1: 统计看板 ──────── */}
        <TabsContent value="stats" className="outline-hidden">
          {activeTab === "stats" ? <FileStats /> : null}
        </TabsContent>

        {/* ──────── TAB 2: 文件列表 ──────── */}
        <TabsContent value="list" className="outline-hidden">
          {activeTab === "list" ? <FileList /> : null}
        </TabsContent>

        <TabsContent value="storage" className="outline-hidden">
          {activeTab === "storage" ? <StorageConfigTab /> : null}
        </TabsContent>
      </Tabs>
    </motion.div>
  )
}
