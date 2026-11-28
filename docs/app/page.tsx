import type { Metadata } from 'next';
import { LandingPage } from '@/components/landing-page';

export const metadata: Metadata = {
  title: '开源自托管的分布式边缘网络 | OpenFlare',
  description:
    'OpenFlare 将分散在各地的服务器编排为统一的高性能边缘集群。在一个控制台内完成反向代理、SmartDNS 智能调度、边缘缓存、WAF 与 PoW 防护、内网穿透及 Pages 静态托管，路由自主，数据可控。',
};

export default function HomePage() {
  return <LandingPage locale="zh" />;
}
