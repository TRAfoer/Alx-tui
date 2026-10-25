package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"alxtui/internal/profile"
)

// ThemePrefs are the user-configurable colors. Empty strings mean "follow the
// built-in theme".
type ThemePrefs struct {
	Accent string `json:"accent,omitempty"`
	Text   string `json:"text,omitempty"`
}

// themeSavedMsg is emitted after writing the local TUI theme file.
type themeSavedMsg struct {
	path string
	err  error
}

// themeConfigPath keeps appearance beside the portable backend profiles and
// copies the legacy theme once. The explicit override continues to win.
func themeConfigPath() (string, error) {
	if p := strings.TrimSpace(os.Getenv("ALX_TUI_CONFIG")); p != "" {
		return p, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	manager, err := profile.New(executable)
	if err != nil {
		return "", err
	}
	path := manager.ThemePath()
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !os.IsNotExist(err) {
		return path, err
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return path, nil
	}
	data, err := os.ReadFile(filepath.Join(dir, "alx-tui", "theme.json"))
	if os.IsNotExist(err) {
		return path, nil
	}
	if err != nil {
		return path, fmt.Errorf("读取旧主题失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if os.IsExist(err) {
		return path, nil
	}
	if err != nil {
		return path, err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		return path, errors.Join(writeErr, closeErr)
	}
	return path, nil
}

func loadThemePrefs(path string) (ThemePrefs, error) {
	var prefs ThemePrefs
	if path == "" {
		return prefs, errors.New("主题配置路径为空")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return prefs, nil
		}
		return prefs, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return prefs, nil
	}
	if err := json.Unmarshal(data, &prefs); err != nil {
		return ThemePrefs{}, fmt.Errorf("解析主题配置失败: %w", err)
	}
	var problems []error
	if prefs.Accent != "" {
		if normalized, ok := normalizeHex(prefs.Accent); ok {
			prefs.Accent = normalized
		} else {
			problems = append(problems, fmt.Errorf("强调色格式无效: %s", prefs.Accent))
			prefs.Accent = ""
		}
	}
	if prefs.Text != "" {
		if normalized, ok := normalizeHex(prefs.Text); ok {
			prefs.Text = normalized
		} else {
			problems = append(problems, fmt.Errorf("一般色格式无效: %s", prefs.Text))
			prefs.Text = ""
		}
	}
	return prefs, errors.Join(problems...)
}

func saveThemePrefs(path string, prefs ThemePrefs) error {
	if path == "" {
		return errors.New("主题配置路径为空，无法保存颜色")
	}
	if prefs.Accent != "" {
		normalized, ok := normalizeHex(prefs.Accent)
		if !ok {
			return fmt.Errorf("强调色格式无效: %s", prefs.Accent)
		}
		prefs.Accent = normalized
	}
	if prefs.Text != "" {
		normalized, ok := normalizeHex(prefs.Text)
		if !ok {
			return fmt.Errorf("一般色格式无效: %s", prefs.Text)
		}
		prefs.Text = normalized
	}
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".theme-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func cmdSaveThemePrefs(path string, prefs ThemePrefs) tea.Cmd {
	return func() tea.Msg {
		return themeSavedMsg{path: path, err: saveThemePrefs(path, prefs)}
	}
}

// resolveDark resolves the base theme mode. Precedence: explicit non-auto
// --theme value, ALX_TUI_THEME, COLORFGBG auto-detection, then dark.
func resolveDark(mode string) bool {
	envMode := strings.ToLower(strings.TrimSpace(os.Getenv("ALX_TUI_THEME")))
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "" && mode != "auto" {
		return mode != "light"
	}
	if envMode == "dark" {
		return true
	}
	if envMode == "light" {
		return false
	}
	if fg := os.Getenv("COLORFGBG"); fg != "" {
		parts := strings.Split(fg, ";")
		if len(parts) >= 2 {
			bg := parts[len(parts)-1]
			if bg == "7" || bg == "15" || bg == "default" {
				return false
			}
		}
	}
	return true
}

// buildTheme applies optional user colors on top of the selected base theme.
func buildTheme(dark bool, prefs ThemePrefs) Theme {
	th := NewTheme(dark)
	nominalBG := "#0B0F17"
	if !dark {
		nominalBG = "#FFFFFF"
	}
	if prefs.Text != "" {
		if txt, ok := normalizeHex(prefs.Text); ok {
			th.Text = lipgloss.Color(txt)
			th.PanelFg = lipgloss.Color(txt)
			th.Muted = lipgloss.Color(mixHex(nominalBG, txt, 0.65))
			th.Faint = lipgloss.Color(mixHex(nominalBG, txt, 0.45))
		}
	}
	if prefs.Accent != "" {
		if accent, ok := normalizeHex(prefs.Accent); ok {
			strong := mixHex("#000000", accent, 0.80)
			if !dark {
				strong = mixHex("#000000", accent, 0.82)
			}
			th.Accent = lipgloss.Color(accent)
			th.AccentStrong = lipgloss.Color(strong)
			th.OnAccent = lipgloss.Color(contrastFG(strong))
			if dark {
				th.Accent2 = lipgloss.Color(mixHex("#FFFFFF", accent, 0.70))
				sel := mixHex(string(th.PanelBg), accent, 0.30)
				th.SelBg = lipgloss.Color(sel)
				th.SelFg = lipgloss.Color(contrastFG(sel))
			} else {
				th.Accent2 = lipgloss.Color(mixHex("#000000", accent, 0.80))
				sel := mixHex("#FFFFFF", accent, 0.16)
				th.SelBg = lipgloss.Color(sel)
				th.SelFg = lipgloss.Color(contrastFG(sel))
			}
		}
	}
	return th
}

// normalizeHex accepts #RGB, #RRGGBB, RGB and RRGGBB and returns #RRGGBB.
func normalizeHex(s string) (string, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	if len(s) == 3 {
		var b strings.Builder
		for _, r := range s {
			b.WriteRune(r)
			b.WriteRune(r)
		}
		s = b.String()
	}
	if len(s) != 6 {
		return "", false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return "", false
		}
	}
	return "#" + strings.ToUpper(s), true
}

