// Package profile prepares the Go Lx-Cli backend in a portable directory
// beside the TUI while preserving existing Go configuration and data.
package profile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"alxtui/internal/alx"
)

type Candidate struct{ Name, Path, Kind, Version string }
type Prepared struct {
	Client   *alx.Client
	Backend  Candidate
	Migrated bool
}
type Manager struct{ Dir, Root string }

func New(executablePath string) (*Manager, error) {
	if executablePath == "" {
		var err error
		executablePath, err = os.Executable()
		if err != nil {
			return nil, err
		}
	}
	executablePath, err := filepath.Abs(executablePath)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(executablePath)
	root := envValue(os.Environ(), "ALX_TUI_HOME")
	if root == "" {
		root = filepath.Join(dir, "alx")
		if runtime.GOOS != "windows" {
			if info, err := os.Stat(root); err == nil && !info.IsDir() {
				root = filepath.Join(dir, "alx-config")
			}
		}
	}
	root, err = absolutePath(root)
	if err != nil {
		return nil, fmt.Errorf("配置根目录无效: %w", err)
	}
	return &Manager{Dir: dir, Root: root}, nil
}

// Discover only inspects the executable directory; Prepare validates Lx-Cli
// before any configuration or data operation.
func (m *Manager) Discover() ([]Candidate, error) {
	if m == nil {
		return nil, errors.New("profile manager is nil")
	}
	names := []string{"Lx-Cli"}
	if runtime.GOOS == "windows" {
		names = []string{"Lx-Cli.exe"}
	}
	var result []Candidate
	for _, name := range names {
		path := filepath.Join(m.Dir, name)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0) {
			continue
		}
		result = append(result, Candidate{Name: name, Path: path})
	}
	return result, nil
}

// Selected ignores legacy backend preferences and only resolves Lx-Cli.
func (m *Manager) Selected(explicit string) (Candidate, error) {
	if m == nil {
		return Candidate{}, errors.New("profile manager is nil")
	}
	if explicit != "" {
		return resolveCandidate(explicit, "--cli")
	}
	if env := envValue(os.Environ(), "LX_CLI_BIN"); env != "" {
		return resolveCandidate(env, "LX_CLI_BIN")
	}
	list, err := m.Discover()
	if err != nil {
		return Candidate{}, err
	}
	if len(list) > 0 {
		return list[0], nil
	}
	return Candidate{}, errors.New("找不到同目录 Lx-Cli 可执行文件（或使用 --cli / LX_CLI_BIN 指定路径）")
}

func (m *Manager) ThemePath() string { return filepath.Join(m.Root, "theme.json") }

var goVersionPattern = regexp.MustCompile(`^(?:v)?[0-9]+\.[0-9]+\.[0-9]+(?:-go(?:[.+-][0-9A-Za-z.-]+)?|\+go(?:[.-][0-9A-Za-z.-]+)?)$`)

