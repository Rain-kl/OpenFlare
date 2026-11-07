'use client';

import { useTranslations } from 'next-intl';
import * as React from 'react';
import { AnimatePresence, motion } from 'motion/react';
import { useUser } from '@/contexts/user-context';
import { Card, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import {
  ArrowRight,
  ExternalLink,
  HelpCircle,
  Layers,
  Shield,
  ShieldCheck,
  Terminal,
  User,
} from 'lucide-react';
import Link from 'next/link';

export function HomeMain() {
  const rt = useTranslations('runtime');

  const { user } = useUser();
  const [isBannerVisible, setIsBannerVisible] = React.useState(true);

  const quickLinks = [
    {
      title: rt('profile_b94d13a4'),
      description: rt(
        'manage_your_account_information_and_preferences_cced83b4',
      ),
      icon: User,
      url: '/settings/profile',
      color: 'text-blue-500',
      bgColor: 'bg-blue-500/10',
      borderColor: 'hover:border-blue-500/30',
    },
    {
      title: rt('user_guide_c83b6b0a'),
      description: rt('api_integration_and_everyday_operation_guides_281269a1'),
      icon: HelpCircle,
      url: 'https://openflare.fyrn.link/',
      color: 'text-purple-500',
      bgColor: 'bg-purple-500/10',
      borderColor: 'hover:border-purple-500/30',
      external: true,
    },
  ];

  const adminLinks = [
    {
      title: rt('system_settings_68ea5dd4'),
      description: rt(
        'manage_registration_login_and_authentication_provider_security_f378ed37',
      ),
      icon: Shield,
      url: '/admin/settings',
      color: 'text-amber-500',
      bgColor: 'bg-amber-500/10',
      borderColor: 'hover:border-amber-500/30',
    },
    {
      title: rt('global_system_configuration_1eb8931e'),
      description: rt('manage_core_platform_runtime_settings_341e4182'),
      icon: ShieldCheck,
      url: '/admin/system',
      color: 'text-indigo-500',
      bgColor: 'bg-indigo-500/10',
      borderColor: 'hover:border-indigo-500/30',
    },
    {
      title: rt('user_permissions_8e8b1d33'),
      description: rt('view_and_manage_registered_users_and_their_db533b7c'),
      icon: User,
      url: '/admin/users',
      color: 'text-rose-500',
      bgColor: 'bg-rose-500/10',
      borderColor: 'hover:border-rose-500/30',
    },
    {
      title: rt('background_task_scheduling_408172d1'),
      description: rt(
        'dispatch_and_monitor_background_and_scheduled_tasks_83967fc9',
      ),
      icon: Layers,
      url: '/admin/tasks',
      color: 'text-teal-500',
      bgColor: 'bg-teal-500/10',
      borderColor: 'hover:border-teal-500/30',
    },
    {
      title: rt('system_logs_366ada1d'),
      description: rt('view_background_task_logs_and_system_status_24fd6a7d'),
      icon: Terminal,
      url: '/admin/logs',
      color: 'text-cyan-500',
      bgColor: 'bg-cyan-500/10',
      borderColor: 'hover:border-cyan-500/30',
    },
  ];

  return (
    <div className='py-6 space-y-8 max-w-6xl mx-auto'>
      {/* 积分收益提示 banner */}
      <AnimatePresence>
        {isBannerVisible && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
            transition={{ duration: 0.3 }}
            className='overflow-hidden'
          >
            <div className='bg-gradient-to-r from-indigo-500/10 via-purple-500/10 to-pink-500/10 border border-dashed border-indigo-500/20 rounded-xl p-6 relative'>
              <div className='max-w-xl space-y-3'>
                <h3 className='text-lg font-bold text-indigo-600 dark:text-indigo-400'>
                  Modern Platform
                </h3>
                <p className='text-xs text-muted-foreground leading-relaxed'>
                  {rt(
                    'a_modern_extensible_administration_platform_with_ready_0d95c0c1',
                  )}
                </p>
                <div className='flex gap-2.5'>
                  <Button
                    size='sm'
                    className='bg-indigo-600 hover:bg-indigo-700 text-xs font-semibold px-4 shadow-sm h-7'
                    asChild
                  >
                    <Link href='/'>{rt('get_started_715ee9dc')}</Link>
                  </Button>
                  <Button
                    size='sm'
                    variant='ghost'
                    className='text-xs font-medium hover:bg-black/5 dark:hover:bg-white/5 h-7'
                    onClick={() => setIsBannerVisible(false)}
                  >
                    {rt('hide_tip_738f4b27')}
                  </Button>
                </div>
              </div>
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* 快捷导航 */}
      <div className='space-y-4'>
        <h2 className='text-lg font-semibold tracking-tight'>
          {rt('quick_navigation_05deefcb')}
        </h2>
        <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4'>
          {quickLinks.map((link, idx) => (
            <motion.div
              key={link.title}
              initial={{ opacity: 0, y: 15 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.4, delay: idx * 0.05 }}
              whileHover={{ y: -4 }}
              className='h-full'
            >
              <Card
                className={`h-full border border-dashed flex flex-col justify-between hover:bg-muted/40 transition-all ${link.borderColor} duration-300 shadow-none`}
              >
                <CardHeader className='p-5'>
                  <div
                    className={`size-10 rounded-lg flex items-center justify-center ${link.bgColor} ${link.color} mb-3`}
                  >
                    <link.icon className='size-5' />
                  </div>
                  <CardTitle className='text-sm font-semibold mb-1 flex items-center gap-1'>
                    {link.title}
                    {link.external && (
                      <ExternalLink className='size-3 text-muted-foreground' />
                    )}
                  </CardTitle>
                  <p className='text-xs text-muted-foreground leading-normal font-normal'>
                    {link.description}
                  </p>
                </CardHeader>
                <div className='px-5 pb-5'>
                  {link.external ? (
                    <Button
                      variant='link'
                      className='p-0 h-auto text-xs text-indigo-500 font-medium'
                      asChild
                    >
                      <Link
                        href={link.url}
                        target='_blank'
                        rel='noopener noreferrer'
                      >
                        {rt('open_fee3b5a5')}
                        <ArrowRight className='size-3 ml-1' />
                      </Link>
                    </Button>
                  ) : (
                    <Button
                      variant='link'
                      className='p-0 h-auto text-xs text-indigo-500 font-medium'
                      asChild
                    >
                      <Link href={link.url}>
                        {rt('open_df42ebc3')}
                        <ArrowRight className='size-3 ml-1' />
                      </Link>
                    </Button>
                  )}
                </div>
              </Card>
            </motion.div>
          ))}
        </div>
      </div>

      {/* 管理面板 (仅管理员可见) */}
      {user?.is_admin && (
        <div className='space-y-4 pt-2'>
          <h2 className='text-lg font-semibold tracking-tight text-rose-500 flex items-center gap-1.5'>
            <Shield className='size-5' />
            {rt('administration_console_ceb0c289')}
          </h2>
          <div className='grid grid-cols-1 md:grid-cols-3 gap-4'>
            {adminLinks.map((link, idx) => (
              <motion.div
                key={link.title}
                initial={{ opacity: 0, y: 15 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.4, delay: (idx + 4) * 0.05 }}
                whileHover={{ y: -4 }}
                className='h-full'
              >
                <Card
                  className={`h-full border border-dashed flex flex-col justify-between hover:bg-muted/40 transition-all ${link.borderColor} duration-300 shadow-none`}
                >
                  <CardHeader className='p-5'>
                    <div
                      className={`size-10 rounded-lg flex items-center justify-center ${link.bgColor} ${link.color} mb-3`}
                    >
                      <link.icon className='size-5' />
                    </div>
                    <CardTitle className='text-sm font-semibold mb-1'>
                      {link.title}
                    </CardTitle>
                    <p className='text-xs text-muted-foreground leading-normal font-normal'>
                      {link.description}
                    </p>
                  </CardHeader>
                  <div className='px-5 pb-5'>
                    <Button
                      variant='link'
                      className='p-0 h-auto text-xs text-indigo-500 font-medium'
                      asChild
                    >
                      <Link href={link.url}>
                        {rt('open_df42ebc3')}
                        <ArrowRight className='size-3 ml-1' />
                      </Link>
                    </Button>
                  </div>
                </Card>
              </motion.div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
