import type {NextConfig} from "next";

const isExport = process.env.NEXT_STANDALONE_EXPORT === 'true';

const nextConfig: NextConfig = {
  reactCompiler: true,
  // Prevent 308 redirects on /api/* trailing slashes; dev rewrites proxy legacy APIs as-is.
  skipTrailingSlashRedirect: true,
  experimental: {
  },
  ...(isExport ? { output: 'export' } : {
    async redirects() {
      return [
        { source: '/openflare', destination: '/', permanent: true },
        { source: '/openflare/:path*', destination: '/:path*', permanent: true },
        { source: '/home', destination: '/', permanent: true },
      ];
    },
    async rewrites() {
      const backendUrl = process.env.WAVELET_BACKEND_URL || 'http://localhost:3000';
      return [
        // 上传文件静态资源
        {
          source: '/f/:id',
          destination: `${ backendUrl }/f/:id`,
        },
        // robots.txt 路由代理到后端动态接口
        {
          source: '/robots.txt',
          destination: `${ backendUrl }/robots.txt`,
        },
        // 标准 RESTful API 接口
        {
          source: '/api/:path*',
          destination: `${ backendUrl }/api/:path*`,
        }
      ];
    },
  })
};

export default nextConfig;
