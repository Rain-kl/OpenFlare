"use client"

import * as React from "react"
import {Globe, Loader2, Mail, MapPin, ShieldCheck, Smartphone, Trash2, UserCheck,} from "lucide-react"

import {Button} from "@/components/ui/button"
import {Badge} from "@/components/ui/badge"
import {Avatar, AvatarFallback, AvatarImage} from "@/components/ui/avatar"
import {Sheet, SheetContent, SheetTitle} from "@/components/ui/sheet"
import type {AdminUser} from "@/lib/services/admin"
import {cn, formatDateTime} from "@/lib/utils"

interface UserDetailSheetProps {
  selectedUser: AdminUser | null
  isOpen: boolean
  onOpenChange: (open: boolean) => void
  detailLoading: boolean
  onStatusToggle: (user: AdminUser) => Promise<void>
  onDeleteTarget: (user: AdminUser) => void
}

export function UserDetailSheet({
  selectedUser,
  isOpen,
  onOpenChange,
  detailLoading,
  onStatusToggle,
  onDeleteTarget,
}: UserDetailSheetProps) {

  const displayValue = (value?: string) => value && value.trim() ? value : "-"

  return (
    <Sheet open={isOpen} onOpenChange={onOpenChange}>
      <SheetContent className="sm:max-w-[400px] w-full p-0 flex flex-col gap-0">
        <SheetTitle className="px-5 py-3">用户档案</SheetTitle>

        {selectedUser && (
          <>
            <div className="flex-1 overflow-y-auto scrollbar-thin scrollbar-thumb-border scrollbar-track-transparent">
              <div className="flex flex-col pb-6">
                <div className="px-5 py-6 border-b border-border/50">
                  <div className="flex flex-col items-center text-center gap-3">
                    <Avatar className="h-20 w-20 rounded-full border-4 border-background ring-1 ring-border/20">
                      <AvatarImage src={selectedUser.avatar_url} />
                      <AvatarFallback className="rounded-full text-xl font-medium bg-secondary text-secondary-foreground">
                        {selectedUser.username.substring(0, 2).toUpperCase()}
                      </AvatarFallback>
                    </Avatar>

                    <div className="space-y-1.5">
                      <h3 className="text-lg font-bold tracking-tight">{selectedUser.nickname}</h3>
                      <div className="flex items-center justify-center gap-2">
                        <code className="px-1.5 py-0.5 rounded-md bg-muted text-[10px] font-mono text-muted-foreground">@{selectedUser.username}</code>
                        <Badge variant="secondary" className="h-4.5 px-1.5 text-[9px] uppercase font-medium">
                          UID: {selectedUser.id}
                        </Badge>
                        {selectedUser.is_admin && (
                          <Badge className="h-4.5 px-1.5 text-[9px] uppercase font-medium bg-primary text-primary-foreground">
                            Admin
                          </Badge>
                        )}
                      </div>
                    </div>

                    {detailLoading && (
                      <div className="flex items-center gap-1 text-[10px] text-muted-foreground">
                        <Loader2 className="size-3 animate-spin" />
                        正在刷新详情
                      </div>
                    )}

                    <div className="gap-4 w-full max-w-[240px] mt-1 pt-4 border-t border-border/50">
                      <div className="flex flex-col gap-0.5">
                        <span className="text-[9px] uppercase tracking-widest text-muted-foreground font-medium">注册时间</span>
                        <span className="font-mono text-xs font-semibold">{formatDateTime(selectedUser.created_at).split(' ')[0]}</span>
                      </div>
                    </div>
                  </div>
                </div>

                <div className="p-6 space-y-6">
                  <div className="space-y-4">
                    <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider px-1">个人资料</h4>
                    <div className="rounded-lg border divide-y bg-background/50">
                      <div className="flex items-center justify-between gap-4 p-3.5 text-sm">
                        <span className="flex items-center gap-2 text-[10px] text-muted-foreground">
                          <Mail className="size-3" />
                          邮箱
                        </span>
                        <span className="min-w-0 truncate text-right text-[10px]">{displayValue(selectedUser.email)}</span>
                      </div>
                      <div className="flex items-center justify-between gap-4 p-3.5 text-sm">
                        <span className="flex items-center gap-2 text-[10px] text-muted-foreground">
                          <Smartphone className="size-3" />
                          手机
                        </span>
                        <span className="min-w-0 truncate text-right text-[10px]">{displayValue(selectedUser.phone)}</span>
                      </div>
                      <div className="flex items-center justify-between gap-4 p-3.5 text-sm">
                        <span className="flex items-center gap-2 text-[10px] text-muted-foreground">
                          <span className="size-3 flex items-center justify-center font-bold text-[9px]">⚧</span>
                          性别
                        </span>
                        <span className="min-w-0 truncate text-right text-[10px]">{displayValue(selectedUser.gender)}</span>
                      </div>
                      <div className="flex items-center justify-between gap-4 p-3.5 text-sm">
                        <span className="flex items-center gap-2 text-[10px] text-muted-foreground">
                          <MapPin className="size-3" />
                          所在地
                        </span>
                        <span className="min-w-0 truncate text-right text-[10px]">{displayValue(selectedUser.location)}</span>
                      </div>
                      <div className="flex items-center justify-between gap-4 p-3.5 text-sm">
                        <span className="flex items-center gap-2 text-[10px] text-muted-foreground">
                          <Globe className="size-3" />
                          网站
                        </span>
                        <span className="min-w-0 truncate text-right text-[10px]">{displayValue(selectedUser.website)}</span>
                      </div>
                      <div className="flex flex-col gap-2 p-3.5 text-sm">
                        <span className="text-[10px] text-muted-foreground">简介</span>
                        <span className="break-words text-[10px] leading-5">{displayValue(selectedUser.bio)}</span>
                      </div>
                    </div>
                  </div>

                  <div className="space-y-4">
                    <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider px-1">系统记录</h4>
                    <div className="rounded-lg border divide-y bg-background/50">
                      <div className="flex items-center justify-between p-3.5 text-sm">
                        <span className="text-[10px]">账户状态</span>
                        <Badge variant={selectedUser.is_active ? "secondary" : "outline"} className="text-[10px]">
                          {selectedUser.is_active ? "正常" : "禁用"}
                        </Badge>
                      </div>
                      <div className="flex items-center justify-between p-3.5 text-sm">
                        <span className="text-[10px]">管理员</span>
                        <span className="font-mono text-[10px]">{selectedUser.is_admin ? "是" : "否"}</span>
                      </div>
                      <div className="flex items-center justify-between p-3.5 text-sm">
                        <span className="text-[10px]">最后登录</span>
                        <span className="font-mono text-[10px]">{formatDateTime(selectedUser.last_login_at)}</span>
                      </div>
                      <div className="flex items-center justify-between p-3.5 text-sm">
                        <span className="text-[10px]">注册时间</span>
                        <span className="font-mono text-[10px]">{formatDateTime(selectedUser.created_at)}</span>
                      </div>
                      <div className="flex items-center justify-between p-3.5 text-sm">
                        <span className="text-[10px]">最后更新</span>
                        <span className="font-mono text-[10px]">{formatDateTime(selectedUser.updated_at)}</span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              {!selectedUser.is_admin && (
                <div className="p-4 border-t bg-background/80 backdrop-blur-md shrink-0 flex flex-col gap-2">
                  <Button
                    variant={selectedUser.is_active ? "destructive" : "default"}
                    className={cn(
                      "w-full h-9 text-xs font-medium transition-all active:scale-[0.98]",
                      selectedUser.is_active
                        ? "bg-red-500 hover:bg-red-600 text-white"
                        : "bg-primary text-primary-foreground hover:bg-primary/90"
                    )}
                    onClick={() => onStatusToggle(selectedUser)}
                  >
                    {selectedUser.is_active ? (
                      <>
                        <ShieldCheck className="size-3 mr-1" />
                        封禁账户
                      </>
                    ) : (
                      <>
                        <UserCheck className="size-3 mr-1" />
                        解除封禁
                      </>
                    )}
                  </Button>
                  <Button
                    variant="outline"
                    className="w-full h-9 text-xs font-medium"
                    onClick={() => onDeleteTarget(selectedUser)}
                  >
                    <Trash2 className="size-3 mr-1" />
                    删除用户
                  </Button>
                </div>
              )}
            </div>
          </>
        )}
      </SheetContent>
    </Sheet>
  )
}
