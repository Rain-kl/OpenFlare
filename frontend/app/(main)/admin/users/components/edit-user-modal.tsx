'use client';

import {useEffect, useState} from 'react';
import {Button} from '@/components/ui/button';
import {Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle,} from '@/components/ui/dialog';
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {Switch} from '@/components/ui/switch';
import {useAdminUsers} from '@/contexts/admin-users-context';
import {useAuth} from '@/components/providers/auth-provider';
import type {AdminUser} from '@/lib/services/admin';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import {Loader2} from 'lucide-react';

interface EditUserForm {
  nickname: string;
  email: string;
  is_admin: boolean;
  password?: string;
}

export function EditUserModal({
  isOpen,
  onClose,
  user,
}: {
  isOpen: boolean;
  onClose: () => void;
  user: AdminUser | null;
}) {
  const { updateUser, deleteUser, getUserDetail } = useAdminUsers();
  const { user: currentUser } = useAuth();

  const isSelf =
    currentUser && user && currentUser.id.toString() === user.id.toString();

  const [form, setForm] = useState<EditUserForm>({
    nickname: '',
    email: '',
    is_admin: false,
    password: '',
  });
  const [saving, setSaving] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

  useEffect(() => {
    let active = true;
    if (isOpen && user) {
      setForm({
        nickname: user.nickname || '',
        email: user.email || '',
        is_admin: user.is_admin || false,
        password: '',
      });
      setErrors({});

      getUserDetail(user.id)
        .then((detail) => {
          if (active && detail) {
            setForm({
              nickname: detail.nickname || '',
              email: detail.email || '',
              is_admin: detail.is_admin || false,
              password: '',
            });
          }
        })
        .catch(() => {
          // ignore fetching error
        });
    } else {
      setSaving(false);
    }
    return () => {
      active = false;
    };
  }, [isOpen, user, getUserDetail]);

  const validate = () => {
    const newErrors: Record<string, string> = {};

    if (!form.email.trim()) {
      newErrors.email = '邮箱不能为空';
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email.trim())) {
      newErrors.email = '邮箱格式不正确';
    }

    if (form.password && form.password.length < 8) {
      newErrors.password = '密码长度不能少于 8 位';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!user || !validate()) return;

    setSaving(true);
    try {
      await updateUser(user.id, {
        nickname: form.nickname.trim() || undefined,
        email: form.email.trim(),
        is_admin: form.is_admin,
        password: form.password?.trim() || undefined,
      });
      onClose();
    } catch {
      // Errors are handled by context toast notifications
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!user) return;
    setSaving(true);
    try {
      await deleteUser(user);
      onClose();
    } catch {
      // Errors are handled by context toast notifications
    } finally {
      setSaving(false);
      setShowDeleteConfirm(false);
    }
  };

  if (!user) return null;

  return (
    <>
      <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
        <DialogContent className='max-w-md'>
          <DialogHeader>
            <DialogTitle>编辑用户</DialogTitle>
            <DialogDescription>
              修改用户档案、授予/撤销管理权限或重置其密码。
            </DialogDescription>
          </DialogHeader>

          <form onSubmit={handleSave} className='space-y-4 pt-2'>
            <div className='space-y-1.5'>
              <Label
                htmlFor='username'
                className='text-xs text-muted-foreground'
              >
                用户名 (不可修改)
              </Label>
              <Input
                id='username'
                value={user.username}
                disabled
                className='bg-muted/50 cursor-not-allowed select-none font-mono'
              />
            </div>

            <div className='space-y-1.5'>
              <Label htmlFor='nickname'>昵称 (选填)</Label>
              <Input
                id='nickname'
                value={form.nickname}
                onChange={(e) =>
                  setForm((prev) => ({ ...prev, nickname: e.target.value }))
                }
                placeholder='请输入昵称'
              />
            </div>

            <div className='space-y-1.5'>
              <Label htmlFor='email'>邮箱</Label>
              <Input
                id='email'
                type='email'
                value={form.email}
                onChange={(e) =>
                  setForm((prev) => ({ ...prev, email: e.target.value }))
                }
                placeholder='请输入邮箱地址'
              />
              {errors.email && (
                <p className='text-xs text-destructive'>{errors.email}</p>
              )}
            </div>

            <div className='space-y-1.5'>
              <Label htmlFor='password'>重置密码 (选填)</Label>
              <Input
                id='password'
                type='password'
                value={form.password}
                onChange={(e) =>
                  setForm((prev) => ({ ...prev, password: e.target.value }))
                }
                placeholder='留空表示保持当前密码，输入则重置 (至少 8 位)'
              />
              {errors.password && (
                <p className='text-xs text-destructive'>{errors.password}</p>
              )}
            </div>

            <div className='flex items-center justify-between rounded-lg border border-dashed p-3 bg-muted/10'>
              <div>
                <div className='font-medium text-sm'>管理员权限</div>
                <div className='text-xs text-muted-foreground'>
                  {isSelf
                    ? '不能撤销当前登录用户的管理员权限。'
                    : '开启后此账号将拥有后台管理权限。'}
                </div>
              </div>
              <Switch
                checked={form.is_admin}
                disabled={!!isSelf || saving}
                onCheckedChange={async (checked) => {
                  setForm((prev) => ({ ...prev, is_admin: checked }));
                  setSaving(true);
                  try {
                    await updateUser(user.id, {
                      nickname: form.nickname.trim() || undefined,
                      email: form.email.trim(),
                      is_admin: checked,
                    });
                  } catch {
                    setForm((prev) => ({ ...prev, is_admin: !checked }));
                  } finally {
                    setSaving(false);
                  }
                }}
              />
            </div>

            <div className='flex justify-between items-center pt-2 border-t mt-2'>
              <div>
                {!user.is_admin && (
                  <Button
                    type='button'
                    variant='destructive'
                    onClick={() => setShowDeleteConfirm(true)}
                    disabled={saving}
                  >
                    删除用户
                  </Button>
                )}
              </div>
              <div className='flex gap-2'>
                <Button
                  variant='outline'
                  type='button'
                  onClick={onClose}
                  disabled={saving}
                >
                  取消
                </Button>
                <Button type='submit' disabled={saving} variant='secondary'>
                  {saving ? '保存中...' : '保存'}
                </Button>
              </div>
            </div>
          </form>
        </DialogContent>
      </Dialog>

      <AlertDialog open={showDeleteConfirm} onOpenChange={setShowDeleteConfirm}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除用户</AlertDialogTitle>
            <AlertDialogDescription>
              确定要删除用户 {user.nickname || user.username}{' '}
              吗？该操作会移除用户账号，删除后无法撤销。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={saving}>取消</AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => {
                e.preventDefault();
                handleDelete();
              }}
              disabled={saving}
            >
              {saving && <Loader2 className='size-3 animate-spin mr-1' />}
              确认删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
