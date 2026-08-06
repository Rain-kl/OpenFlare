import { ImageResponse } from 'next/og';

export const dynamic = 'force-static';

export const size = {
  width: 32,
  height: 32,
};
export const contentType = 'image/png';

export default function Icon() {
  return new ImageResponse(
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
      {/* lucide-react 1.x 将图标标记为客户端组件，无法在 ImageResponse 服务端渲染，改用内联 SVG */}
      <svg
        width={20}
        height={20}
        viewBox='0 0 24 24'
        fill='none'
        stroke='white'
        strokeWidth={2}
        strokeLinecap='round'
        strokeLinejoin='round'
      >
        <path d='M2 12q2.5 2 5 0t5 0 5 0 5 0' />
        <path d='M2 19q2.5 2 5 0t5 0 5 0 5 0' />
        <path d='M2 5q2.5 2 5 0t5 0 5 0 5 0' />
      </svg>
    </div>,
    {
      ...size,
    },
  );
}
