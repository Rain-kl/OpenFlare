# 包大小、程序结构和 CLI

关于包拆分、避免 init()、run() 模式和 CLI 结构的详细指南。

## 何时拆分包

```
包是否变得太大？
├─ 你能用一句话描述它的用途吗？
│  ├─ 不能 → 按职责拆分
│  └─ 能 → 保留，但检查以下内容
├─ 包中的文件是否从未导入彼此的未导出符号？
│  └─ 是 → 这些文件可以是独立的包
├─ 包是否有不同的用户群体使用不同部分？
│  └─ 是 → 按用户边界拆分
└─ godoc 页面是否过于庞大？
   └─ 是 → 拆分以提高可发现性
```

### 何时不应拆分

- 不要仅因为文件很长就拆分——聚焦的包中的大文件是可以的
- 不要创建只包含一个类型或函数的包
- 如果会产生循环依赖则不要拆分
- 避免将内部辅助工具拆分到 `util` 或 `internal/helpers` 包中

### 何时合并包

- 如果客户端代码很可能需要两个类型交互，保持它们在一起
- 如果类型有紧密耦合的实现
- 如果用户需要同时导入两个包才能有意义地使用其中任何一个

### 文件组织

Go 中没有"一个类型一个文件"的惯例。文件应该足够聚焦以便知道哪个文件包含什么内容，且足够小以便轻松查找。

---

## 避免 init()

优先使用显式函数而非 `init()`：

```go
// 不好：init() 带有 I/O 和环境依赖
var _config Config

func init() {
    cwd, _ := os.Getwd()
    raw, _ := os.ReadFile(path.Join(cwd, "config.yaml"))
    yaml.Unmarshal(raw, &_config)
}
```

```go
// 好：用于加载配置的显式函数
func loadConfig() (Config, error) {
    cwd, err := os.Getwd()
    if err != nil {
        return Config{}, err
    }

    raw, err := os.ReadFile(path.Join(cwd, "config.yaml"))
    if err != nil {
        return Config{}, err
    }

    var config Config
    if err := yaml.Unmarshal(raw, &config); err != nil {
        return Config{}, err
    }
    return config, nil
}
```

**init() 的可接受使用场景：**
- 无法用单个赋值完成的复杂表达式
- 可插拔钩子（例如 `database/sql` 方言、编码注册表）
- 确定性预计算

---

## Main 中的退出

仅在 `main()` 中调用 `os.Exit` 或 `log.Fatal*`。所有其他函数应
返回 error 来表示失败。

**为什么这很重要：**
- 不明显的控制流：任何函数都可以退出程序
- 难以测试：退出程序的函数也会退出测试
- 跳过的清理：`defer` 语句会被跳过

```go
// 不好：在辅助函数中使用 log.Fatal
func readFile(path string) string {
    f, err := os.Open(path)
    if err != nil {
        log.Fatal(err)  // 退出程序，跳过 defer
    }
    b, err := io.ReadAll(f)
    if err != nil {
        log.Fatal(err)
    }
    return string(b)
}
```

```go
// 好：返回 error，让 main() 决定是否退出
func main() {
    body, err := readFile(path)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(body)
}

func readFile(path string) (string, error) {
    f, err := os.Open(path)
    if err != nil {
        return "", err
    }
    b, err := io.ReadAll(f)
    if err != nil {
        return "", err
    }
    return string(b), nil
}
```

### run() 模式

优先在 `main()` 中**最多调用一次** `os.Exit` 或 `log.Fatal`。将
业务逻辑提取到返回 error 的独立函数中。

```go
func main() {
    if err := run(); err != nil {
        log.Fatal(err)
    }
}

func run() error {
    args := os.Args[1:]
    if len(args) != 1 {
        return errors.New("missing file")
    }

    f, err := os.Open(args[0])
    if err != nil {
        return err
    }
    defer f.Close()  // 将始终执行

    b, err := io.ReadAll(f)
    if err != nil {
        return err
    }

    // 处理 b...
    return nil
}
```

**`run()` 模式的优势：**
- 简短的 `main()` 函数，单一退出点
- 所有业务逻辑都可测试
- `defer` 语句始终执行

---

## 命令行接口

### Flag 命名

使用小写、连字符分隔的 flag 名称：

```go
// 好
flag.String("output-dir", ".", "directory for output files")
flag.Bool("dry-run", false, "print actions without executing")

// 不好
flag.String("outputDir", ".", "")    // camelCase
flag.String("output_dir", ".", "")   // 下划线
```

### 子命令

对于带有子命令的复杂 CLI，为每个子命令使用 `flag.NewFlagSet`：

```go
func main() {
    serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
    port := serveCmd.Int("port", 8080, "listen port")

    migrateCmd := flag.NewFlagSet("migrate", flag.ExitOnError)
    dryRun := migrateCmd.Bool("dry-run", false, "preview changes")

    switch os.Args[1] {
    case "serve":
        serveCmd.Parse(os.Args[2:])
        runServe(*port)
    case "migrate":
        migrateCmd.Parse(os.Args[2:])
        runMigrate(*dryRun)
    default:
        fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
        os.Exit(1)
    }
}
```

对于更大的 CLI，考虑使用 `cobra` 或 `urfave/cli` 等库。仅从
`main()` 退出。
