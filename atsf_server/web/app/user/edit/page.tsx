import { LegacyRouteRedirect } from '@/features/shared/components/legacy-route-redirect';

export default function LegacyEditCurrentUserRoute() {
  return <LegacyRouteRedirect href='/users' />;
}
