// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

import { Suspense } from 'react';
import { Loader2 } from 'lucide-react';
import { AdminSettingsPageClient } from './page-client';

export default function AdminSettingsPage() {
  return (
    <Suspense
      fallback={
        <div className='flex items-center justify-center min-h-[400px]'>
          <Loader2 className='size-6 animate-spin text-primary' />
        </div>
      }
    >
      <AdminSettingsPageClient />
    </Suspense>
  );
}
