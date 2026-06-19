import * as React from "react";
import Link from "next/link";
import {motion} from "motion/react";
import {ArrowRight, CreditCard, Globe, Shield, Zap} from "lucide-react";
import {cn} from "@/lib/utils";
import {Button} from "@/components/ui/button";

export interface HeroSectionProps {
  className?: string;
}

/**
 * Hero Section - 首页 Hero 展示
 */
export const HeroSection = React.memo(function HeroSection({ className }: HeroSectionProps) {

  return (
    <section className={cn("w-full", className)}>
      <motion.div
        initial={{ opacity: 0, y: 40 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true }}
        transition={{
          delay: 0.3,
          duration: 0.8,
          ease: "easeInOut",
        }}
        className="relative z-10 w-full h-screen flex flex-col justify-center px-6"
      >
        <div className="container mx-auto max-w-7xl grid lg:grid-cols-2 gap-12 lg:gap-20 items-center">

          <div className="max-w-5xl pt-20 lg:pt-0">
            <motion.h1
              initial={{ opacity: 0, y: 20 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ duration: 0.6, delay: 0.1, ease: [0.16, 1, 0.3, 1] }}
              className="text-4xl md:text-5xl lg:text-6xl font-extrabold tracking-tight leading-[1.1] mb-6 text-foreground"
            >
              Modern Platform <br />
              <span className="bg-clip-text text-primary">
                为二次开发而生
              </span>
            </motion.h1>

            <motion.p
              initial={{ opacity: 0, y: 20 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ duration: 0.6, delay: 0.2, ease: [0.16, 1, 0.3, 1] }}
              className="text-sm md:text-base text-muted-foreground max-w-xl leading-relaxed mb-10"
            >
              通用的、现代化的后台管理系统脚手架
              <br className="hidden md:block" />
              开箱即用、完整基建、极易扩展，助您快速构建企业级应用。
            </motion.p>

            <motion.div
              initial={{ opacity: 0, y: 20 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ duration: 0.6, delay: 0.3, ease: [0.16, 1, 0.3, 1] }}
              className="flex flex-col sm:flex-row items-center gap-4"
            >
              <Link href="/" className="w-full sm:w-auto">
                <Button
                  size="lg"
                  className="w-full rounded-full bg-primary hover:bg-primary/90 font-medium transition-all active:scale-95"
                >
                  立即开始
                  <ArrowRight className="size-4" />
                </Button>
              </Link>

              <Link href="/docs/how-to-use" className="w-full sm:w-auto">
                <Button
                  variant="secondary"
                  size="lg"
                  className="w-full rounded-full font-medium active:scale-95"
                >
                  了解更多
                </Button>
              </Link>
            </motion.div>

            <motion.div
              initial={{ opacity: 0 }}
              whileInView={{ opacity: 1 }}
              viewport={{ once: true }}
              transition={{ duration: 0.8, delay: 0.5 }}
              className="mt-16 flex flex-wrap gap-8 text-sm font-medium text-muted-foreground border-t border-border/50 pt-8"
            >
              <div className="flex items-center gap-2">
                <Zap className="w-5 h-5 text-yellow-500" />
                <span>开箱即用</span>
              </div>
              <div className="flex items-center gap-2">
                <Globe className="w-5 h-5 text-blue-500" />
                <span>高度可扩展</span>
              </div>
              <div className="flex items-center gap-2">
                <Shield className="w-5 h-5 text-green-500" />
                <span>工业级基建</span>
              </div>
            </motion.div>
          </div>

          <div className="hidden lg:block relative h-full min-h-[500px] w-full">
            <motion.div
              initial={{ opacity: 0, x: 50, rotate: -5 }}
              whileInView={{ opacity: 1, x: 0, rotate: 0 }}
              viewport={{ once: true }}
              transition={{ duration: 1, delay: 0.2, ease: "easeOut" }}
              className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-full max-w-md aspect-square"
            >
              <div className="absolute top-0 right-0 w-64 h-64 bg-purple-500/30 rounded-full blur-3xl animate-pulse" />
              <div className="absolute bottom-0 left-0 w-64 h-64 bg-blue-500/30 rounded-full blur-3xl animate-pulse delay-75" />

              <div className="relative z-10 w-full h-64 bg-background/40 backdrop-blur-2xl border border-white/10 rounded-3xl shadow-2xl p-6 flex flex-col justify-between transform transition-transform hover:scale-[1.02] duration-500">
                <div className="flex gap-4 items-center">
                  <CreditCard className="size-6" />
                  <span>Modern Platform</span>
                </div>

                <div className="space-y-4">
                  <div className="space-y-2">
                    <div className="text-xs text-muted-foreground mb-2">Stack</div>
                    <div className="flex flex-wrap gap-2">
                      <span className="inline-block px-2 py-1 bg-blue-500/10 text-blue-500 rounded text-xs font-medium">Go • Gin</span>
                      <span className="inline-block px-2 py-1 bg-purple-500/10 text-purple-500 rounded text-xs font-medium">React • Next.js</span>
                    </div>
                  </div>
                  <div className="flex justify-between items-end">
                    <div>
                      <p className="text-sm text-muted-foreground mb-1">Ready for</p>
                      <div className="text-lg font-bold tracking-tight text-foreground flex items-center gap-1">
                        <span>Develop</span>
                      </div>
                    </div>
                    <div className="h-8 w-8 rounded-full bg-primary" />
                  </div>
                </div>
              </div>

              <motion.div
                animate={{ y: [0, -20, 0] }}
                transition={{ duration: 4, repeat: Infinity, ease: "easeInOut" }}
                className="absolute -top-6 -right-6 z-20 bg-background/60 backdrop-blur-xl border border-white/20 p-4 rounded-2xl shadow-xl"
              >
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded-full flex items-center justify-center bg-emerald-500/20 text-emerald-500">
                    <Zap className="size-5" />
                  </div>
                  <div>
                    <p className="text-xs text-muted-foreground">Features</p>
                    <div className="text-sm font-bold">
                      <span>Modern</span>
                    </div>
                  </div>
                </div>
              </motion.div>

            </motion.div>
          </div>
        </div>
      </motion.div>
    </section>
  );
});
