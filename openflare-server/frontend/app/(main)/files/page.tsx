"use client"

import * as React from "react"
import {motion} from "motion/react"

import {RequireAuth} from "@/components/auth/require-auth"
import {UserFileManager} from "@/components/common/user/file-manager"
import {FolderOpen} from "lucide-react"

export default function UserFilesPage() {
  return (
    <RequireAuth>
    <motion.div
      initial={{ opacity: 0, y: 15 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.35, ease: "easeOut" }}
      className="flex w-full flex-col gap-6 py-6"
    >
      {/* 顶部标题区 */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 pb-5">
        <div className="flex items-center gap-2">
          <FolderOpen className="size-5 text-primary" />
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">我的文件</h1>
          </div>
        </div>
      </div>

      <UserFileManager />
    </motion.div>
    </RequireAuth>
  )
}
