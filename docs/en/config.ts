import { defineAdditionalConfig, type DefaultTheme } from 'vitepress'

export default defineAdditionalConfig({
  description:
    'OpenFlare is a lightweight, self-hosted OpenResty control plane for managing reverse proxy rules, configuration publishing, node synchronization, TLS certificates, and basic observability.',

  themeConfig: {
    nav: nav(),

    sidebar: {
      '/en/guide/': { base: '/en/guide/', items: sidebarGuide() },
      '/en/reference/': { base: '/en/reference/', items: sidebarReference() },
      '/en/deployment/': { base: '/en/deployment/', items: sidebarDeployment() },
      '/en/design/': { base: '/en/design/', items: sidebarDesign() },
      '/en/changelog/': { base: '/en/changelog/', items: [] }
    },

    editLink: {
      pattern: 'https://github.com/Rain-kl/OpenFlare/edit/main/docs/:path',
      text: 'Edit this page on GitHub'
    },

    footer: {
      message: 'Released under the Apache License 2.0',
      copyright: 'Copyright © OpenFlare contributors'
    },

    docFooter: {
      prev: 'Previous Page',
      next: 'Next Page'
    },

    outline: {
      label: 'On this page'
    },

    lastUpdated: {
      text: 'Last updated at'
    },

    notFound: {
      title: 'Page Not Found',
      quote: 'This document does not have a corresponding page yet.',
      linkLabel: 'Go to Home',
      linkText: 'Back to OpenFlare Docs'
    },

    langMenuLabel: 'Language',
    returnToTopLabel: 'Back to top',
    sidebarMenuLabel: 'Menu',
    darkModeSwitchLabel: 'Theme',
    lightModeSwitchTitle: 'Switch to light theme',
    darkModeSwitchTitle: 'Switch to dark theme',
    skipToContentLabel: 'Skip to content'
  }
})

function nav(): DefaultTheme.NavItem[] {
  return [
    { text: 'Guide', link: '/en/guide/', activeMatch: '/en/guide/' },
    { text: 'Deployment', link: '/en/deployment/', activeMatch: '/en/deployment/' },
    { text: 'Reference', link: '/en/reference/', activeMatch: '/en/reference/' },
    { text: 'Design', link: '/en/design/', activeMatch: '/en/design/' },
    { text: 'Changelog', link: '/en/changelog/', activeMatch: '/en/changelog/' }
  ]
}

function sidebarGuide(): DefaultTheme.SidebarItem[] {
  return [
    {
      text: 'Guide',
      items: [
        { text: 'Overview', link: '' },
        { text: 'Quick Start', link: 'quick-start' },
        { text: 'TLS Certificates & Auto-Renewal', link: 'certificates' },
        { text: 'Zone Domain Migration', link: 'zone-domain-migration' },
        { text: 'Create a Reverse Proxy Config', link: 'proxy-config' },
        { text: 'Pages Static Hosting Usage', link: 'pages-usage' },
        { text: 'Tunnel & Intranet Penetration', link: 'tunnel-usage' },
        { text: 'WAF Security Protection', link: 'waf-usage' },
        { text: 'WAF Auto IP Group Expressions', link: 'waf-ip-group-expr' },
        { text: 'Uptime Kuma Monitoring Sync', link: 'uptime-kuma' },
        { text: 'SSO Login Configuration', link: 'sso' },
        { text: 'Publish First Configuration', link: 'first-site' },
        { text: 'Troubleshooting', link: 'troubleshooting' },
        { text: 'Credits', link: 'credits' }
      ]
    }
  ]
}

function sidebarReference(): DefaultTheme.SidebarItem[] {
  return [
    {
      text: 'Reference',
      items: [
        { text: 'Overview', link: '' },
        { text: 'Configuration Options', link: 'configuration' },
        { text: 'CLI Commands', link: 'cli' }
      ]
    }
  ]
}

function sidebarDeployment(): DefaultTheme.SidebarItem[] {
  return [
    {
      text: 'Deployment',
      items: [
        { text: 'Overview', link: '' },
        { text: 'Deployment Guide', link: 'deployment' },
        { text: 'Start the Server', link: 'server' },
        { text: 'Access Agent', link: 'agent' },
        { text: 'Deploy Relay (Tunnel)', link: 'relay' },
        { text: 'Deploy OpenFlared', link: 'openflared' },
        { text: 'Upgrade & Maintenance', link: 'upgrade' }
      ]
    }
  ]
}

function sidebarDesign(): DefaultTheme.SidebarItem[] {
  return [
    {
      text: 'Design',
      items: [
        { text: 'Product Boundaries', link: '' },
        { text: 'System Architecture', link: 'architecture' },
        { text: 'Zone & Domain Resource Design', link: 'zone-design' },
        { text: 'Cloudflare DNS Pointing Design', link: 'cloudflare-pointing' },
        { text: 'Agent & Publish Model', link: 'agent-design' },
        { text: 'Tunnel & Intranet Penetration', link: 'tunnel-design' },
        { text: 'WAF Design', link: 'waf-design' },
        { text: 'WAF Orchestration Rule Design', link: 'waf-orchestration-design' },
        { text: 'Pages Static Hosting Design', link: 'pages-design' },
        { text: 'Edge Cache Strategy Design', link: 'edge-cache-design' },
        { text: 'Origin Error Page Design', link: 'origin-error-page' },
        { text: 'Edge Observability & Traffic Stats', link: 'observability-design' },
        { text: 'Observability Transport Model', link: 'observability-transport-model' },
        { text: 'Observability Protocol & Tables', link: 'observability-data-model' },
        { text: 'Log Store Decoupling', link: 'logstore' },
        { text: 'Uptime Kuma Sync Design', link: 'kuma-design' },
        { text: 'Login CAPTCHA Design', link: 'login-captcha' }
      ]
    }
  ]
}
