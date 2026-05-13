import { defineAdditionalConfig, type DefaultTheme } from 'vitepress'

export default defineAdditionalConfig({
  description:
    'OpenFlare 是轻量、自托管的 OpenResty 控制面，用于管理反向代理、配置发布、节点同步、TLS 证书与基础观测。',

  themeConfig: {
    nav: nav(),

    sidebar: {
      '/guide/': { base: '/guide/', items: sidebarGuide() },
      '/reference/': { base: '/reference/', items: sidebarReference() },
      '/design/': { base: '/design/', items: sidebarDesign() }
    },

    editLink: {
      pattern: 'https://github.com/Rain-kl/OpenFlare/edit/main/docs/:path',
      text: '在 GitHub 上编辑此页面'
    },

    footer: {
      message: '基于 Apache License 2.0 发布',
      copyright: 'Copyright © OpenFlare contributors'
    },

    docFooter: {
      prev: '上一页',
      next: '下一页'
    },

    outline: {
      label: '页面导航'
    },

    lastUpdated: {
      text: '最后更新于'
    },

    notFound: {
      title: '页面未找到',
      quote: '这份文档还没有对应页面。',
      linkLabel: '前往首页',
      linkText: '回到 OpenFlare 文档'
    },

    langMenuLabel: '语言',
    returnToTopLabel: '回到顶部',
    sidebarMenuLabel: '菜单',
    darkModeSwitchLabel: '主题',
    lightModeSwitchTitle: '切换到浅色模式',
    darkModeSwitchTitle: '切换到深色模式',
    skipToContentLabel: '跳转到内容'
  }
})

function nav(): DefaultTheme.NavItem[] {
  return [
    { text: '指南', link: '/guide/', activeMatch: '/guide/' },
    { text: '参考', link: '/reference/', activeMatch: '/reference/' },
    { text: '设计', link: '/design/', activeMatch: '/design/' }
  ]
}

function sidebarGuide(): DefaultTheme.SidebarItem[] {
  return [
    {
      text: '指南',
      items: [
        { text: '概览', link: '' },
        { text: '快速开始', link: 'quick-start' },
        { text: '部署说明', link: 'deployment' },
        { text: 'SSO 登录配置', link: 'sso' },
        { text: '启动 Server', link: 'server' },
        { text: '接入 Agent', link: 'agent' },
        { text: '发布第一份配置', link: 'first-site' },
        { text: '升级与维护', link: 'upgrade' }
      ]
    }
  ]
}

function sidebarReference(): DefaultTheme.SidebarItem[] {
  return [
    {
      text: '参考',
      items: [
        { text: '概览', link: '' },
        { text: '配置项', link: 'configuration' },
        { text: '命令与脚本', link: 'cli' },
        { text: 'API 约定', link: 'api' },
        { text: '仓库结构', link: 'repository' }
      ]
    }
  ]
}

function sidebarDesign(): DefaultTheme.SidebarItem[] {
  return [
    {
      text: '设计',
      items: [
        { text: '产品边界', link: '' },
        { text: '系统架构', link: 'architecture' },
        { text: '发布模型', link: 'release-model' },
        { text: '开发约束', link: 'development' }
      ]
    }
  ]
}