func parseRGB(hex string) (int, int, int, bool) {
	normalized, ok := normalizeHex(hex)
	if !ok {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(normalized[1:], 16, 24)
	if err != nil {
		return 0, 0, 0, false
	}
	return int(v>>16) & 0xff, int(v>>8) & 0xff, int(v) & 0xff, true
}

func rgbHex(r, g, b int) string {
	return fmt.Sprintf("#%02X%02X%02X", clamp(r, 0, 255), clamp(g, 0, 255), clamp(b, 0, 255))
}

// mixHex blends two colors. weight is the contribution of dst (0..1).
func mixHex(src, dst string, weight float64) string {
	sr, sg, sb, ok1 := parseRGB(src)
	dr, dg, db, ok2 := parseRGB(dst)
	if !ok1 || !ok2 {
		return src
	}
	weight = clampFloat(weight, 0, 1)
	return rgbHex(
		int(float64(sr)*(1-weight)+float64(dr)*weight+0.5),
		int(float64(sg)*(1-weight)+float64(dg)*weight+0.5),
		int(float64(sb)*(1-weight)+float64(db)*weight+0.5),
	)
}

func relativeLuminance(hex string) float64 {
	r, g, b, ok := parseRGB(hex)
	if !ok {
		return 0
	}
	channel := func(v int) float64 {
		c := float64(v) / 255.0
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(r) + 0.7152*channel(g) + 0.0722*channel(b)
}

// contrastFG returns a light or dark foreground with the best contrast.
func contrastFG(bg string) string {
	if relativeLuminance(bg) > 0.42 {
		return "#111827"
	}
	return "#FFFFFF"
}

// styleSearchInput applies the current theme to the search box.
func styleSearchInput(in *textinput.Model, th Theme) {
	in.PromptStyle = lipgloss.NewStyle().Foreground(th.Accent2)
	in.TextStyle = lipgloss.NewStyle().Foreground(th.Text)
	in.PlaceholderStyle = lipgloss.NewStyle().Foreground(th.Faint)
	in.Cursor.Style = lipgloss.NewStyle().Foreground(th.Accent).Bold(true)
}

// styleDialogInput keeps dialog input cells opaque on the panel background.
func styleDialogInput(in *textinput.Model, th Theme) {
	in.PromptStyle = lipgloss.NewStyle().Foreground(th.Accent2).Background(th.InputBg)
	in.TextStyle = lipgloss.NewStyle().Foreground(th.Text).Background(th.InputBg)
	in.PlaceholderStyle = lipgloss.NewStyle().Foreground(th.Faint).Background(th.InputBg)
	in.Cursor.Style = lipgloss.NewStyle().Foreground(th.Accent).Background(th.InputBg).Bold(true)
}

// applyTheme recomputes the palette after a color preference change.
func (a *App) applyTheme() {
	a.th = buildTheme(a.baseDark, a.themePrefs)
	styleSearchInput(&a.searchInput, a.th)
	if a.dlg != nil && a.dlg.hasInput {
		styleDialogInput(&a.dlg.input, a.th)
	}
}

func (a *App) saveThemeCmd() tea.Cmd {
	if a.themePath == "" {
		a.showToast("无法保存颜色：未找到主题配置路径", toastError)
		return nil
	}
	return cmdSaveThemePrefs(a.themePath, a.themePrefs)
}

func (a *App) setAccentColor(value string) tea.Cmd {
	normalized, ok := normalizeHex(value)
	if !ok {
		a.showToast("强调色格式无效，应为 #RRGGBB", toastError)
		return nil
	}
	a.themePrefs.Accent = normalized
	a.applyTheme()
	a.themeConfigErr = nil
	return a.saveThemeCmd()
}

func (a *App) setTextColor(value string) tea.Cmd {
	value = strings.TrimSpace(value)
	if value != "" {
		normalized, ok := normalizeHex(value)
		if !ok {
			a.showToast("一般色格式无效，应为 #RRGGBB", toastError)
			return nil
		}
		value = normalized
	}
	a.themePrefs.Text = value
	a.applyTheme()
	a.themeConfigErr = nil
	return a.saveThemeCmd()
}

func (a *App) resetThemeColors() tea.Cmd {
	if a.themePrefs.Accent == "" && a.themePrefs.Text == "" {
		return nil
	}
	a.themePrefs = ThemePrefs{}
	a.applyTheme()
	a.themeConfigErr = nil
	a.showToast("已恢复默认颜色", toastSuccess)
	return a.saveThemeCmd()
}
