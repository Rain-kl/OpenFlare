"use client"

import {Suspense} from "react"
import {useRouter, useSearchParams} from "next/navigation"
import {Tabs, TabsContent, TabsList, TabsTrigger} from "@/components/ui/tabs"
import {TaskManager} from "./components/task-manager"
import {TaskSchedulesManager} from "./components/task-schedules"
import {TaskExecutionsManager} from "./components/task-executions"
import {Spinner} from "@/components/ui/spinner"
import {Layers} from "lucide-react"

function TasksPageContent() {
  const searchParams = useSearchParams()
  const router = useRouter()
  const activeTab = searchParams.get("tab") || "tasks"

  const handleTabChange = (value: string) => {
    router.push(`/admin/tasks?tab=${value}`)
  }
  return (
    <div className="py-6 space-y-6">
      <div className="flex items-center gap-2">
        <Layers className="size-5 text-primary" />
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">任务管理</h1>
        </div>
      </div>

      <Tabs value={activeTab} onValueChange={handleTabChange} className="w-full">
        <TabsList variant="line" className="w-fit inline-flex gap-8 mb-6">
          <TabsTrigger value="tasks" className="px-0 pb-2 text-xs font-semibold">
            任务管理
          </TabsTrigger>
          <TabsTrigger value="schedules" className="px-0 pb-2 text-xs font-semibold">
            定时任务
          </TabsTrigger>
          <TabsTrigger value="executions" className="px-0 pb-2 text-xs font-semibold">
            任务日志
          </TabsTrigger>
        </TabsList>
        <TabsContent value="tasks" className="space-y-4 outline-none">
          <TaskManager />
        </TabsContent>
        <TabsContent value="schedules" className="space-y-4 outline-none">
          <TaskSchedulesManager />
        </TabsContent>
        <TabsContent value="executions" className="space-y-4 outline-none">
          <TaskExecutionsManager />
        </TabsContent>
      </Tabs>
    </div>
  )
}

export default function TasksPage() {
  return (
    <Suspense fallback={
      <div className="flex items-center justify-center min-h-[400px]">
        <Spinner className="h-8 w-8" />
      </div>
    }>
      <TasksPageContent />
    </Suspense>
  )
}
