# Edge 自更新后的探测进程

- Edge 建立控制 WebSocket 时创建探测监督器，收到探测任务后才启动子进程。自更新确认就绪时会把 `edge-update-trial` 改名为 `edge-update-active`，此前保存的试运行路径随即失效。
- Linux 上通过 `/proc/self/exe` 启动同一二进制的 `probe-worker` 子命令，内核在执行时定位当前运行的文件。子进程崩溃后重启也使用这个入口。
- 运行时回归测试先让试运行中的 Edge 建立控制 WebSocket，再放行心跳完成升级。试运行路径消失后，下发指向回环地址的 TCP 探测，以子进程返回的 SSRF 拒绝结果确认任务已送达，同时验证原有的地址限制仍生效。
