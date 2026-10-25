package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"alxtui/internal/ui"
)

func main() {
	var (
		cliPath     string
		configPath  string
		theme       string
		showVer     bool
		keepPlaying bool
	)
	flag.StringVar(&cliPath, "cli", "", "Lx-Cli 路径（默认使用程序同目录的 Lx-Cli）")
	flag.StringVar(&configPath, "config", "", "Lx-Cli 配置文件（默认程序旁 alx/go/config.toml）")
	flag.StringVar(&theme, "theme", "auto", "主题：auto / dark / light")
	flag.BoolVar(&showVer, "version", false, "显示版本")
	flag.BoolVar(&keepPlaying, "keep-playing", false, "退出 TUI 时保留 Lx-Cli/mpv 后台播放")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "ALX TUI — Lx-Cli 的终端音乐界面\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "用法: %s [选项]\n\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\n快捷键: 1-4 切换页面 · Tab 切页 · 鼠标左键播放 · 右键歌曲菜单 · q 退出\n")
		fmt.Fprintf(flag.CommandLine.Output(), "退出行为: 默认通过 Lx-Cli quit 关闭 mpv；使用 --keep-playing 保留后台播放。\n")
	}
	flag.Parse()

	if showVer {
		fmt.Printf("alx-tui %s\n", ui.Version)
		return
	}

	app, err := ui.New(ui.Options{CLIPath: cliPath, ConfigPath: configPath, Theme: theme})
	if err != nil {
		fmt.Fprintln(os.Stderr, "启动失败:", err)
		os.Exit(1)
	}

	p := tea.NewProgram(
		app,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithReportFocus(),
	)

	// Bubble Tea already turns SIGINT/SIGTERM into a normal QuitMsg. SIGHUP is
	// what many terminals send when the window is closed, and Bubble Tea does
	// not handle it, so route only SIGHUP through Program.Quit. Adding a second
	// handler for SIGTERM can race with Bubble Tea's own quit path.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP)
	done := make(chan struct{})
	go func() {
		select {
		case <-sigCh:
			p.Quit()
		case <-done:
		}
	}()

	_, runErr := p.Run()
	close(done)
	signal.Stop(sigCh)

	if !keepPlaying {
		if err := app.Shutdown(); err != nil {
			fmt.Fprintln(os.Stderr, "关闭播放器失败:", err)
		}
	}

	if runErr != nil {
		fmt.Fprintln(os.Stderr, "运行失败:", runErr)
		os.Exit(1)
	}
}
