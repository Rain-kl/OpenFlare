'use client';

import { useTranslations } from 'next-intl';

import { Badge } from '@/components/ui/badge';

const MEASUREMENT_SOURCE_KEYS = new Set([
  'tcp_passive',
  'tcp_active',
  'icmp_active',
  'traceroute',
  'none',
]);

interface MeasurementSourceBadgeProps {
  source?: string;
}

export function MeasurementSourceBadge({
  source,
}: MeasurementSourceBadgeProps) {
  const t = useTranslations('networkTools');
  const clean = source || 'none';
  const label = MEASUREMENT_SOURCE_KEYS.has(clean)
    ? t(`quality.sources.${clean}` as Parameters<typeof t>[0])
    : clean;

  return (
    <Badge variant={clean === 'none' ? 'outline' : 'secondary'}>{label}</Badge>
  );
}
