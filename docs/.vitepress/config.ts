import { defineConfig, type HeadConfig, resolveSiteDataByRoute } from 'vitepress'
import llmstxt from 'vitepress-plugin-llms'

const prod = !!process.env.NETLIFY

export default defineConfig({
  title: 'OpenFlare',
  lastUpdated: true,
  cleanUrls: true,
  ignoreDeadLinks: true,
  metaChunk: true,
  srcExclude: [
    'zh/**',
    'components/**',
    'snippets/**',
    'plan/**',
    'guideline/**'
  ],

  markdown: {
    math: true
  },

  sitemap: {
    hostname: 'https://openflare.io'
  },

  head: [
    ['meta', { name: 'theme-color', content: '#10b981' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:site_name', content: 'OpenFlare' }],
    ['meta', { property: 'og:url', content: 'https://openflare.io/' }],
    ['script', { async: '', src: 'https://www.googletagmanager.com/gtag/js?id=G-TBZPQFMLFH' }],
    [
      'script',
      {},
      `window.dataLayer = window.dataLayer || [];
function gtag(){dataLayer.push(arguments);}
gtag('js', new Date());
gtag('config', 'G-TBZPQFMLFH');`
    ]
  ],

  themeConfig: {
    socialLinks: [
      { icon: 'github', link: 'https://github.com/Rain-kl/OpenFlare' }
    ],
    search: {
      provider: 'local'
    }
  },

  locales: {
    root: { label: '简体中文', lang: 'zh-Hans', dir: 'ltr' },
    en: { label: 'English', lang: 'en-US', dir: 'ltr' }
  },

  vite: {
    plugins: [
      prod &&
        llmstxt({
          workDir: '.',
          ignoreFiles: ['index.md']
        })
    ],
    experimental: {
      enableNativePlugin: true
    }
  },

  transformPageData: prod
    ? (pageData, ctx) => {
        const site = resolveSiteDataByRoute(
          ctx.siteConfig.site,
          pageData.relativePath
        )
        const title = `${pageData.title || site.title} | ${
          pageData.description || site.description
        }`
        ;((pageData.frontmatter.head ??= []) as HeadConfig[]).push(
          ['meta', { property: 'og:locale', content: site.lang }],
          ['meta', { property: 'og:title', content: title }]
        )
      }
    : undefined
})
