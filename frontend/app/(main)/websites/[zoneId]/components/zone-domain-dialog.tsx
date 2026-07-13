'use client';

import { QuickCreateZoneDomainDialog } from '../../components/quick-create-zone-domain-dialog';

/** Zone 详情页添加域名（固定 Zone，支持简写 / @ / 完整 FQDN）。 */
export function ZoneDomainDialog({
  open,
  onOpenChange,
  zoneId,
  zoneRoot,
  onSaved,
}: {
  open: boolean;
  onOpenChange(open: boolean): void;
  zoneId: number;
  zoneRoot: string;
  onSaved(): Promise<unknown> | void;
}) {
  return (
    <QuickCreateZoneDomainDialog
      open={open}
      onOpenChange={onOpenChange}
      fixedZoneId={zoneId}
      fixedZoneRoot={zoneRoot}
      onCreated={async () => {
        await onSaved();
      }}
    />
  );
}
