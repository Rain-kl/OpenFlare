import * as React from 'react';
import { cn } from '@/lib/utils';
import Link from 'next/link';
import { LucideIcon } from 'lucide-react';

// GitHub 品牌图标（lucide-react 1.x 移除了品牌图标，这里用内联 SVG 替代）
const GithubIcon = React.forwardRef<SVGSVGElement, React.ComponentProps<'svg'>>(
  (props, ref) => (
    <svg
      ref={ref}
      viewBox='0 0 24 24'
      fill='currentColor'
      aria-hidden='true'
      {...props}
    >
      <path d='M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12' />
    </svg>
  ),
);
GithubIcon.displayName = 'GithubIcon';

export interface FooterSectionProps {
  className?: string;
}

/**
 * Footer Section - 页脚
 */
export const FooterSection = React.memo(function FooterSection({
  className,
}: FooterSectionProps) {
  return (
    <footer
      className={cn(
        'relative z-10 w-full bg-transparent border-t border-white/10 mt-0 backdrop-blur-sm',
        className,
      )}
    >
      <div className='container mx-auto max-w-7xl px-6 py-20 lg:py-32'>
        <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-12 lg:gap-8 mb-20'>
          <div className='lg:col-span-2 space-y-6'>
            <Link href='/' className='flex items-center gap-2'>
              <div className='w-10 h-10 p-2 rounded bg-primary text-sm text-primary-foreground flex items-center justify-center font-bold'>
                OF
              </div>
              <span className='text-2xl font-bold tracking-tight text-foreground'>
                OpenFlare
              </span>
            </Link>
            <p className='text-muted-foreground text-base leading-relaxed max-w-sm'>
              开源边缘节点与反向代理管理平台。基于 Go + React 构建，统一管理
              OpenResty 节点、网站规则与 WAF 配置。
            </p>
            <div className='flex gap-4 pt-2'>
              <SocialLink
                icon={GithubIcon}
                href='https://github.com/Rain-kl/OpenFlare/'
              />
            </div>
          </div>

          <div className='lg:col-span-1'>
            <h3 className='font-semibold text-foreground mb-6'>产品</h3>
            <ul className='space-y-4 text-sm text-muted-foreground'>
              <li>
                <FooterLink href='/'>仪表盘</FooterLink>
              </li>
              <li>
                <FooterLink href='/settings'>个人设置</FooterLink>
              </li>
            </ul>
          </div>

          <div className='lg:col-span-1'>
            <h3 className='font-semibold text-foreground mb-6'>开发</h3>
            <ul className='space-y-4 text-sm text-muted-foreground'>
              <li>
                <FooterLink href='https://openflare.fyrn.link/'>
                  使用文档
                </FooterLink>
              </li>
              <li>
                <FooterLink href='https://github.com/Rain-kl/OpenFlare'>
                  源代码
                </FooterLink>
              </li>
            </ul>
          </div>

          <div className='lg:col-span-1'>
            <h3 className='font-semibold text-foreground mb-6'>社区</h3>
            <ul className='space-y-4 text-sm text-muted-foreground'>
              <li>
                <FooterLink href='https://github.com/Rain-kl/OpenFlare/issues'>
                  GitHub Issues
                </FooterLink>
              </li>
              <li>
                <FooterLink href='https://github.com/Rain-kl/OpenFlare/discussions'>
                  讨论
                </FooterLink>
              </li>
            </ul>
          </div>
        </div>

        <div className='pt-8 border-t border-border flex flex-col md:flex-row justify-between items-center gap-4 text-sm text-muted-foreground'>
          <p>© 2026 Modern Platform. All rights reserved.</p>
          <div className='flex gap-8'>
            <Link
              href='/docs/privacy-policy'
              className='hover:text-foreground transition-colors'
            >
              隐私政策
            </Link>
            <Link
              href='/docs/terms-of-service'
              className='hover:text-foreground transition-colors'
            >
              服务条款
            </Link>
          </div>
        </div>
      </div>

      <div className='absolute bottom-0 left-0 w-full overflow-hidden pointer-events-none opacity-[0.02]'>
        <div className='text-[12vw] 2xl:text-[180px] font-black leading-none text-foreground whitespace-nowrap select-none text-center transform translate-y-1/3 transition-all duration-700'>
          Modern Platform
        </div>
      </div>
    </footer>
  );
});

function SocialLink({ icon: Icon, href }: { icon: LucideIcon; href: string }) {
  return (
    <Link
      href={href}
      className='w-10 h-10 rounded-full bg-muted/50 flex items-center justify-center text-muted-foreground hover:bg-primary hover:text-primary-foreground transition-all duration-300'
    >
      <Icon className='w-5 h-5' />
    </Link>
  );
}

function FooterLink({
  href,
  children,
}: {
  href: string;
  children: React.ReactNode;
}) {
  return (
    <Link
      href={href}
      className='hover:text-foreground transition-colors flex items-center group'
    >
      <span className='relative'>
        {children}
        <span className='absolute left-0 -bottom-0.5 w-0 h-px bg-foreground transition-all duration-300 group-hover:w-full' />
      </span>
    </Link>
  );
}
