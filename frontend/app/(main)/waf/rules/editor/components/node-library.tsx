import { Ban, Fingerprint, Globe2, ShieldCheck } from 'lucide-react';

import { Button } from '@/components/ui/button';

import {
  NODE_TYPE_LABELS,
  WAF_NODE_DRAG_MIME,
  type AddableNodeType,
} from './node-factory';

const items = [
  { type: 'ip_match' as const, icon: Fingerprint },
  { type: 'geo_match' as const, icon: Globe2 },
  { type: 'pow' as const, icon: ShieldCheck },
  { type: 'block' as const, icon: Ban },
] satisfies { type: AddableNodeType; icon: typeof Fingerprint }[];

export function NodeLibrary() {
  return (
    <div className='flex items-center gap-2'>
      {items.map(({ type, icon: Icon }) => (
        <Button
          key={type}
          type='button'
          variant='outline'
          size='sm'
          draggable
          className='cursor-grab active:cursor-grabbing'
          onDragStart={(event) => {
            event.dataTransfer.setData(WAF_NODE_DRAG_MIME, type);
            event.dataTransfer.setData('text/plain', type);
            event.dataTransfer.effectAllowed = 'copy';
          }}
        >
          <Icon data-icon='inline-start' />
          {NODE_TYPE_LABELS[type]}
        </Button>
      ))}
    </div>
  );
}
