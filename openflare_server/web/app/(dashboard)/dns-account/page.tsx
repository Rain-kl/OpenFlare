import type { Metadata } from 'next';
import { DnsAccountsPage } from '@/features/dns-accounts/components/dns-accounts-page';

export const metadata: Metadata = {
  title: 'DNS 账号 - OpenFlare',
};

export default function Page() {
  return <DnsAccountsPage />;
}
