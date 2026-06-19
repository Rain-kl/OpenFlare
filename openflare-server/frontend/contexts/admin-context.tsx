"use client"

import * as React from "react"
import {createContext, useCallback, useContext, useRef, useState} from "react"
import {useQueryClient} from "@tanstack/react-query"

import type {SystemConfig, UpdateSystemConfigRequest} from "@/lib/services/admin"
import services from "@/lib/services"
import {handleContextError} from "@/lib/utils/error-handling"


/** Admin 上下文状态接口 */
export interface AdminContextState {
  systemConfigs: SystemConfig[]
  systemConfigsLoading: boolean
  systemConfigsError: Error | null
  refetchSystemConfigs: (type?: 'system' | 'business') => Promise<void>
  updateSystemConfig: (key: string, data: UpdateSystemConfigRequest) => Promise<void>
}

const AdminContext = createContext<AdminContextState | null>(null)

/**
 * Admin Provider
 * 提供 admin 相关的数据状态管理
 *
 * @example
 * ```tsx
 * <AdminProvider>
 *   <div>内容</div>
 * </AdminProvider>
 * ```
 * @param {React.ReactNode} children - Admin Provider 的子元素
 */
export function AdminProvider({ children }: { children: React.ReactNode }) {
  const queryClient = useQueryClient()
  const [systemConfigs, setSystemConfigs] = useState<SystemConfig[]>([])
  const [systemConfigsLoading, setSystemConfigsLoading] = useState(false)
  const [systemConfigsError, setSystemConfigsError] = useState<Error | null>(null)

  const systemRequestIdRef = useRef(0)
  const lastConfigTypeRef = useRef<'system' | 'business' | undefined>(undefined)

  /** 获取系统配置列表 */
  const refetchSystemConfigs = useCallback(async (type?: 'system' | 'business') => {
    lastConfigTypeRef.current = type
    const requestId = ++systemRequestIdRef.current

    try {
      setSystemConfigsLoading(true)
      setSystemConfigsError(null)
      const data = await services.adminSystemConfig.listSystemConfigs(type)

      if (requestId !== systemRequestIdRef.current) {
        return
      }

      setSystemConfigs(data)
      setSystemConfigsLoading(false)
    } catch (error) {
      if (requestId !== systemRequestIdRef.current) {
        return
      }

      const errorObject = handleContextError(error, '加载系统配置失败', { logError: true })
      setSystemConfigsError(errorObject)
      setSystemConfigsLoading(false)
    }
  }, [])

  /** 更新系统配置 */
  const updateSystemConfig = useCallback(async (key: string, data: UpdateSystemConfigRequest) => {
    try {
      await services.adminSystemConfig.updateSystemConfig(key, data)
      await queryClient.invalidateQueries({ queryKey: ['public-config'] })
      await refetchSystemConfigs(lastConfigTypeRef.current)
    } catch (error) {
      handleContextError(error, '更新系统配置失败')
      throw error
    }
  }, [queryClient, refetchSystemConfigs])

  const value: AdminContextState = {
    systemConfigs,
    systemConfigsLoading,
    systemConfigsError,
    refetchSystemConfigs,
    updateSystemConfig,
  }

  return (
    <AdminContext.Provider value={value}>
      {children}
    </AdminContext.Provider>
  )
}

/**
 * 使用 Admin 上下文
 *
 * @example
 * ```tsx
 * const { systemConfigs } = useAdmin()
 * ```
 * @returns {AdminContextState} Admin 上下文状态
 */
export function useAdmin() {
  const context = useContext(AdminContext)

  if (!context) {
    throw new Error('useAdmin 必须在 AdminProvider 内部使用')
  }

  return context
}
