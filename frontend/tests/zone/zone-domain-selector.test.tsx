import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, expect, it, vi} from 'vitest';

import {ZoneDomainSelector} from '@/app/(main)/proxy-routes/components/zone-domain-selector';
import type {ZoneDomainItem, ZoneItem} from '@/lib/services/openflare';

vi.mock('next/link', () => ({
  default: ({children, href}: {children: React.ReactNode; href: string}) => (
    <a href={href}>{children}</a>
  ),
}));

const zones: ZoneItem[] = [
  {id: 1, domain: 'arctel.de', remark: '', created_at: '', updated_at: ''},
];

const domains: ZoneDomainItem[] = [
  {
    id: 7,
    zone_id: 1,
    proxy_route_id: null,
    domain: 'api.arctel.de',
    cert_id: 9,
    remark: '',
    created_at: '',
    updated_at: '',
  },
  {
    id: 8,
    zone_id: 1,
    proxy_route_id: 99,
    domain: 'bound.arctel.de',
    cert_id: null,
    remark: '',
    created_at: '',
    updated_at: '',
  },
];

describe('ZoneDomainSelector', () => {
  it('renders domains and toggles selection', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();

    render(
      <ZoneDomainSelector
        value={[7]}
        onChange={onChange}
        domains={domains}
        zones={zones}
      />,
    );

    expect(screen.getByText('api.arctel.de')).toBeVisible();
    expect(screen.getByText('bound.arctel.de')).toBeVisible();
    expect(screen.getByText('路由 #99')).toBeVisible();

    // Deselect already selected domain
    await user.click(screen.getByText('api.arctel.de'));
    expect(onChange).toHaveBeenCalledWith([]);
  });

  it('does not select domains bound to another route', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();

    render(
      <ZoneDomainSelector
        value={[]}
        onChange={onChange}
        domains={domains}
        zones={zones}
        currentRouteId={1}
      />,
    );

    await user.click(screen.getByText('bound.arctel.de'));
    expect(onChange).not.toHaveBeenCalled();
  });
});
