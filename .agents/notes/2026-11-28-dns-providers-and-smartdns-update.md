# DNS 提供商与 SmartDNS 远程更新

DNS 账号保存加密凭据、最近验证状态和可访问区域。API 探测失败时保留上次成功发现的区域，使该区域的操作明确报告凭据不可用。每日 Asynq 调度和手动刷新共用写入路径。探测结果按凭据和开始时间有条件写入，避免较早的探测覆盖新配置。

`internal/openflare/dnsprovider` 的注册中心为 Cloudflare、AliDNS 和 DNSPod 选择记录适配器、DNS-01 策略及能力。指向分组按记录域名匹配最具体的区域。同等范围优先使用记录原账号、其次使用迁移前绑定的 Cloudflare 账号，其余重叠需要清理账号范围。记录 ID 与账号、区域一起保存，防止跨提供商使用旧 ID。AliDNS 和 DNSPod 只同步 DNS，自动忽略代理设置。

原 Cloudflare 独立连接令牌迁入 DNS 账号，已绑定 DNS 账号的连接继续引用原账号。首次刷新前保留已有连接的同步能力。SmartDNS 一键委派仅在授权的父区运行，支持 Cloudflare 和 AliDNS。操作者确认已保存的 NS 名称后，协调器先创建缺失记录，再移除旧记录。

AliDNS 使用官方 Go SDK v5 和 API `2015-01-09`。查询记录用 `COMBINATION` 对主机记录和线路做精确匹配。DNSPod 使用官方 Go SDK 的 `v20210323` 包，记录操作使用默认线路。Cloudflare 区域发现使用 API v4，每页最多 50 条。

- [阿里云云解析 DNS API](https://www.alibabacloud.com/help/en/dns/api-alidns-2015-01-09-overview) · [官方 Go SDK](https://github.com/alibabacloud-go/alidns-20150109)
- [腾讯云 DNSPod API](https://cloud.tencent.com/document/api/1427/56172) · [官方 Go SDK](https://github.com/TencentCloud/tencentcloud-sdk-go)
- [Cloudflare 区域列表](https://developers.cloudflare.com/api/resources/zones/methods/list/) · [子域委派](https://developers.cloudflare.com/dns/manage-dns-records/how-to/subdomains-outside-cloudflare/)

SmartDNS 更新请求通过实例心跳传递，批量入队在数据库内原子执行。独立节点只接受更新 ID 与正式版或 Canary 渠道。低权限 DNS 服务写入状态目录，systemd 更新服务以主机配置的 GitHub 仓库和安装路径执行升级。更新失败时还原旧二进制，控制面只接受当前请求 ID 的进度回报。Docker 和内嵌实例沿用各自的部署渠道更新。
