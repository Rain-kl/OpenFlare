"use client"

import * as React from "react"
import {createContext, useCallback, useContext, useEffect, useRef, useState} from "react"
import {toast} from "sonner"
import type {AdminUser, CreateUserRequest, ListUsersRequest} from "@/lib/services/admin"
import services from "@/lib/services"

/** 用户列表查询参数 */
export interface UserQueryParams {
  page: number
  page_size: number
  user_id?: string
  username?: string
  status?: 'all' | 'active' | 'inactive'
}

/** Admin Users Context State Interface */
interface AdminUsersContextState {
  users: AdminUser[]
  total: number
  loading: boolean
  error: Error | null

  // Params
  page: number
  pageSize: number
  searchUserId: string
  searchUsername: string
  statusFilter: 'all' | 'active' | 'inactive'

  // Actions
  setPage: (page: number) => void
  setPageSize: (size: number) => void
  setSearchUserId: (userId: string) => void
  setSearchUsername: (username: string) => void
  setStatusFilter: (status: 'all' | 'active' | 'inactive') => void

  fetchUsers: (force?: boolean) => Promise<void>
  refresh: () => Promise<void>
  getUserDetail: (id: string) => Promise<AdminUser>
  updateUserStatus: (user: AdminUser) => Promise<void>
  createUser: (req: CreateUserRequest) => Promise<AdminUser>
  deleteUser: (user: AdminUser) => Promise<void>
}

const CACHE_DURATION = 5 * 60 * 1000 // 5 minutes cache

const AdminUsersContext = createContext<AdminUsersContextState | null>(null)

