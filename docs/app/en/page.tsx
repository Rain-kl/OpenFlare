import type { Metadata } from 'next';
import { LandingPage } from '@/components/landing-page';

export const metadata: Metadata = {
  title: 'Open-Source Self-Hosted Distributed Edge Network',
  description:
    'Orchestrate your own servers into a unified, high-performance edge cluster. Manage reverse proxies, SmartDNS traffic steering, edge caching, WAF & PoW defense, encrypted tunnels, and Pages hosting from one control plane.',
};

export default function EnglishHomePage() {
  return <LandingPage locale="en" />;
}
