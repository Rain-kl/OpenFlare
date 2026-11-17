# PoW 防护

在「安全性 → PoW 防护」选择站点，即可独立启用工作量证明校验。设置计算模式、难度、通行有效期和挑战有效期后保存，再到「版本发布」发布配置，使策略在节点生效。

并行模式最多使用 8 个 Worker；单线程模式使用 1 个 Worker。难度每增加 1，预期计算量约增加 16 倍。通行凭证按访问域名保存为 HttpOnly Cookie，在受保护路径的有效访问会续期。

## 选择触发路径

| 模式 | 行为 |
| --- | --- |
| 全站校验，排除白名单 | 校验所有到达 PoW 的请求；匹配白名单的路径跳过校验 |
| 仅校验黑名单路径 | 只校验匹配黑名单的路径；至少配置一条黑名单规则 |

白名单始终优先。每个列表中的规则按「任意一项匹配」处理。跳过 PoW 后，其他防护规则继续执行。

| 匹配方式 | 示例 | 效果 |
| --- | --- | --- |
| 精确 | `/login` | 只匹配该路径 |
| 前缀 | `/admin/` | 匹配该目录及其子路径 |
| 通配符 | `/admin/*` | `*` 匹配任意数量字符，包括 `/` 和空字符串；匹配整个路径 |
| 正则 | `^/admin(?:/|$)` | 匹配 `/admin` 及其子路径，排除 `/administrator` |

匹配对象是 OpenResty 规范化后的 URI 路径，不含查询参数，区分大小写。普通路径须以 `/` 开头；正则保存时按 Go 正则语法验证，节点通过 PCRE 执行。每个列表最多 100 条规则，每条最多 1024 字节。

例如，仅保护后台时，选择「仅校验黑名单路径」，添加黑名单正则 `^/admin(?:/|$)`，再添加白名单精确路径 `/admin/health`。首页、静态资源和健康检查可直接访问，后台页面需要先通过验证。

挑战页用于浏览器访问。Webhook、健康检查及无法执行 JavaScript 的 API 客户端应通过路径规则跳过校验。

路径匹配和跳过动作的设计参考 [Cloudflare 规则运算符](https://developers.cloudflare.com/ruleset-engine/rules-language/operators/) 与 [规则动作](https://developers.cloudflare.com/ruleset-engine/rules-language/actions/)。

## 自定义挑战页

进入「响应页面 → PoW 挑战页」，分别选择页面风格与组件样式：

- 页面风格：现有风格、Cloudflare 五秒盾风格。
- 验证组件：进度条、Turnstile 状态转换。两种组件都自动执行本地 PoW 验证。

留空 HTML 会使用所选内置页面。「加载模板」会将模板放入编辑器，便于继续修改布局、文案与样式。「使用内置页面」会清空自定义 HTML，保存后恢复所选风格。预览在隔离的 iframe 中模拟计算与成功状态；真实访问会执行 PoW，服务端核验成功后刷新原始地址。

HTML 最大为 256 KiB，支持以下占位符。文字占位符会进行 HTML 转义：

| 占位符 | 内容 |
| --- | --- |
| `{{widget}}` | 所选验证组件 |
| `{{host}}` | 当前访问域名 |
| `{{lang}}` | 挑战页语言，`zh-CN` 或 `en` |
| `{{title}}`、`{{status}}` | 内置标题与说明 |
| `{{protected}}` | 防护说明 |
| `{{why}}`、`{{why_description}}` | 验证原因与解释 |
| `{{noscript}}` | JavaScript 不可用时的提示 |

验证运行时会自动注入 HTML。最小模板如下：

```html
<!DOCTYPE html>
<html lang="{{lang}}">
<head><meta charset="utf-8"><title>{{title}}</title></head>
<body>
  <h1>{{host}}</h1>
  {{widget}}
  <noscript>{{noscript}}</noscript>
</body>
</html>
```

也可以直接放置组件，指定不同样式：

```html
<openflare-pow appearance="progress"></openflare-pow>
<openflare-pow appearance="turnstile"></openflare-pow>
```

自定义界面可以监听 `openflare:pow` 事件。`detail.status` 依次为 `loading`、`working`、`verifying`、`success`，失败时为 `error`；`progress` 为 0～100 的估算进度，只有服务端核验成功后才到 100。

```html
<p id="verification-status" role="status"></p>
<script>
  window.addEventListener('openflare:pow', (event) => {
    document.getElementById('verification-status').textContent = event.detail.message;
  });
</script>
```

运行时还提供 `window.OpenFlarePoW.state`、`subscribe(callback)` 和 `retry()`；`subscribe` 会立即调用回调提供当前状态，并返回用于取消订阅的函数。组件样式封装在 Shadow DOM 中，可使用 `--accent`、`--success`、`--error`、`--surface`、`--line` CSS 变量调整颜色。

## 旧 WAF PoW 策略

未保存独立策略的站点继续使用已有 WAF PoW 节点。WAF 节点库不再提供新增 PoW 入口，已有节点仍可查看和编辑。

站点第一次保存独立策略后，旧 WAF PoW 节点自动沿 `next` 跳过，其他 WAF 规则保持原有执行顺序。独立策略即使关闭也会接管该站点的 PoW 开关。若原来通过 IP、地域或 UA 分支有条件地触发 PoW，迁移前应按实际需求设置新的路径规则。
