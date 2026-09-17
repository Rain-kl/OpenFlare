'use client';

import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { motion } from 'motion/react';
import { ArrowLeft, Home } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useTranslations } from 'next-intl';

/** Full-page 403 UI, used by /403 and Next.js forbidden(). */
export function ForbiddenPage() {
  const router = useRouter();
  const t = useTranslations('forbidden');

  return (
    <div className='relative min-h-screen w-full flex flex-col items-center justify-center bg-background overflow-hidden selection:bg-primary/20'>
      <div className='absolute inset-0 -z-10 overflow-hidden'>
        <div className='absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 size-[500px] bg-primary/20 rounded-full blur-[120px] opacity-20 animate-pulse' />
      </div>

      <div className='container px-6 flex flex-col items-center text-center z-10'>
        <motion.div
          initial={{ opacity: 0, scale: 0.95 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={{ duration: 0.8, ease: 'easeOut' }}
          className='relative mb-6'
        >
          <h1 className='text-[12rem] md:text-[16rem] font-bold leading-none tracking-tighter text-transparent bg-clip-text bg-gradient-to-b from-foreground/10 to-foreground/5 select-none'>
            403
          </h1>
          <div className='absolute inset-0 flex items-center justify-center'>
            <p className='text-2xl font-medium tracking-[0.2em] text-foreground/80 uppercase'>
              {t('label')}
            </p>
          </div>
        </motion.div>

        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.2, duration: 0.8 }}
          className='flex flex-col gap-8'
        >
          <p className='text-muted-foreground max-w-[400px] mx-auto text-sm leading-relaxed'>
            {t('description')}
          </p>

          <div className='flex justify-center gap-4'>
            <Button
              variant='secondary'
              size='sm'
              onClick={() => router.back()}
              className='rounded-full w-24 text-xs border-foreground/10 hover:bg-foreground/10 transition-all duration-300'
            >
              <ArrowLeft className='size-3 opacity-70' />
              {t('previousPage')}
            </Button>

            <Link href='/home'>
              <Button
                variant='default'
                size='sm'
                className='rounded-full w-24 text-xs hover:bg-primary/80 transition-all duration-300'
              >
                <Home className='size-3' />
                {t('home')}
              </Button>
            </Link>
          </div>
        </motion.div>
      </div>

      <p className='absolute bottom-8 text-xs text-muted-foreground/30 font-mono'>
        ERR_HTTP_FORBIDDEN
      </p>
    </div>
  );
}
