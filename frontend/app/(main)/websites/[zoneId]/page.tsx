import {ZonePageClient} from './page-client'

export default async function ZonePage({params}: PageProps<'/websites/[zoneId]'>) {
  const {zoneId} = await params
  return <ZonePageClient zoneId={Number(zoneId)} />
}