func (m *Manager) Prepare(ctx context.Context, b Candidate, configOverride string) (Prepared, error) {
	if m == nil {
		return Prepared{}, errors.New("profile manager is nil")
	}
	if err := ctx.Err(); err != nil {
		return Prepared{}, err
	}
	path := b.Path
	if path == "" {
		path = filepath.Join(m.Dir, b.Name)
	}
	resolved, err := resolveCandidate(path, "后端")
	if err != nil {
		return Prepared{}, err
	}
	// Capture inherited paths before replacing them on this client. Never mutate
	// the process environment: the current backend may still be running.
	inherited := os.Environ()
	probe := &alx.Client{Bin: resolved.Path, Env: portableEnv(inherited, "", "", "")}
	version, err := probe.Version(ctx)
	if err != nil {
		return Prepared{}, fmt.Errorf("后端版本验证失败: %w", err)
	}
	version = strings.TrimSpace(version)
	if !goVersionPattern.MatchString(version) {
		return Prepared{}, fmt.Errorf("仅支持 Go Lx-Cli 后端，无法接受版本 %q", version)
	}
	resolved.Kind, resolved.Version = "go", version
	target := filepath.Join(m.Root, "go")
	if configOverride != "" {
		configOverride, err = absolutePath(configOverride)
		if err != nil {
			return Prepared{}, err
		}
	}
	info, err := os.Stat(target)
	if err == nil {
		if !info.IsDir() {
			return Prepared{}, fmt.Errorf("后端配置路径不是目录: %s", target)
		}
		client := makeClient(resolved.Path, target, configOverride, inherited)
		if _, err := client.Config(ctx); err != nil {
			return Prepared{}, fmt.Errorf("后端配置验证失败: %w", err)
		}
		return Prepared{Client: client, Backend: resolved}, nil
	}
	if !os.IsNotExist(err) {
		return Prepared{}, err
	}
	if err := os.MkdirAll(m.Root, 0700); err != nil {
		return Prepared{}, err
	}
	stage, err := os.MkdirTemp(m.Root, ".go-stage-*")
	if err != nil {
		return Prepared{}, err
	}
	defer os.RemoveAll(stage)
	for _, p := range []string{filepath.Join(stage, "data", "sources"), filepath.Join(stage, "cache")} {
		if err := os.MkdirAll(p, 0700); err != nil {
			return Prepared{}, err
		}
	}
	migrated, err := m.migrate(ctx, stage, target, inherited)
	if err != nil {
		return Prepared{}, fmt.Errorf("旧配置迁移失败: %w", err)
	}
	client := makeClient(resolved.Path, stage, configOverride, inherited)
	if _, err := client.Config(ctx); err != nil {
		return Prepared{}, fmt.Errorf("后端配置验证失败: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return Prepared{}, err
	}
	// A concurrently created profile must never be replaced, even if empty.
	if _, err := os.Lstat(target); err == nil {
		return Prepared{}, fmt.Errorf("目标配置目录已存在: %s", target)
	} else if !os.IsNotExist(err) {
		return Prepared{}, err
	}
	if err := os.Rename(stage, target); err != nil {
		return Prepared{}, err
	}
	return Prepared{Client: makeClient(resolved.Path, target, configOverride, inherited), Backend: resolved, Migrated: migrated}, nil
}

func makeClient(bin, root, override string, inherited []string) *alx.Client {
	config := override
	if config == "" {
		config = filepath.Join(root, "config.toml")
	}
	return &alx.Client{Bin: bin, ConfigPath: config, Env: portableEnv(inherited, config, filepath.Join(root, "data"), filepath.Join(root, "cache"))}
}

func portableEnv(inherited []string, config, data, cache string) []string {
	result := make([]string, 0, len(inherited)+3)
	for _, entry := range inherited {
		key, _, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, "ALX_HOME") || strings.EqualFold(key, "ALX_CONFIG") || strings.EqualFold(key, "ALX_DATA") || strings.EqualFold(key, "ALX_CACHE") {
			continue
		}
		result = append(result, entry)
	}
	return append(result, "ALX_CONFIG="+config, "ALX_DATA="+data, "ALX_CACHE="+cache)
}

func resolveCandidate(path, label string) (Candidate, error) {
	resolved, err := exec.LookPath(path)
	if err != nil && !errors.Is(err, exec.ErrDot) {
		return Candidate{}, fmt.Errorf("%s 指定的后端不可用 %s: %w", label, path, err)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return Candidate{}, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return Candidate{}, err
	}
	if !info.Mode().IsRegular() {
		return Candidate{}, fmt.Errorf("后端不是常规文件: %s", resolved)
	}
	return Candidate{Name: filepath.Base(resolved), Path: resolved}, nil
}

func absolutePath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimLeft(strings.TrimPrefix(path, "~"), "/\\"))
	}
	return filepath.Abs(os.ExpandEnv(path))
}

func envValue(env []string, key string) string {
	for i := len(env) - 1; i >= 0; i-- {
		k, value, _ := strings.Cut(env[i], "=")
		if strings.EqualFold(k, key) {
			return value
		}
	}
	return ""
}
