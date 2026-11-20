import type { Metadata } from 'next';
import { LandingPage } from '@/components/landing-page';

export const metadata: Metadata = {
  title: 'Distributed edge security gateway',
  description:
    'Manage Pingora Edge nodes, publish site configuration, and apply WAF and proof-of-work protection at the edge with OpenFlare.',
};

export default function EnglishHomePage() {
  return <LandingPage locale="en" />;
}
