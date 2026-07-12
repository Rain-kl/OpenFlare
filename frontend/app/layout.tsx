import type {Metadata} from "next";
import {Toaster} from "@/components/ui/sonner";
import {ThemeProvider} from "@/components/layout/theme-provider";
import {CustomThemeProvider} from "@/lib/theme";
import {BellRingProvider} from "@/contexts/bell-ring-context";
import {NotificationSettingsProvider} from "@/contexts/notification-settings-context";
import {UserProvider} from "@/contexts/user-context";
import {AppQueryProvider} from "@/components/providers/query-provider";
import {SiteTitleUpdater} from "@/components/providers/title-updater";
import {RobotsMeta} from "@/components/layout/robots-meta";
import "./globals.css";

export const metadata: Metadata = {
  title: "OpenFlare",
  description: "OpenFlare 边缘节点与反向代理管理平台",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="zh-CN"
      className="hide-scrollbar font-sans"
      suppressHydrationWarning
    >
      <body
        className="hide-scrollbar font-sans antialiased"
      >
        <ThemeProvider
          attribute="class"
          defaultTheme="system"
          enableSystem
          disableTransitionOnChange
        >
          <CustomThemeProvider>
            <AppQueryProvider>
              <SiteTitleUpdater />
              <RobotsMeta />
              <UserProvider>
                <NotificationSettingsProvider>
                  <BellRingProvider>
                    {children}
                    <Toaster position="top-center" />
                  </BellRingProvider>
                </NotificationSettingsProvider>
              </UserProvider>
            </AppQueryProvider>
          </CustomThemeProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
