import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
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
  {id: 1, domain: 'arctel.de', created_at: '', updated_at: ''},
];

const domains: ZoneDomainItem[] = [
  {
    id: 7,
    zone_id: 1,
    proxy_route_id: null,
    domain: 'api.arctel.de',
    cert_id: 9,
    created_at: '',
    updated_at: '',
  },
  {
    id: 8,
    zone_id: 1,
    proxy_route_id: 99,
    domain: 'bound.arctel.de',
    cert_id: null,
    created_at: '',
    updated_at: '',
  },
];

function renderSelector(ui: React.ReactElement) {
  const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

describe('ZoneDomainSelector', () => {
  it('renders domains and toggles selection', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();

    renderSelector(
      <ZoneDomainSelector
        value={[7]}
        onChange={onChange}
        domains={domains}
        zones={zones}
      />,
    );

    expect(screen.getByText('api.arctel.de')).toBeVisible();
    // Bound to another route — hidden by default
    expect(screen.queryByText('bound.arctel.de')).not.toBeInTheDocument();
    expect(screen.getByRole('button', {name: /快捷新增域名/})).toBeVisible();

    await user.click(screen.getByText('api.arctel.de'));
    expect(onChange).toHaveBeenCalledWith([]);
  });

  it('hides domains bound to another route', () => {
    renderSelector(
      <ZoneDomainSelector
        value={[]}
        onChange={vi.fn()}
        domains={domains}
        zones={zones}
        currentRouteId={1}
      />,
    );

    expect(screen.getByText('api.arctel.de')).toBeVisible();
    expect(screen.queryByText('bound.arctel.de')).not.toBeInTheDocument();
  });

  it('still shows domains already bound to the current route', () => {
    renderSelector(
      <ZoneDomainSelector
        value={[8]}
        onChange={vi.fn()}
        domains={domains}
        zones={zones}
        currentRouteId={99}
      />,
    );

    expect(screen.getByText('bound.arctel.de')).toBeVisible();
    expect(screen.getByText('api.arctel.de')).toBeVisible();
  });
});
