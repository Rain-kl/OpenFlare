# Frontend i18n Design

Date: 2026-07-24  
Status: Implemented in OpenFlare (ported from Wavelet 1625cfb, extended to product console)  
Scope: Frontend UI only

## 1. Goals

Add bilingual UI support for Wavelet frontend:

- Languages: `zh-CN` and `en`
- Default locale: `zh-CN`
- Locale resolution: explicit user choice → browser language → default
- Phase 1: infrastructure + core paths only (layout / auth / settings)
- Must remain compatible with `NEXT_STANDALONE_EXPORT` static export

### Non-goals (Phase 1)

- Backend API error / message localization
- Email / push notification localization
- URL locale prefixes (`/en/...`, `/zh-CN/...`) and SEO hreflang
- Full translation of all admin business pages

## 2. Context

Current state:

- Root layout hardcodes `lang='zh-CN'`
- UI copy is mostly Chinese string literals across many TSX files
- Date formatting often hardcodes `zh-CN` / `date-fns/locale` `zhCN`
- No i18n library is installed
- Frontend supports both normal Next rewrites mode and static export embed mode

## 3. Approach

Use **next-intl in non-routing / provider mode**.

Why this approach:

- Mature App Router integration and clear `useTranslations` API
- ICU message format ready when needed
- Avoids locale-prefixed routing, which conflicts with static-export simplicity and current route structure
- Cookie + browser detection matches product preference without SEO path requirements

Rejected alternatives:

- Fully custom Context + JSON: lower dependency cost, but reimplements interpolation/plurals/type safety poorly
- `i18next` + `react-i18next`: powerful, but heavier and less natural for this Next App Router setup

## 4. Architecture

```
RootLayout
  html lang={locale}
  ThemeProvider
    CustomThemeProvider
      AppQueryProvider
        NextIntlClientProvider(locale, messages)
          existing User / Notification / Bell providers
            pages + components
```

### Key files

| Path | Responsibility |
| --- | --- |
| `frontend/i18n/config.ts` | Supported locales, default locale, cookie name, normalize helpers |
| `frontend/i18n/request.ts` | `getRequestConfig` for server-side locale/messages resolution when not in export mode |
| `frontend/i18n/client.ts` | Client helpers to read/write locale preference |
| `frontend/messages/zh-CN.json` | Chinese messages |
| `frontend/messages/en.json` | English messages |
| `frontend/components/common/language-switcher.tsx` (or under `layout/`) | Language switch UI |
| `frontend/lib/i18n-format.ts` (optional location under `i18n/`) | Locale-aware date/number formatting helpers |

### Runtime flow

1. Resolve locale: cookie `NEXT_LOCALE` → browser languages → `zh-CN`
2. Load `messages/{locale}.json`
3. Provide locale + messages through `NextIntlClientProvider`
4. Components call `useTranslations('<namespace>')`
5. Language switcher writes cookie and refreshes locale/messages
6. Update `document.documentElement.lang`

## 5. Locale Resolution

Supported locales: `zh-CN`, `en`

Normalization:

- `zh`, `zh-CN`, `zh-Hans*` → `zh-CN`
- `en`, `en-US`, `en-GB`, other `en-*` → `en`
- anything else → `zh-CN`

Priority:

1. User explicit choice stored in cookie `NEXT_LOCALE`
2. Browser language (`Accept-Language` on server, `navigator.languages` on client)
3. Default `zh-CN`

Invalid cookie values are normalized to a supported locale and may be rewritten to a valid value.

## 6. Static Export Compatibility

Constraints:

- No locale-segment routes
- No middleware-based locale rewriting required for correctness
- `build:embed` (`NEXT_STANDALONE_EXPORT=true`) must continue to work

Behavior:

- **Normal SSR/dev**: resolve locale on server when possible to reduce first-paint language flash
- **Static export**: ship both message catalogs; resolve on client from cookie/browser; accept a brief default-language flash similar to theme hydration, using existing `suppressHydrationWarning` patterns where needed

## 7. Message Organization

