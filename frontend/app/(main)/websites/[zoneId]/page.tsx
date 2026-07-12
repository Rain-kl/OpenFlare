import {Suspense} from 'react'
import {Globe} from 'lucide-react'
import dynamic from 'next/dynamic'
import {LoadingStateWithBorder} from '@/components/layout/loading'

const ZonePageClient = dynamic(
  () => import('./page-client').then((mod) => mod.ZonePageClient),
  {
    ssr: false,
    loading: () => (
      <div className="py-6 px-1">
        <LoadingStateWithBorder icon={Globe} description="加载 Zone 详情中..." />
      </div>
    ),
  }
)

export default async function ZonePage({params}: PageProps<'/websites/[zoneId]'>) {
  const {zoneId} = await params
  return (
    <Suspense fallback={
      <div className="py-6 px-1">
        <LoadingStateWithBorder icon={Globe} description="加载 Zone 详情中..." />
      </div>
    }>
      <ZonePageClient zoneId={Number(zoneId)} />
    </Suspense>
  )
}
