import {ImageResponse} from 'next/og'
import {WavesIcon} from 'lucide-react'

export const dynamic = 'force-static'

export const size = {
  width: 32,
  height: 32,
}
export const contentType = 'image/png'

export default function Icon() {
  return new ImageResponse(
    (
      <div
        style={{
          background: 'black',
          width: '100%',
          height: '100%',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          borderRadius: '20%',
        }}
      >
        <WavesIcon color="white" size={20} />
      </div>
    ),
    {
      ...size,
    }
  )
}
