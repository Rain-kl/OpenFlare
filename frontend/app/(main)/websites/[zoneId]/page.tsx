import {Suspense} from 'react'
import {Globe} from 'lucide-react'
import {LoadingStateWithBorder} from '@/components/layout/loading'
import {ZonePageClient} from './page-client'

export async function generateStaticParams() {
  return [{zoneId: '1'}];
}

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
