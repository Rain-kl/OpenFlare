import { Suspense } from 'react';
import { SmartDNSPolicyEditor } from './policy-editor';

export default function Page() {
  return (
    <Suspense>
      <SmartDNSPolicyEditor />
    </Suspense>
  );
}
