# Pages ZIP 解包库升级

Dependabot PR #71 将 Pingora 的 `zip` 从 2.6.1 升级至 9.0.5。本地修订锁定 9.2.3，其中包含 GHSA-fpwh-q2m5-vfm2 的修复。新版 `ZipFile::name()` 返回 `Result<Cow<str>>`，预扫描与实际提取均传播名称读取错误。预扫描保留名称供公共目录校验，实际提取直接借用名称。现有 ZIP 解包测试显式使用 Deflate，以覆盖 Pages 压缩包的读取路径。
