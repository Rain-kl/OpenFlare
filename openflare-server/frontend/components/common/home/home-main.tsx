"use client"

import * as React from "react"
import {AnimatePresence, motion} from "motion/react"
import {useUser} from "@/contexts/user-context"
import {Card, CardHeader, CardTitle} from "@/components/ui/card"
import {Button} from "@/components/ui/button"
import {ArrowRight, ExternalLink, FileText, HelpCircle, Layers, Shield, ShieldCheck, Terminal, User} from "lucide-react"
import Link from "next/link"

export function HomeMain() {
  const { user } = useUser()
  const [isBannerVisible, setIsBannerVisible] = React.useState(true)

  const quickLinks = [
    {
      title: "个人资料",
      description: "管理您的个人账户信息及个性化配置",
      icon: User,
      url: "/settings/profile",
      color: "text-blue-500",
      bgColor: "bg-blue-500/10",
      borderColor: "hover:border-blue-500/30",
    },
    {
      title: "开发接口文档",
      description: "查看开放平台的 RESTful 接口规格说明",
      icon: FileText,
      url: "/docs/api",
      color: "text-emerald-500",
      bgColor: "bg-emerald-500/10",
      borderColor: "hover:border-emerald-500/30",
      external: true,
    },
    {
      title: "使用文档",
      description: "学习如何集成 API 及日常操作帮助指南",
      icon: HelpCircle,
      url: "/docs/how-to-use",
      color: "text-purple-500",
      bgColor: "bg-purple-500/10",
      borderColor: "hover:border-purple-500/30",
      external: true,
    },
  ]

  const adminLinks = [
    {
      title: "系统设置",
      description: "管理登录注册开关与认证源安全策略",
      icon: Shield,
      url: "/admin/settings",
      color: "text-amber-500",
      bgColor: "bg-amber-500/10",
      borderColor: "hover:border-amber-500/30",
    },
    {
      title: "全局系统配置",
      description: "动态管理平台运行时核心配置参数",
      icon: ShieldCheck,
      url: "/admin/system",
      color: "text-indigo-500",
      bgColor: "bg-indigo-500/10",
      borderColor: "hover:border-indigo-500/30",
    },
    {
      title: "用户权限管理",
      description: "集中查询并管理系统注册用户的启用状态",
      icon: User,
      url: "/admin/users",
      color: "text-rose-500",
      bgColor: "bg-rose-500/10",
      borderColor: "hover:border-rose-500/30",
    },
    {
      title: "后台任务调度",
      description: "分发与观测系统异步定时任务的执行情况",
      icon: Layers,
      url: "/admin/tasks",
      color: "text-teal-500",
      bgColor: "bg-teal-500/10",
      borderColor: "hover:border-teal-500/30",
    },
    {
      title: "系统日志",
      description: "查看异步任务执行日志与系统运行状态详情",
      icon: Terminal,
      url: "/admin/logs",
      color: "text-cyan-500",
      bgColor: "bg-cyan-500/10",
      borderColor: "hover:border-cyan-500/30",
    },
  ]

  return (
    <div className="py-6 space-y-8 max-w-6xl mx-auto">
      {/* 积分收益提示 banner */}
      <AnimatePresence>
        {isBannerVisible && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: "auto" }}
            exit={{ opacity: 0, height: 0 }}
            transition={{ duration: 0.3 }}
            className="overflow-hidden"
          >
            <div className="bg-gradient-to-r from-indigo-500/10 via-purple-500/10 to-pink-500/10 border border-dashed border-indigo-500/20 rounded-xl p-6 relative">
              <div className="max-w-xl space-y-3">
                <h3 className="text-lg font-bold text-indigo-600 dark:text-indigo-400">Modern Platform</h3>
                <p className="text-xs text-muted-foreground leading-relaxed">
                  通用的、现代化的后台管理系统脚手架, 开箱即用、完整基建、极易扩展，助您快速构建企业级应用。
                </p>
                <div className="flex gap-2.5">
                  <Button
                    size="sm"
                    className="bg-indigo-600 hover:bg-indigo-700 text-xs font-semibold px-4 shadow-sm h-7"
                    asChild
                  >
                    <Link href="/">开始使用</Link>
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    className="text-xs font-medium hover:bg-black/5 dark:hover:bg-white/5 h-7"
                    onClick={() => setIsBannerVisible(false)}
                  >
                    隐藏提示
                  </Button>
                </div>
              </div>
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* 快捷导航 */}
      <div className="space-y-4">
        <h2 className="text-lg font-semibold tracking-tight">常用功能导航</h2>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {quickLinks.map((link, idx) => (
            <motion.div
              key={link.title}
              initial={{ opacity: 0, y: 15 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.4, delay: idx * 0.05 }}
              whileHover={{ y: -4 }}
              className="h-full"
            >
              <Card className={`h-full border border-dashed flex flex-col justify-between hover:bg-muted/40 transition-all ${link.borderColor} duration-300 shadow-none`}>
                <CardHeader className="p-5">
                  <div className={`size-10 rounded-lg flex items-center justify-center ${link.bgColor} ${link.color} mb-3`}>
                    <link.icon className="size-5" />
                  </div>
                  <CardTitle className="text-sm font-semibold mb-1 flex items-center gap-1">
                    {link.title}
                    {link.external && <ExternalLink className="size-3 text-muted-foreground" />}
                  </CardTitle>
                  <p className="text-xs text-muted-foreground leading-normal font-normal">
                    {link.description}
                  </p>
                </CardHeader>
                <div className="px-5 pb-5">
                  {link.external ? (
                    <Button variant="link" className="p-0 h-auto text-xs text-indigo-500 font-medium" asChild>
                      <Link href={link.url} target="_blank" rel="noopener noreferrer">
                        立即跳转 <ArrowRight className="size-3 ml-1" />
                      </Link>
                    </Button>
                  ) : (
                    <Button variant="link" className="p-0 h-auto text-xs text-indigo-500 font-medium" asChild>
                      <Link href={link.url}>
                        立即进入 <ArrowRight className="size-3 ml-1" />
                      </Link>
                    </Button>
                  )}
                </div>
              </Card>
            </motion.div>
          ))}
        </div>
      </div>

      {/* 管理面板 (仅管理员可见) */}
      {user?.is_admin && (
        <div className="space-y-4 pt-2">
          <h2 className="text-lg font-semibold tracking-tight text-rose-500 flex items-center gap-1.5">
            <Shield className="size-5" />
            后台管理控制台
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {adminLinks.map((link, idx) => (
              <motion.div
                key={link.title}
                initial={{ opacity: 0, y: 15 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.4, delay: (idx + 4) * 0.05 }}
                whileHover={{ y: -4 }}
                className="h-full"
              >
                <Card className={`h-full border border-dashed flex flex-col justify-between hover:bg-muted/40 transition-all ${link.borderColor} duration-300 shadow-none`}>
                  <CardHeader className="p-5">
                    <div className={`size-10 rounded-lg flex items-center justify-center ${link.bgColor} ${link.color} mb-3`}>
                      <link.icon className="size-5" />
                    </div>
                    <CardTitle className="text-sm font-semibold mb-1">
                      {link.title}
                    </CardTitle>
                    <p className="text-xs text-muted-foreground leading-normal font-normal">
                      {link.description}
                    </p>
                  </CardHeader>
                  <div className="px-5 pb-5">
                    <Button variant="link" className="p-0 h-auto text-xs text-indigo-500 font-medium" asChild>
                      <Link href={link.url}>
                        立即进入 <ArrowRight className="size-3 ml-1" />
                      </Link>
                    </Button>
                  </div>
                </Card>
              </motion.div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
