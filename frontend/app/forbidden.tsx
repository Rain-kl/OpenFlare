'use client';

import { ForbiddenPage } from '@/components/layout/forbidden-page';

/** Next.js convention: rendered when forbidden() is called. */
export default function Forbidden() {
  return <ForbiddenPage />;
}
