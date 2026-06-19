"use client"

import {useEffect, useState} from "react"
import {Button} from "@/components/ui/button"
import {Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle} from "@/components/ui/dialog"
import {Input} from "@/components/ui/input"
import {Label} from "@/components/ui/label"
import {Switch} from "@/components/ui/switch"
import {useAdminUsers} from "@/contexts/admin-users-context"
import type {CreateUserRequest} from "@/lib/services/admin"

const emptyForm: CreateUserRequest = {
  username: "",
  password: "",
  nickname: "",
  email: "",
  is_active: true,
  is_admin: false,
}

export function CreateUserModal({
  isOpen,
  onClose,
}: {
  isOpen: boolean
  onClose: () => void
}) {
  const { createUser } = useAdminUsers()
  const [form, setForm] = useState<CreateUserRequest>(emptyForm)
  const [saving, setSaving] = useState(false)
  const [errors, setErrors] = useState<Record<string, string>>({})

  useEffect(() => {
    if (isOpen) {
      setForm(emptyForm)
      setErrors({})
    } else {
      setSaving(false)
    }
  }, [isOpen])

  const validate = () => {
    const newErrors: Record<string, string> = {}
    if (!form.username.trim()) {
      newErrors.username = "用户名不能为空"
    } else if (form.username.trim().length < 3) {
      newErrors.username = "用户名长度不能少于 3 位"
    }

    if (!form.email.trim()) {
      newErrors.email = "邮箱不能为空"
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email.trim())) {
      newErrors.email = "邮箱格式不正确"
    }

    if (!form.password) {
      newErrors.password = "密码不能为空"
    } else if (form.password.length < 8) {
      newErrors.password = "密码长度不能少于 8 位"
    }

    setErrors(newErrors)
    return Object.keys(newErrors).length === 0
  }

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!validate()) return

    setSaving(true)
    try {
      await createUser({
        ...form,
        username: form.username.trim(),
        nickname: form.nickname?.trim() || undefined,
        email: form.email.trim(),
      })
      onClose()
    } catch {
      // Errors are already handled by context toast notifications
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>新增用户</DialogTitle>
          <DialogDescription>
            直接创建一个本地密码登录的新账户。
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSave} className="space-y-4 pt-2">
          <div className="space-y-1.5">
            <Label htmlFor="username">用户名</Label>
            <Input
              id="username"
              value={form.username}
              onChange={(e) => setForm((prev) => ({ ...prev, username: e.target.value }))}
              placeholder="请输入用户名 (至少 3 位)"
            />
            {errors.username && (
              <p className="text-xs text-destructive">{errors.username}</p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="nickname">昵称 (选填)</Label>
            <Input
              id="nickname"
              value={form.nickname}
              onChange={(e) => setForm((prev) => ({ ...prev, nickname: e.target.value }))}
              placeholder="请输入昵称"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="email">邮箱</Label>
            <Input
              id="email"
              type="email"
              value={form.email}
              onChange={(e) => setForm((prev) => ({ ...prev, email: e.target.value }))}
              placeholder="请输入邮箱地址"
            />
            {errors.email && (
              <p className="text-xs text-destructive">{errors.email}</p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="password">密码</Label>
            <Input
              id="password"
              type="password"
              value={form.password}
              onChange={(e) => setForm((prev) => ({ ...prev, password: e.target.value }))}
              placeholder="请输入登录密码 (至少 8 位)"
            />
            {errors.password && (
              <p className="text-xs text-destructive">{errors.password}</p>
            )}
          </div>

          <div className="flex items-center justify-between rounded-lg border border-dashed p-3 bg-muted/10">
            <div>
              <div className="font-medium text-sm">启用账户</div>
              <div className="text-xs text-muted-foreground">禁用后此账号将无法登录系统。</div>
            </div>
            <Switch
              checked={form.is_active}
              onCheckedChange={(checked) => setForm((prev) => ({ ...prev, is_active: checked }))}
            />
          </div>

          <div className="flex items-center justify-between rounded-lg border border-dashed p-3 bg-muted/10">
            <div>
              <div className="font-medium text-sm">管理员权限</div>
              <div className="text-xs text-muted-foreground">开启后此账号将拥有后台管理权限。</div>
            </div>
            <Switch
              checked={form.is_admin}
              onCheckedChange={(checked) => setForm((prev) => ({ ...prev, is_admin: checked }))}
            />
          </div>

          <div className="flex justify-end gap-2 pt-2 border-t mt-2">
            <Button variant="outline" type="button" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" disabled={saving} variant="secondary">
              {saving ? "创建中..." : "创建"}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
