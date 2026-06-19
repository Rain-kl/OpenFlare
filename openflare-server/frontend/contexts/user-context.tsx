"use client"

import {createContext, type ReactNode, useCallback, useContext, useEffect, useRef, useState} from 'react'

import {AuthService} from '@/lib/services/auth'
import {User} from '@/lib/services/auth/types'


/** 用户状态接口 */
interface UserState {
  user: User | null
  loading: boolean
  error: string | null
}

/** 用户上下文接口 */
interface UserContextValue extends UserState {
  setUser: (user: User) => void
  refetch: () => Promise<void>
  logout: () => Promise<void>
}



/** 用户上下文 */
const UserContext = createContext<UserContextValue | undefined>(undefined)

/**
 * 用户Provider组件
 *
 * @param {React.ReactNode} children - 用户 Provider 的子元素
 * @returns {React.ReactNode} 用户 Provider 组件
 * @example
 * ```tsx
 * <UserProvider>
 *   <UserContext.Provider value={{ user, loading, error, refetch, updatePayKey, getTrustLevelLabel, getPayLevelLabel, logout }}>
 *     {children}
 *   </UserContext.Provider>
 * </UserProvider>
 * ```
 */
export function UserProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<UserState>({
    user: null,
    loading: true,
    error: null,
  })

  const isMountedRef = useRef(true)



  /** 获取用户信息 */
  const fetchUser = useCallback(async () => {
    try {
      setState(prev => ({ ...prev, loading: true, error: null }))
      const user = await AuthService.getUserInfo()

      if (!isMountedRef.current) return

      setState({ user, loading: false, error: null })
    } catch (error) {
      if (!isMountedRef.current) return

      setState({
        user: null,
        loading: false,
        error: error instanceof Error ? error.message : '获取用户信息失败',
      })
    }
  }, [])

  /** 重新获取用户信息 */
  const refetch = useCallback(async () => {
    await fetchUser()
  }, [fetchUser])

  /** 直接设置用户信息（登录/注册后免二次请求） */
  const setUser = useCallback((user: User) => {
    setState({ user, loading: false, error: null })
  }, [])

  /** 用户登出 */
  const logout = useCallback(async () => {
    try {
      await AuthService.logout()

      if (!isMountedRef.current) return

      setState({ user: null, loading: false, error: null })
      window.location.href = '/login'
    } catch (error) {
      if (!isMountedRef.current) {
        throw error
      }

      const errorMessage = error instanceof Error ? error.message : '登出失败'
      setState(prev => ({
        ...prev,
        error: errorMessage,
      }))
      throw new Error(errorMessage)
    }
  }, [])

  /** 组件挂载时获取用户信息（登录/注册页跳过，避免无意义请求） */
  useEffect(() => {
    isMountedRef.current = true

    const path = window.location.pathname
    if (path === "/login" || path === "/register") {
      setState({user: null, loading: false, error: null})
      return () => {
        isMountedRef.current = false
      }
    }

    fetchUser()

    return () => {
      isMountedRef.current = false
    }
  }, [fetchUser])

  return (
    <UserContext.Provider
      value={{
        ...state,
        setUser,
        refetch,
        logout,
      }}
    >
      {children}
    </UserContext.Provider>
  )
}

/**
 * 使用用户上下文的Hook
 *
 * @returns {UserContextValue} 用户上下文值
 * @example
 * ```tsx
 * const { user, loading, error, refetch, updatePayKey, getTrustLevelLabel, getPayLevelLabel, logout } = useUser()
 * ```
 */
export function useUser(): UserContextValue {
  const context = useContext(UserContext)
  if (context === undefined) {
    throw new Error('useUser must be used within a UserProvider')
  }
  return context
}
