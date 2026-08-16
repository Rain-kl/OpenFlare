'use client';

import * as React from 'react';
import Link from 'next/link';
import { motion, useAnimation } from 'motion/react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb';
import { useUser } from '@/contexts/user-context';
import {
  ArrowRight,
  BookOpen,
  Camera,
  Edit,
  Globe,
  Info,
  Link2,
  Loader2,
  Lock,
  Mail,
  MapPin,
  Phone,
  Shield,
  Unlink,
  User as UserIcon,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Separator } from '@/components/ui/separator';
import { Textarea } from '@/components/ui/textarea';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Label } from '@/components/ui/label';
import type {
  ChangePasswordRequest,
  UpdateProfileRequest,
} from '@/lib/services/auth';
import { AuthService } from '@/lib/services/auth';
import services from '@/lib/services';
import {
  ImageCrop,
  ImageCropApply,
  ImageCropContent,
  ImageCropReset,
} from '@/components/ui/image-crop';
import { toast } from 'sonner';
import { useTranslations } from 'next-intl';

export function ProfileMain() {
  const { user, loading, refetch } = useUser();
  const controls = useAnimation();
  const t = useTranslations('settings.profile');
  const isAnimatingRef = React.useRef(false);
  const queryClient = useQueryClient();

  // 修改密码 State
  const [oldPassword, setOldPassword] = React.useState('');
  const [newPassword, setNewPassword] = React.useState('');
  const [confirmPassword, setConfirmPassword] = React.useState('');

  // 编辑个人资料 State
  const [isEditDialogOpen, setIsEditDialogOpen] = React.useState(false);
  const [nickname, setNickname] = React.useState('');
  const [email, setEmail] = React.useState('');
  const [bio, setBio] = React.useState('');
  const [phone, setPhone] = React.useState('');
  const [gender, setGender] = React.useState('secret');
  const [website, setWebsite] = React.useState('');
  const [location, setLocation] = React.useState('');
  const [avatarUrl, setAvatarUrl] = React.useState('');

  // 头像裁剪 State
  const [cropFile, setCropFile] = React.useState<File | null>(null);
  const [isCropDialogOpen, setIsCropDialogOpen] = React.useState(false);
  const fileInputRef = React.useRef<HTMLInputElement>(null);

  // 初始化编辑表单数据
  React.useEffect(() => {
    if (user) {
      setNickname(user.nickname || '');
      setEmail(user.email || '');
      setBio(user.bio || '');
      setPhone(user.phone || '');
      setGender(user.gender || 'secret');
      setWebsite(user.website || '');
      setLocation(user.location || '');
      setAvatarUrl(user.avatar_url || '');
    }
  }, [user, isEditDialogOpen]);

  const changePasswordMutation = useMutation({
    mutationFn: (req: ChangePasswordRequest) => AuthService.changePassword(req),
    onSuccess: () => {
      toast.success(t('passwordChangeSuccess'));
      setOldPassword('');
      setNewPassword('');
      setConfirmPassword('');
      void refetch();
    },
    onError: (error: Error) => {
      toast.error(error.message || t('passwordChangeFailed'));
    },
  });

  const handlePasswordChange = (e: React.FormEvent) => {
    e.preventDefault();
    if (newPassword !== confirmPassword) {
      toast.error(t('passwordMismatch'));
      return;
    }
    if (newPassword.length < 8) {
      toast.error(t('passwordTooShort'));
      return;
    }
    changePasswordMutation.mutate({
      old_password: oldPassword,
      new_password: newPassword,
    });
  };

  const updateProfileMutation = useMutation({
    mutationFn: (req: UpdateProfileRequest) => AuthService.updateProfile(req),
    onSuccess: () => {
      toast.success(t('profileUpdateSuccess'));
      setIsEditDialogOpen(false);
      void refetch();
    },
    onError: (error: Error) => {
      toast.error(error.message || t('profileUpdateFailed'));
    },
  });

  const handleProfileSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    updateProfileMutation.mutate({
      nickname,
      email,
      avatar_url: avatarUrl,
      bio,
      phone,
      gender,
      website,
      location,
    });
  };

  const externalAccountBindingsQuery = useQuery({
    queryKey: ['auth', 'external-accounts'],
    queryFn: () => AuthService.getExternalAccountBindings(),
  });

  const publicAuthSourcesQuery = useQuery({
    queryKey: ['auth', 'public-sources'],
    queryFn: () => AuthService.getAuthSources(),
  });

  const bindSourceMutation = useMutation({
    mutationFn: async (sourceName: string) => {
      const { authorize_url } = await AuthService.getAuthorizeUrl(
        sourceName,
        'bind',
      );
      sessionStorage.setItem(
        'redirect_after_login',
        `${window.location.pathname}${window.location.search}`,
      );
      window.location.href = authorize_url;
    },
    onError: (error: Error) => {
      toast.error(error.message || t('bindSourceFailed'));
    },
  });

  const handleAvatarClick = () => {
    if (isAnimatingRef.current) return;

    isAnimatingRef.current = true;
    controls.start({
      rotate: [0, -20, 20, -20, 20, 0],
      transition: { duration: 0.5, ease: 'easeInOut' },
    });

    setTimeout(() => {
      isAnimatingRef.current = false;
    }, 650);
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      setCropFile(file);
      setIsCropDialogOpen(true);
    }
  };

  const handleCroppedImage = async (croppedBase64: string) => {
    try {
      const res = await services.upload.uploadBase64Image(
        croppedBase64,
        'avatar',
        'avatar.png',
      );
      setAvatarUrl(`/f/${res.id}`);
      setIsCropDialogOpen(false);
      setCropFile(null);
      toast.success(t('avatarUploadSuccess'));
    } catch (err) {
      toast.error((err as Error).message || t('avatarUploadFailed'));
    }
  };

  const getGenderLabel = (g?: string) => {
    switch (g) {
      case 'male':
        return t('genderMale');
      case 'female':
        return t('genderFemale');
      case 'secret':
        return t('genderSecret');
      default:
        return g || t('genderSecret');
    }
  };

  if (loading) {
    return (
      <div className='py-6 space-y-4 max-w-4xl mx-auto'>
        <div className='border-b border-border pb-4'>
          <h1 className='text-2xl font-semibold'>{t('title')}</h1>
        </div>
      </div>
    );
  }

  if (!user) {
    return (
      <div className='py-6 space-y-6 max-w-4xl mx-auto'>
        <div className='text-sm text-muted-foreground'>{t('userNotFound')}</div>
      </div>
    );
  }

  return (
    <div className='py-6 space-y-6 max-w-4xl mx-auto'>
      <div className='font-semibold'>
        <Breadcrumb>
          <BreadcrumbList>
            <BreadcrumbItem>
              <BreadcrumbLink asChild>
                <Link href='/settings' className='text-base text-primary'>
                  {t('breadcrumbSettings')}
                </Link>
              </BreadcrumbLink>
            </BreadcrumbItem>
            <BreadcrumbSeparator />
            <BreadcrumbItem>
              <BreadcrumbPage className='text-base font-semibold'>
                {t('breadcrumbProfile')}
              </BreadcrumbPage>
            </BreadcrumbItem>
          </BreadcrumbList>
        </Breadcrumb>
      </div>

      {/* 基本资料面板 */}
      <div className='space-y-6 bg-card border border-dashed rounded-lg p-6'>
        <div className='border-b pb-4 flex justify-between items-center'>
          <div>
            <h2 className='text-lg font-semibold tracking-tight'>
              {t('basicInfo')}
            </h2>
            <p className='text-xs text-muted-foreground'>
              {t('basicInfoDesc')}
            </p>
          </div>
          <Button
            variant='outline'
            size='sm'
            className='text-xs border-dashed'
            onClick={() => setIsEditDialogOpen(true)}
          >
            <Edit className='size-3.5 mr-1.5' />
            {t('editProfile')}
          </Button>
        </div>

        <div className='flex flex-col sm:flex-row items-center sm:items-start gap-6 pt-2'>
          <motion.div
            animate={controls}
            onClick={handleAvatarClick}
            className='cursor-pointer origin-center shrink-0'
            whileHover={{ scale: 1.05 }}
          >
            <Avatar className='size-20 md:size-24 border-2 border-primary/10 shadow-md'>
              <AvatarImage
                src={user.avatar_url}
                alt={user.nickname || user.username}
              />
              <AvatarFallback className='text-2xl bg-primary text-primary-foreground font-bold'>
                {(user.nickname || user.username).slice(0, 2).toUpperCase()}
              </AvatarFallback>
            </Avatar>
          </motion.div>

          <div className='flex-1 w-full space-y-6'>
            <div className='grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-6'>
              <div className='space-y-1'>
                <div className='text-xs text-muted-foreground flex items-center gap-1.5'>
                  <UserIcon className='size-3 text-muted-foreground/70' />
                  {t('account')}
                </div>
                <div className='text-sm font-semibold'>@{user.username}</div>
              </div>

              <div className='space-y-1'>
                <div className='text-xs text-muted-foreground flex items-center gap-1.5'>
                  <UserIcon className='size-3 text-muted-foreground/70' />
                  {t('nickname')}
                </div>
                <div className='text-sm font-semibold'>
                  {user.nickname || t('notSet')}
                </div>
              </div>

              <div className='space-y-1'>
                <div className='text-xs text-muted-foreground flex items-center gap-1.5'>
                  <Info className='size-3 text-muted-foreground/70' />
                  {t('uid')}
                </div>
                <div className='text-sm font-mono font-semibold'>{user.id}</div>
              </div>

              <div className='space-y-1'>
                <div className='text-xs text-muted-foreground flex items-center gap-1.5'>
                  <Mail className='size-3 text-muted-foreground/70' />
                  {t('email')}
                </div>
                <div className='text-sm font-semibold truncate max-w-[220px]'>
                  {user.email || t('notBound')}
                </div>
              </div>

              <div className='space-y-1'>
                <div className='text-xs text-muted-foreground flex items-center gap-1.5'>
                  <Phone className='size-3 text-muted-foreground/70' />
                  {t('phone')}
                </div>
                <div className='text-sm font-semibold'>
                  {user.phone || t('notSet')}
                </div>
              </div>

              <div className='space-y-1'>
                <div className='text-xs text-muted-foreground flex items-center gap-1.5'>
                  <UserIcon className='size-3 text-muted-foreground/70' />
                  {t('gender')}
                </div>
                <div className='text-sm font-semibold'>
                  {getGenderLabel(user.gender)}
                </div>
              </div>

              <div className='space-y-1'>
                <div className='text-xs text-muted-foreground flex items-center gap-1.5'>
                  <Globe className='size-3 text-muted-foreground/70' />
                  {t('website')}
                </div>
                <div className='text-sm font-semibold truncate max-w-[220px]'>
                  {user.website ? (
                    <a
                      href={
                        user.website.startsWith('http')
                          ? user.website
                          : `http://${user.website}`
                      }
                      target='_blank'
                      rel='noopener noreferrer'
                      className='text-primary hover:underline'
                    >
                      {user.website}
                    </a>
                  ) : (
                    t('notSet')
                  )}
                </div>
              </div>

              <div className='space-y-1'>
                <div className='text-xs text-muted-foreground flex items-center gap-1.5'>
                  <MapPin className='size-3 text-muted-foreground/70' />
                  {t('location')}
                </div>
                <div className='text-sm font-semibold'>
                  {user.location || t('notSet')}
                </div>
              </div>

              <div className='space-y-1'>
                <div className='text-xs text-muted-foreground flex items-center gap-1.5'>
                  <Shield className='size-3 text-muted-foreground/70' />
                  {t('isAdmin')}
                </div>
                <div className='text-sm font-semibold flex items-center gap-1'>
                  {user.is_admin ? (
                    <span className='text-rose-600 flex items-center gap-1'>
                      <Shield className='size-3.5' />
                      {t('yes')}
                    </span>
                  ) : (
                    <span>{t('no')}</span>
                  )}
                </div>
              </div>
            </div>

            <Separator className='border-dashed' />

            <div className='space-y-1'>
              <div className='text-xs text-muted-foreground flex items-center gap-1.5'>
                <BookOpen className='size-3 text-muted-foreground/70' />
                {t('bio')}
              </div>
              <div className='text-sm text-foreground/80 leading-relaxed max-w-2xl bg-muted/20 border border-dashed rounded-lg p-3'>
                {user.bio || t('bioDefault')}
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* 下方左右并排布局容器 */}
      <div className='grid grid-cols-1 md:grid-cols-2 gap-6'>
        {/* 修改密码面板 */}
        <div className='space-y-6 bg-card border border-dashed rounded-lg p-6 flex flex-col justify-between'>
          <div>
            <div className='border-b pb-4 flex items-center gap-2'>
              <div className='p-1.5 rounded-lg bg-amber-500/10 text-amber-500'>
                <Lock className='size-4' />
              </div>
              <div>
                <h2 className='text-base font-semibold tracking-tight'>
                  {t('changePassword')}
                </h2>
                <p className='text-[11px] text-muted-foreground'>
                  {t('changePasswordDesc')}
                </p>
              </div>
            </div>

            {user.need_change_password && (
              <div className='mt-4 rounded-lg border border-amber-500/30 bg-amber-500/5 px-3.5 py-2.5 text-xs text-amber-500 flex items-start gap-2.5'>
                <Info className='size-4 shrink-0 mt-0.5' />
                <div>
                  <p className='font-semibold'>{t('passwordRiskWarning')}</p>
                  <p className='mt-0.5 text-amber-500/80 leading-relaxed font-normal'>
                    {t('mustChangePassword')}
                  </p>
                </div>
              </div>
            )}

            <form onSubmit={handlePasswordChange} className='space-y-3 pt-4'>
              <div className='space-y-1.5'>
                <label className='text-xs font-medium text-muted-foreground'>
                  {t('currentPassword')}
                </label>
                <Input
                  type='password'
                  placeholder={t('currentPasswordPlaceholder')}
                  value={oldPassword}
                  onChange={(e) => setOldPassword(e.target.value)}
                  className='h-8 text-xs rounded-lg'
                  required
                />
              </div>
              <div className='space-y-1.5'>
                <label className='text-xs font-medium text-muted-foreground'>
                  {t('newPassword')}
                </label>
                <Input
                  type='password'
                  placeholder={t('newPasswordPlaceholder')}
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  className='h-8 text-xs rounded-lg'
                  required
                />
              </div>
              <div className='space-y-1.5'>
                <label className='text-xs font-medium text-muted-foreground'>
                  {t('confirmNewPassword')}
                </label>
                <Input
                  type='password'
                  placeholder={t('confirmNewPasswordPlaceholder')}
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  className='h-8 text-xs rounded-lg'
                  required
                />
              </div>

              <div className='pt-2'>
                <Button
                  type='submit'
                  size='sm'
                  className='w-full sm:w-auto h-8 text-xs'
                  disabled={changePasswordMutation.isPending}
                >
                  {changePasswordMutation.isPending
                    ? t('submitting')
                    : t('confirmChange')}
                </Button>
              </div>
            </form>
          </div>
        </div>

        {/* 账号绑定面板 */}
        <div className='space-y-6 bg-card border border-dashed rounded-lg p-6 flex flex-col justify-between'>
          <div>
            <div className='border-b pb-4 flex items-center gap-2'>
              <div className='p-1.5 rounded-lg bg-primary/10 text-primary'>
                <Link2 className='size-4' />
              </div>
              <div>
                <h2 className='text-base font-semibold tracking-tight'>
                  {t('thirdPartyBinding')}
                </h2>
                <p className='text-[11px] text-muted-foreground'>
                  {t('thirdPartyBindingDesc')}
                </p>
              </div>
            </div>

            {/* 已绑定账号列表 */}
            <div className='space-y-2 pt-4'>
              <h3 className='text-[11px] font-semibold text-muted-foreground uppercase tracking-wider'>
                {t('boundAccounts')}
              </h3>
              {externalAccountBindingsQuery.isPending ? (
                <div className='flex items-center justify-center py-4'>
                  <Loader2 className='size-4 animate-spin text-primary' />
                </div>
              ) : (externalAccountBindingsQuery.data ?? []).length > 0 ? (
                <div className='space-y-2'>
                  {(externalAccountBindingsQuery.data ?? []).map((binding) => (
                    <div
                      key={binding.id}
                      className='flex items-center justify-between gap-4 rounded-xl border border-dashed p-3 bg-card hover:bg-muted/10 transition-all duration-300'
                    >
                      <div className='space-y-0.5'>
                        <span className='font-semibold text-xs text-foreground block'>
                          {binding.auth_source_label}
                        </span>
                        <span className='text-[10px] text-muted-foreground font-mono block truncate max-w-[150px]'>
                          {binding.external_username ||
                            binding.email ||
                            t('noAccountIdentifier')}
                        </span>
                      </div>
                      <Button
                        type='button'
                        variant='ghost'
                        size='sm'
                        className='text-[11px] text-muted-foreground hover:text-rose-500 hover:bg-rose-500/10 rounded-lg h-7 px-2 transition-colors'
                        onClick={async () => {
                          await AuthService.deleteExternalAccountBinding(
                            binding.id,
                          );
                          await queryClient.invalidateQueries({
                            queryKey: ['auth', 'external-accounts'],
                          });
                          toast.success(t('bindingRemoved'));
                        }}
                      >
                        <Unlink className='size-3 mr-1' />
                        {t('unbind')}
                      </Button>
                    </div>
                  ))}
                </div>
              ) : (
                <div className='rounded-xl border border-dashed border-border/50 px-4 py-6 text-center text-[11px] text-muted-foreground bg-muted/5 flex flex-col items-center justify-center'>
                  <Link2 className='size-5 text-muted-foreground/30 mb-1' />
                  {t('noBoundAccounts')}
                </div>
              )}
            </div>

            <Separator className='border-dashed my-4' />

            {/* 绑定新账号列表 */}
            <div className='space-y-2'>
              <h3 className='text-[11px] font-semibold text-muted-foreground uppercase tracking-wider'>
                {t('bindNewAccount')}
              </h3>
              {publicAuthSourcesQuery.isPending ? (
                <div className='flex items-center justify-center py-4'>
                  <Loader2 className='size-4 animate-spin text-primary' />
                </div>
              ) : (publicAuthSourcesQuery.data ?? []).length > 0 ? (
                <div className='grid grid-cols-1 gap-2'>
                  {(publicAuthSourcesQuery.data ?? []).map((source) => (
                    <Button
                      key={source.id}
                      type='button'
                      variant='outline'
                      className='flex items-center justify-between w-full border border-dashed rounded-xl px-3 py-2 text-left font-normal text-xs hover:bg-primary/5 hover:text-primary hover:border-primary/30 transition-all duration-300 group h-8'
                      onClick={() => {
                        void bindSourceMutation.mutateAsync(source.name);
                      }}
                    >
                      <div className='flex items-center gap-1.5'>
                        <Link2 className='size-3 text-muted-foreground group-hover:text-primary' />
                        <span>
                          {t('bindSource', {
                            name: source.display_name || source.name,
                          })}
                        </span>
                      </div>
                      <ArrowRight className='size-3 opacity-0 -translate-x-1 group-hover:opacity-100 group-hover:translate-x-0 transition-all text-primary' />
                    </Button>
                  ))}
                </div>
              ) : (
                <div className='text-[11px] text-muted-foreground text-center py-2'>
                  {t('noAvailableSources')}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* 编辑资料 Dialog */}
      <Dialog open={isEditDialogOpen} onOpenChange={setIsEditDialogOpen}>
        <DialogContent className='max-w-md'>
          <DialogHeader>
            <DialogTitle>{t('editProfileTitle')}</DialogTitle>
            <DialogDescription>{t('editProfileDesc')}</DialogDescription>
          </DialogHeader>

          <form onSubmit={handleProfileSubmit} className='space-y-4'>
            {/* 上传头像域 */}
            <div className='flex flex-col items-center gap-2 py-2'>
              <div
                className='group relative cursor-pointer rounded-full border border-dashed hover:border-primary/50 transition-all'
                onClick={() => fileInputRef.current?.click()}
              >
                <Avatar className='size-20 border-2 border-primary/5 shadow-md'>
                  <AvatarImage src={avatarUrl} alt={nickname} />
                  <AvatarFallback className='text-xl bg-primary text-primary-foreground font-bold'>
                    {(nickname || 'U').slice(0, 2).toUpperCase()}
                  </AvatarFallback>
                </Avatar>
                <div className='absolute inset-0 bg-black/40 rounded-full flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity'>
                  <Camera className='size-5 text-white' />
                </div>
              </div>
              <span className='text-[10px] text-muted-foreground'>
                {t('uploadAvatar')}
              </span>
              <input
                type='file'
                ref={fileInputRef}
                accept='image/*'
                className='hidden'
                onChange={handleFileChange}
              />
            </div>

            <div className='grid grid-cols-2 gap-3'>
              <div className='space-y-1'>
                <Label htmlFor='nickname' className='text-xs'>
                  {t('nickname')}
                </Label>
                <Input
                  id='nickname'
                  value={nickname}
                  onChange={(e) => setNickname(e.target.value)}
                  className='h-8 text-xs rounded-lg'
                  placeholder={t('nicknamePlaceholder')}
                  required
                />
              </div>

              <div className='space-y-1'>
                <Label htmlFor='email' className='text-xs'>
                  {t('email')}
                </Label>
                <Input
                  id='email'
                  type='email'
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className='h-8 text-xs rounded-lg'
                  placeholder={t('emailPlaceholder')}
                />
              </div>

              <div className='space-y-1'>
                <Label htmlFor='phone' className='text-xs'>
                  {t('phone')}
                </Label>
                <Input
                  id='phone'
                  value={phone}
                  onChange={(e) => setPhone(e.target.value)}
                  className='h-8 text-xs rounded-lg'
                  placeholder={t('phonePlaceholder')}
                />
              </div>

              <div className='space-y-1'>
                <Label htmlFor='gender' className='text-xs'>
                  {t('gender')}
                </Label>
                <Select value={gender} onValueChange={setGender}>
                  <SelectTrigger
                    id='gender'
                    size='sm'
                    className='w-full text-xs h-8'
                  >
                    <SelectValue placeholder={t('genderPlaceholder')} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='male'>{t('genderMale')}</SelectItem>
                    <SelectItem value='female'>{t('genderFemale')}</SelectItem>
                    <SelectItem value='secret'>{t('genderSecret')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className='space-y-1'>
                <Label htmlFor='website' className='text-xs'>
                  {t('website')}
                </Label>
                <Input
                  id='website'
                  value={website}
                  onChange={(e) => setWebsite(e.target.value)}
                  className='h-8 text-xs rounded-lg'
                  placeholder='https://...'
                />
              </div>

              <div className='space-y-1'>
                <Label htmlFor='location' className='text-xs'>
                  {t('location')}
                </Label>
                <Input
                  id='location'
                  value={location}
                  onChange={(e) => setLocation(e.target.value)}
                  className='h-8 text-xs rounded-lg'
                  placeholder={t('locationPlaceholder')}
                />
              </div>
            </div>

            <div className='space-y-1'>
              <Label htmlFor='bio' className='text-xs'>
                {t('bio')}
              </Label>
              <Textarea
                id='bio'
                value={bio}
                onChange={(e) => setBio(e.target.value)}
                className='text-xs min-h-[60px] rounded-lg'
                placeholder={t('bioPlaceholder')}
              />
            </div>

            <DialogFooter className='pt-2'>
              <Button
                type='button'
                variant='outline'
                size='sm'
                className='h-8 text-xs'
                onClick={() => setIsEditDialogOpen(false)}
              >
                {t('cancel')}
              </Button>
              <Button
                type='submit'
                size='sm'
                className='h-8 text-xs'
                disabled={updateProfileMutation.isPending}
              >
                {updateProfileMutation.isPending
                  ? t('saving')
                  : t('saveChanges')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* 头像裁剪 Dialog */}
      <Dialog open={isCropDialogOpen} onOpenChange={setIsCropDialogOpen}>
        <DialogContent className='max-w-md'>
          <DialogHeader>
            <DialogTitle>{t('cropAvatar')}</DialogTitle>
            <DialogDescription>{t('cropAvatarDesc')}</DialogDescription>
          </DialogHeader>

          {cropFile && (
            <div className='flex flex-col items-center gap-4 py-2'>
              <ImageCrop file={cropFile} aspect={1} onCrop={handleCroppedImage}>
                <ImageCropContent className='max-w-sm rounded-lg border border-dashed' />
                <div className='mt-3 flex justify-center gap-2'>
                  <ImageCropReset />
                  <ImageCropApply />
                </div>
              </ImageCrop>
            </div>
          )}

          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              size='sm'
              className='h-8 text-xs'
              onClick={() => {
                setIsCropDialogOpen(false);
                setCropFile(null);
              }}
            >
              {t('cancel')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
