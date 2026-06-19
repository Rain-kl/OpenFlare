"use client"

import {useUser} from "@/contexts/user-context"

/**
 * Auth provider bridge hook
 *
 * Provides a stable interface for components that use the useAuth() pattern.
 * Delegates to the canonical UserContext under the hood.
 */
export function useAuth() {
  return useUser()
}
