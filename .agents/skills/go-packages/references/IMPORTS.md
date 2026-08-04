# Import 组织

Go import 组织的详细规则和示例。

## Import 分组

Import 按组组织，组之间用空行分隔。标准库包始终放在第一组。

**最小分组（Uber）：** 标准库，然后其他所有。

**扩展分组（Google）：** 标准库 → 其他 → protocol buffers → 副作用。

```go
// 好：标准库与外部包分开
import (
    "fmt"
    "os"

    "go.uber.org/atomic"
    "golang.org/x/sync/errgroup"
)
```

```go
// 好：完整分组，包含 proto 和副作用
import (
    "fmt"
    "os"

    "github.com/dsnet/compress/flate"
    "golang.org/x/text/encoding"

    foopb "myproj/foo/proto/proto"

    _ "myproj/rpc/protocols/dial"
)
```

## Import 重命名

避免重命名 import，除非为了避免名称冲突；好的包名不需要重命名。
在发生冲突时，**优先重命名最本地的或项目特定的 import**。

**必须重命名：** 与其他 import 冲突、生成的 protocol buffer 包
（删除下划线，添加 `pb` 后缀）。

**可以重命名：** 无意义的名称（例如 `v1`）、与本地变量冲突。

```go
// 好：Proto 包用 pb 后缀重命名
import (
    foosvcpb "path/to/package/foo_service_go_proto"
)

// 好：当需要 url 变量时使用 urlpkg
import (
    urlpkg "net/url"
)

func parseEndpoint(url string) (*urlpkg.URL, error) {
    return urlpkg.Parse(url)
}
```

## 空白 Import（`import _`）

仅为副作用而导入的包（使用 `import _ "pkg"`）
应仅在程序的主包（main）或需要它们的测试中导入。

```go
// 好：在主包中使用空白 import
package main

import (
    _ "time/tzdata"
    _ "image/jpeg"
)
```

## 点 Import（`import .`）

**不要**使用点 import。它们使程序难以阅读，因为不清楚
`Quux` 这样的名称是当前包中的顶层标识符还是导入包中的。

**例外：** `import .` 形式在由于循环依赖而无法成为被测试包的一部分的测试文件中可能有用：

```go
package foo_test

import (
    "bar/testutil" // 也导入了 "foo"
    . "foo"
)
```

在这种情况下，测试文件不能是 `foo` 包，因为它使用了
`bar/testutil`，而后者导入了 `foo`。因此 `import .` 形式让文件
假装是 `foo` 包的一部分，即使实际上不是。

**除了这一种情况外，不要在程序中使用 `import .`。**

```go
// 不好：点 import 隐藏了来源
import . "foo"
var myThing = Bar() // Bar 来自哪里？

// 好：显式限定
import "foo"
var myThing = foo.Bar()
```
