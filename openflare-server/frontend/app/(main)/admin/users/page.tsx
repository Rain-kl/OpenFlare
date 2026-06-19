"use client"

import {useEffect, useState} from "react"
import {Button} from "@/components/ui/button"
import {Switch} from "@/components/ui/switch"
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow} from "@/components/ui/table"
import {Badge} from "@/components/ui/badge"
import {Avatar, AvatarFallback, AvatarImage} from "@/components/ui/avatar"
import {Eye, Layers, Loader2, Plus, Trash2, UserRound, UserX,} from "lucide-react"
import {Tooltip, TooltipContent, TooltipProvider, TooltipTrigger} from "@/components/ui/tooltip"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle
} from "@/components/ui/alert-dialog"

import type {AdminUser} from "@/lib/services/admin"
import {formatDateTime} from "@/lib/utils"
import {EmptyStateWithBorder} from "@/components/layout/empty"
import {LoadingStateWithBorder} from "@/components/layout/loading"
import {ErrorInline} from "@/components/layout/error"
import {useAdminUsers} from "@/contexts/admin-users-context"
import {CreateUserModal} from "./components/create-user-modal"
import {UserFilterBar} from "./components/user-filter-bar"
import {UserDetailSheet} from "./components/user-detail-sheet"

export default function UsersPage() {
  const {
    users,
    loading,
    error,
    fetchUsers,
    getUserDetail,
    updateUserStatus,
    deleteUser
  } = useAdminUsers()

  const [selectedUser, setSelectedUser] = useState<AdminUser | null>(null)
  const [detailOpen, setDetailOpen] = useState(false)
  const [detailLoading, setDetailLoading] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<AdminUser | null>(null)
  const [deleteLoading, setDeleteLoading] = useState(false)
  const [createModalOpen, setCreateModalOpen] = useState(false)

  useEffect(() => {
    fetchUsers()
  }, [fetchUsers])

  const handleStatusToggle = async (user: AdminUser) => {
    await updateUserStatus(user)

    if (selectedUser?.id === user.id) {
      setSelectedUser(prev => prev ? { ...prev, is_active: !prev.is_active } : null)
    }
  }

  const handleShowDetail = async (user: AdminUser) => {
    setSelectedUser(user)
    setDetailOpen(true)
    setDetailLoading(true)

    try {
      const detail = await getUserDetail(user.id)
      setSelectedUser(detail)
    } catch {
      setSelectedUser(user)
    } finally {
      setDetailLoading(false)
    }
  }

  const handleDeleteUser = async () => {
    if (!deleteTarget) return

    setDeleteLoading(true)
    try {
      await deleteUser(deleteTarget)
      if (selectedUser?.id === deleteTarget.id) {
        setDetailOpen(false)
        setSelectedUser(null)
      }
      setDeleteTarget(null)
    } finally {
      setDeleteLoading(false)
    }
  }

  return (
    <div className="py-6 space-y-4">
      {/* 顶部标题栏 */}
      <div className="flex items-center justify-between pb-2">
        <div className="flex items-center gap-2">
          <UserRound className="size-5 text-primary" />
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">用户管理</h1>
          </div>
        </div>
        <Button variant="secondary" size="sm" className="h-7 text-xs" onClick={() => setCreateModalOpen(true)}>
          <Plus className="size-3.5 mr-1" />
          新增用户
        </Button>
      </div>

      {/* 筛选与分页栏 (已拆分为独立自治组件) */}
      <UserFilterBar />

      {error ? (
        <div className="p-8 border border-dashed rounded-lg">
          <ErrorInline error={error} onRetry={() => fetchUsers(true)} className="justify-center" />
        </div>
      ) : loading && users.length === 0 ? (
        <LoadingStateWithBorder icon={Layers} description="加载用户列表中..." />
      ) : users.length === 0 ? (
        <EmptyStateWithBorder icon={UserX} description="暂无用户数据" />
      ) : (
        <div className="border border-dashed shadow-none rounded-lg overflow-hidden">
          <TooltipProvider delayDuration={0}>
          <Table className="w-full caption-bottom text-sm min-w-full">
            <TableHeader className="sticky top-0 z-20 bg-background">
              <TableRow className="border-b border-dashed hover:bg-transparent">
                <TableHead className="w-[90px] whitespace-nowrap py-2 h-8">ID</TableHead>
                <TableHead className="w-[120px] whitespace-nowrap py-2 h-8">用户</TableHead>
                <TableHead className="whitespace-nowrap min-w-[140px] py-2 h-8 pl-4">上次登陆</TableHead>
                <TableHead className="whitespace-nowrap min-w-[140px] py-2 h-8">注册时间</TableHead>
                <TableHead className="whitespace-nowrap min-w-[140px] py-2 h-8">上次更新</TableHead>
                <TableHead className="sticky right-0 text-center bg-background z-10 w-[110px] py-2 h-8">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {users.map((user) => (
                <TableRow
                  key={user.id}
                  className="border-dashed hover:bg-muted/30 cursor-pointer group"
                  onClick={() => handleShowDetail(user)}
                >
                  <TableCell className="font-mono text-[11px] text-muted-foreground py-1">{user.id}</TableCell>
                  <TableCell className="py-1">
                    <div className="flex items-center gap-2">
                      <Avatar className="h-7 w-7 rounded-sm border">
                        <AvatarImage src={user.avatar_url} />
                        <AvatarFallback className="rounded-sm text-[10px]">
                          {user.username.substring(0, 2).toUpperCase()}
                        </AvatarFallback>
                      </Avatar>
                      <div className="flex flex-col gap-0">
                        <div className="flex items-center gap-1.5">
                          <span className="font-medium text-[11px] leading-tight max-w-[100px] truncate" title={user.nickname}>{user.nickname}</span>
                          {user.is_admin && (
                            <Badge variant="secondary" className="text-[9px] h-3.5 px-0.5 rounded-[2px] font-normal leading-none tracking-tighter">
                              ADM
                            </Badge>
                          )}
                        </div>
                        <div className="flex items-center gap-1.5">
                          <span className="text-[10px] text-muted-foreground font-mono leading-tight">@{user.username}</span>
                        </div>
                      </div>
                    </div>
                  </TableCell>

                  <TableCell className="text-[10px] text-muted-foreground font-mono whitespace-nowrap py-1 pl-4">
                    {formatDateTime(user.last_login_at)}
                  </TableCell>
                  <TableCell className="text-[10px] text-muted-foreground font-mono whitespace-nowrap py-1">
                    {formatDateTime(user.created_at)}
                  </TableCell>
                  <TableCell className="text-[10px] text-muted-foreground font-mono whitespace-nowrap py-1">
                    {formatDateTime(user.updated_at)}
                  </TableCell>
                  <TableCell className="sticky right-0 text-center bg-background z-10 py-1" onClick={(e) => e.stopPropagation()}>
                    <div className="flex items-center justify-center gap-0.5">
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <div>
                              <Switch
                                checked={user.is_active}
                                onCheckedChange={() => handleStatusToggle(user)}
                                disabled={user.is_admin}
                                className="scale-75 data-[state=checked]:bg-green-600 h-4 w-7"
                              />
                            </div>
                          </TooltipTrigger>
                          <TooltipContent side="top" className="text-xs">
                            {user.is_admin ? '管理员账户' : user.is_active ? '禁用账户' : '启用账户'}
                          </TooltipContent>
                        </Tooltip>

                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button variant="ghost" size="icon" className="h-6 w-6 text-muted-foreground hover:text-foreground" onClick={() => handleShowDetail(user)}>
                              <Eye className="size-3" />
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent side="top" className="text-xs">
                            查看详情
                          </TooltipContent>
                        </Tooltip>

                      {!user.is_admin && (
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                variant="ghost"
                                size="icon"
                                className="h-6 w-6 text-muted-foreground hover:text-destructive"
                                onClick={() => setDeleteTarget(user)}
                              >
                                <Trash2 className="size-3" />
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent side="top" className="text-xs">
                              删除用户
                            </TooltipContent>
                          </Tooltip>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          </TooltipProvider>
        </div>
      )}

      {/* 用户详情侧拉抽屉 (已拆分为独立子组件) */}
      <UserDetailSheet
        selectedUser={selectedUser}
        isOpen={detailOpen}
        onOpenChange={setDetailOpen}
        detailLoading={detailLoading}
        onStatusToggle={handleStatusToggle}
        onDeleteTarget={setDeleteTarget}
      />

      {/* 删除确认警告弹窗 */}
      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && !deleteLoading && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除用户</AlertDialogTitle>
            <AlertDialogDescription>
              确定要删除用户 {deleteTarget?.nickname || deleteTarget?.username} 吗？该操作会移除用户账号，删除后无法撤销。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteLoading}>取消</AlertDialogCancel>
            <AlertDialogAction onClick={handleDeleteUser} disabled={deleteLoading}>
              {deleteLoading && <Loader2 className="size-3 animate-spin" />}
              确认删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* 新建用户模态弹窗 */}
      <CreateUserModal isOpen={createModalOpen} onClose={() => setCreateModalOpen(false)} />
    </div>
  )
}
