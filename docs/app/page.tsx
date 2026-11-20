import type { Metadata } from 'next';
import { LandingPage } from '@/components/landing-page';

export const metadata: Metadata = {
  title: '分布式边缘安全网关',
  description:
    '使用 OpenFlare 统一管理 Pingora Edge 集群、发布站点配置，并在边缘执行 WAF 与 PoW 防护。',
};

export default function HomePage() {
  return <LandingPage locale="zh" />;
}
