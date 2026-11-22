import type { Metadata } from 'next';
import { RootProvider } from 'fumadocs-ui/provider/next';
import './global.css';

export const metadata: Metadata = {
  metadataBase: new URL('https://openflare.cc'),
  title: {
    default: 'OpenFlare 文档',
    template: '%s | OpenFlare',
  },
  description:
    'OpenFlare 分布式反向代理与边缘安全网关文档，涵盖部署、配置、WAF、隧道及架构设计。',
  icons: {
    icon: [
      { url: '/favicon.ico', sizes: 'any' },
      { url: '/brand/openflare-icon.svg', type: 'image/svg+xml' },
    ],
  },
  openGraph: {
    type: 'website',
    siteName: 'OpenFlare',
    url: 'https://openflare.cc/',
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="zh-Hans" suppressHydrationWarning>
      <body>
        <RootProvider search={{ options: { type: 'static' } }}>
          {children}
        </RootProvider>
      </body>
    </html>
  );
}