Single catalog files with nested namespaces:

```json
{
  "common": {
    "save": "保存",
    "cancel": "取消",
    "loading": "加载中..."
  },
  "layout": {
    "nav": {
      "home": "首页",
      "myFiles": "我的文件"
    },
    "userMenu": {
      "settings": "设置",
      "logout": "退出登录"
    }
  },
  "auth": {
    "login": {
      "title": "登录",
      "submit": "登录"
    }
  },
  "settings": {
    "appearance": {
      "language": "语言",
      "languageDesc": "选择界面显示语言"
    }
  }
}
```

Conventions:

- Keys use camelCase and hierarchical grouping
- Prefer complete phrases as values; avoid assembling sentences in components
- Use ICU only when needed (`{name}`, plural forms)
- Backend `error_msg` values are shown as-is in Phase 1
- Frontend-owned toast / validation copy is translated

Both locale files must keep the same key tree. A key-alignment check script is recommended.

## 8. Language Switcher UX

Placement:

- Header toolbar near theme controls
- Appearance settings page as an explicit preference row

UI labels for language options use native names and do not themselves translate:

- `中文`
- `English`

On change:

1. Persist `NEXT_LOCALE`
2. Apply new locale/messages (via refresh or controlled provider update)
3. Sync `document.documentElement.lang`
4. Preserve unrelated UI state where practical (theme, auth session, sidebar collapse)

## 9. Phase 1 Migration Scope

### In scope

- Install and wire `next-intl`
- Message catalogs for core namespaces
- Locale resolution + persistence
- `LanguageSwitcher`
- Translate:
  - layout shell: sidebar nav/user menu, header accessible labels / titles
  - auth: login / register / OTP labels, buttons, validation messages
  - settings: appearance (including language preference), profile, security, notifications, access-token visible copy
- Replace date/number hardcoding only where touched by the above paths
- Ensure `html lang` reflects active locale

### Out of scope

- Remaining admin pages and deep business modules
- Backend localization
- Route prefixing / SEO alternate links

Unmigrated pages may remain Chinese hard-coded; mixed-language UI is acceptable during incremental rollout.

## 10. Formatting Helpers

Introduce locale-aware helpers for dates/numbers used by migrated surfaces, e.g.:

- `formatDateTime(value, locale)`
- `formatNumber(value, locale)`

`date-fns` locale objects should follow active locale (`zhCN` / `enUS`) when a migrated component uses them.

## 11. Error Handling & Fallbacks

| Case | Behavior |
| --- | --- |
| Missing message key | Dev warning; do not crash; show key or fallback language value |
| Unsupported cookie locale | Normalize to supported locale / default |
| Partial migration | Keep hard-coded Chinese on unmigrated screens |
| Backend error strings | Display raw `error_msg` |

## 12. Testing & Acceptance

Manual:

1. No cookie + browser Chinese → Chinese UI
2. No cookie + browser English → English UI
3. Manual switch to English survives refresh
4. Manual switch back to Chinese survives refresh
5. Core paths (layout/auth/settings) have no major residual hard-coded Chinese UI copy
6. `pnpm build` and `pnpm build:embed` both succeed
7. Language switch does not break theme, session, or sidebar state

Automated (recommended):

- Unit tests for `normalizeLocale` / resolution priority
- Script or test asserting `zh-CN.json` and `en.json` key parity

## 13. Rollout Plan (high level)

1. Add i18n infrastructure and empty/core message files
2. Mount provider and language switcher
3. Migrate layout shell copy
4. Migrate auth copy
5. Migrate settings copy + appearance language control
6. Verify SSR and static-export builds
7. Document how later pages should adopt `useTranslations`

## 14. Open Implementation Notes

- Prefer cookie name `NEXT_LOCALE` unless an existing project cookie convention conflicts during implementation
- Prefer minimal surface-area integration with next-intl; avoid introducing locale-based routing APIs that break static export
- Keep `internal/util` and backend packages untouched
- After implementation, follow repo frontend conventions and existing provider composition style