export function AdminUsersProvider({ children }: { children: React.ReactNode }) {
  // State
  const [users, setUsers] = useState<AdminUser[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  // Query Params State
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [searchUserId, setSearchUserId] = useState("")
  const [searchUsername, setSearchUsername] = useState("")
  const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'inactive'>('all')
  const [debouncedSearchUserId, setDebouncedSearchUserId] = useState("")
  const [debouncedSearchUsername, setDebouncedSearchUsername] = useState("")

  // Cache
  const cacheRef = useRef<Record<string, { data: AdminUser[], total: number, timestamp: number }>>({})
  const latestRequestIdRef = useRef(0)

  // Debounce search
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearchUserId(searchUserId)
      setDebouncedSearchUsername(searchUsername)
      if (searchUserId !== debouncedSearchUserId || searchUsername !== debouncedSearchUsername) {
        setPage(1) // Reset to page 1 on search change
      }
    }, 500)
    return () => clearTimeout(timer)
  }, [searchUserId, searchUsername, debouncedSearchUserId, debouncedSearchUsername])

  const generateCacheKey = (params: UserQueryParams) => {
    return JSON.stringify(params)
  }

  const fetchUsers = useCallback(async (force = false) => {
    const params: UserQueryParams = {
      page,
      page_size: pageSize,
      user_id: debouncedSearchUserId || undefined,
      username: debouncedSearchUsername || undefined,
      status: statusFilter
    }

    const cacheKey = generateCacheKey(params)
    const now = Date.now()
    const cached = cacheRef.current[cacheKey]

    // Use cache if valid and not forced
    if (!force && cached && (now - cached.timestamp < CACHE_DURATION)) {
      setUsers(cached.data)
      setTotal(cached.total)
      setLoading(false)
      setError(null)
      return
    }

    setLoading(true)
    setError(null)
    const requestId = ++latestRequestIdRef.current

    try {
      // Current API doesn't support status filter in listUsers?
      // The previous implementation did client-side filtering.
      // Ideally backend supports it. If not, we fetch and filter?
      // "services.adminUser.listUsers" in previous code only took page, page_size, username.
      // So status filter was client side.
      // However, caching client-filtered result is tricky if we don't have all data.
      // But previous implementation fetched *paged* data then filtered? No, that would be wrong (filtering 20 items might leave 0).
      // Let's check previous implementation:
      // "const data = await services.adminUser.listUsers(...) ... let filteredUsers = data.users ... if (statusFilter...) filtered..."
      // This means filtering happens ONLY on the current page of results! This is technically buggy if the user wants "all inactive users".
      // But preserving that behavior for now.

      const requestParams: ListUsersRequest = {
        page,
        page_size: pageSize,
        user_id: debouncedSearchUserId || undefined,
        username: debouncedSearchUsername || undefined
      }

      const response = await services.adminUser.listUsers(requestParams)

      if (requestId !== latestRequestIdRef.current) return

      let resultUsers = response.users
      // Client-side filtering as per previous logic (though restrictive)
      if (statusFilter === 'active') {
        resultUsers = resultUsers.filter(u => u.is_active)
      } else if (statusFilter === 'inactive') {
        resultUsers = resultUsers.filter(u => !u.is_active)
      }

      setUsers(resultUsers)
      setTotal(response.total) // Note: Total might be inaccurate if we filter client side! But per previous code...

      // Update cache
      cacheRef.current[cacheKey] = {
        data: resultUsers,
        total: response.total,
        timestamp: now
      }

    } catch (err) {
      if (requestId !== latestRequestIdRef.current) return
      setError(err instanceof Error ? err : new Error('Failed to fetch users'))
    } finally {
      if (requestId === latestRequestIdRef.current) {
        setLoading(false)
      }
    }
  }, [page, pageSize, debouncedSearchUserId, debouncedSearchUsername, statusFilter])

  // Removed auto-fetch useEffect. Consumer (UsersManager) should trigger fetch.


  const refresh = async () => {
    await fetchUsers(true)
  }

  const updateUserStatus = async (user: AdminUser) => {
    const originalStatus = user.is_active

    // Optimistic update
    setUsers(prev => prev.map(u =>
      u.id === user.id ? { ...u, is_active: !u.is_active } : u
    ))

    // Clear cache because data changed
    cacheRef.current = {}

    try {
      await services.adminUser.updateUserStatus(user.id, { is_active: !user.is_active })
      toast.success(`已${ !user.is_active ? '启用' : '禁用' }用户 ${ user.username }`)
    } catch {
      // Revert on error
      setUsers(prev => prev.map(u =>
        u.id === user.id ? { ...u, is_active: originalStatus } : u
      ))
      toast.error('更新状态失败')
    }
  }

  const getUserDetail = async (id: string) => {
    return services.adminUser.getUser(id)
  }

  const createUser = async (req: CreateUserRequest) => {
    try {
      const newUser = await services.adminUser.createUser(req)
      setUsers(prev => [newUser, ...prev])
      setTotal(prev => prev + 1)
      // Clear cache because data changed
      cacheRef.current = {}
      toast.success(`已成功创建用户 ${ newUser.username }`)
      return newUser
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '创建用户失败')
      throw err
    }
  }

  const deleteUser = async (user: AdminUser) => {
    try {
      await services.adminUser.deleteUser(user.id)
      setUsers(prev => prev.filter(u => u.id !== user.id))
      setTotal(prev => Math.max(0, prev - 1))
      cacheRef.current = {}
      toast.success(`已删除用户 ${ user.username }`)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '删除用户失败')
      throw err
    }
  }

  const value = {
    users,
    total,
    loading,
    error,
    page,
    pageSize,
    searchUserId,
    searchUsername,
    statusFilter,
    setPage,
    setPageSize,
    setSearchUserId,
    setSearchUsername,
    setStatusFilter,
    fetchUsers,
    refresh,
    getUserDetail,
    updateUserStatus,
    createUser,
    deleteUser
  }

  return (
    <AdminUsersContext.Provider value={value}>
      {children}
    </AdminUsersContext.Provider>
  )
}

export function useAdminUsers() {
  const context = useContext(AdminUsersContext)
  if (!context) {
    throw new Error('useAdminUsers must be used within an AdminUsersProvider')
  }
  return context
}
