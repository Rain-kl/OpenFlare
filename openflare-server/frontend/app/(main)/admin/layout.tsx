"use client"

import {AdminUsersProvider} from "@/contexts/admin-users-context"
import {RequireAdminAuth} from "@/components/auth/require-auth"

export default function AdminLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <RequireAdminAuth>
      <AdminUsersProvider>
        {children}
      </AdminUsersProvider>
    </RequireAdminAuth>
  )
}