# alx-tui

基于 Go / Bubble Tea 的音乐界面，固定调用 Lx-Cli。支持搜索、播放、音源管理、歌单链接导入、歌词、下载及外观设置。

## 构建与使用

需要 Go 1.23+，播放需要 mpv。先构建旁边的 Lx-Cli 项目，再执行：

```powershell
.\build.cmd
.\run.cmd
```

构建产物位于 `bin/`。脚本会复制相邻项目已编译的 `Lx-Cli.exe`；也可手动将它放到 `bin/alx-tui.exe` 旁。直接双击 `bin/alx-tui.exe` 即可启动。

跨平台构建：`go build -o bin/alx-tui .`，并放入同目录的 `Lx-Cli`。

按 `1`–`4` 切页，`q` 退出。默认配置位于程序旁 `alx/`；`--cli` 指定 Lx-Cli，`ALX_TUI_HOME` 覆盖配置根，`ALX_TUI_CONFIG` 覆盖主题文件。`bin/`、运行数据和编译产物不纳入 Git。

## 验证

```powershell
go test ./...
go vet ./...
```
