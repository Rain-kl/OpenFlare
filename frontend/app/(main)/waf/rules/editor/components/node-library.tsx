import {Ban, Fingerprint, Globe2, Plus, ShieldCheck} from 'lucide-react';

import {Button} from '@/components/ui/button';
import type {WAFRuleNode} from '@/lib/services/openflare';

type AddableType = Extract<WAFRuleNode['type'], 'ip_match' | 'geo_match' | 'pow' | 'block'>;
const items = [
  {type: 'ip_match', label: 'IP 匹配', icon: Fingerprint}, {type: 'geo_match', label: '地域匹配', icon: Globe2},
  {type: 'pow', label: 'PoW 挑战', icon: ShieldCheck}, {type: 'block', label: '阻止', icon: Ban},
] satisfies {type: AddableType; label: string; icon: typeof Plus}[];

export function NodeLibrary({onAdd}: {onAdd: (type: AddableType) => void}) {
  return <div className="flex items-center gap-2">{items.map(({type, label, icon: Icon}) => <Button key={type} variant="outline" size="sm" onClick={() => onAdd(type)}><Icon data-icon="inline-start" />{label}</Button>)}</div>;
}
