package ui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"alxtui/internal/alx"
)

type settingsHit struct {
	y, x, w, h int
	kind       HitKind
	value      string
	index      int
}

type chipOption struct {
	value string
	label string
}

func (a *App) renderSettingsPage(r Rect) []string {
	t := a.th
	if r.H <= 0 || r.W <= 0 {
		return nil
	}
	const pad = 2
	innerX := r.X + pad
	innerW := r.W - pad*2
	if innerW < 24 {
		return []string{padVisible(t.Dim("窗口太窄"), r.W)}
	}
	if a.sourcesSelected < -1 {
		a.sourcesSelected = -1
	}
	if a.sourcesSelected == -1 && len(a.sources) > 0 && a.sourcesLoaded {
		a.sourcesSelected = 0
	}

	contentH := r.H - 2
	if contentH <= 0 {
		return []string{padVisible(t.Bold(t.Accent, "设置"), r.W)}
	}

	var lines []string
	var hits []settingsHit
	add := func(line string) int {
		lines = append(lines, line)
		return len(lines) - 1
	}
	addHit := func(y, x, w, h int, kind HitKind, value string, index int) {
		if w > 0 && h > 0 {
			hits = append(hits, settingsHit{y: y, x: x, w: w, h: h, kind: kind, value: value, index: index})
		}
	}
	section := func(title string) {
		if len(lines) > 0 {
			add("")
		}
		add(t.Bold(t.Accent, "▎"+title))
		add(t.Sep(innerW))
	}
	// ---- 音源 ----
	section("音源")
	if a.cfgErr != nil {
		add("  " + t.BadText("✕ 读取配置失败: ") + t.FaintText(truncate(prettyText(a.cfgErr.Error()), max(10, innerW-20))))
	}
	add(t.FaintText("自定义 JS 音源会自动参与搜索与播放解析；默认音源用于搜索筛选。"))

	srcOptions := []chipOption{
		{"all", "全部"}, {"wy", "网易云"}, {"tx", "QQ音乐"},
		{"kg", "酷狗"}, {"kw", "酷我"}, {"mg", "咪咕"},
	}
	if innerW < 88 {
		srcOptions = []chipOption{{"all", "全部"}, {"wy", "WY"}, {"tx", "TX"}, {"kg", "KG"}, {"kw", "KW"}, {"mg", "MG"}}
	}
	line, rowHits := a.settingsChips(len(lines), innerW, "默认音源", srcOptions, a.searchSource, HitSettingsSourceChip)
	add(line)
	hits = append(hits, rowHits...)

	qualityOptions := []chipOption{{"128k", "128k"}, {"320k", "320k"}, {"flac", "FLAC"}}
	line, rowHits = a.settingsChips(len(lines), innerW, "默认音质", qualityOptions, a.quality, HitSettingsQualityChip)
	add(line)
	hits = append(hits, rowHits...)

	// Custom source heading with actions.
	sourceLeft := t.Bold(t.Text, "自定义音源") + " " + t.FaintText(fmt.Sprintf("%d 个", len(a.sources)))
	if a.sourcesLoading {
		sourceLeft += "  " + t.AccentText(a.th.Spinner(a.spinner))
	}
	actions := []actionButton{
		{label: "＋ 添加", kind: HitSettingsAddSource, primary: true},
		{label: "删除", kind: HitSettingsRemoveSource, danger: true, disabled: a.selectedSource() == nil},
		{label: "⟳", kind: HitSettingsRefresh},
	}
	line, rowHits = a.settingsActions(len(lines), innerW, sourceLeft, actions)
	add(line)
	hits = append(hits, rowHits...)

	switch {
	case a.sourcesLoading && !a.sourcesLoaded:
		add("  " + t.FaintText(a.th.Spinner(a.spinner)+" 正在读取已安装音源…"))
	case a.sourcesErr != nil:
		add("  " + t.BadText("✕ 音源列表加载失败"))
		for _, l := range wrapText(prettyText(a.sourcesErr.Error()), max(10, innerW-4)) {
			add("  " + t.FaintText(l))
		}
	case len(a.sources) == 0:
		add("  " + t.FaintText("尚未安装自定义音源"))
		add("  " + t.FaintText("点击「＋ 添加」输入本地 .js 文件路径或远程 URL"))
	default:
		for i := range a.sources {
			src := a.sources[i]
			selected := i == a.sourcesSelected
			start := len(lines)
			marker := "  "
			nameStyle := t.Dim
			if selected {
				marker = t.AccentText("▌ ")
				nameStyle = func(s string) string { return t.Bold(t.Accent, s) }
			}
			name := truncate(prettyText(src.Name), max(4, innerW-20))
			enabled := t.FaintText("停用")
			if src.Enabled {
				enabled = t.GoodText("启用")
			}
			line1 := marker + nameStyle(name) + "  " + t.FaintText(truncate(prettyText(src.Version), max(0, innerW-displayWidth(name)-16))) + "  " + enabled
			add(padVisible(line1, innerW))
			platforms := strings.Join(src.PlatformList(), " ")
			if platforms == "" {
				platforms = "—"
			}
			line2 := "   " + t.FaintText("ID "+truncate(prettyText(src.ID), max(6, innerW-24))) + t.FaintText("  ·  平台 "+truncate(prettyText(platforms), max(0, innerW-30)))
			add(padVisible(line2, innerW))
			addHit(start, 0, innerW, 2, HitSettingsSourceRow, "", i)
		}
		add("  " + t.FaintText("点击一行选中，可删除；支持添加本地脚本或 URL。"))
	}

	// ---- 外观 ----
	section("外观")
	accLines, accHits := a.settingsColorControl(len(lines), innerW, "强调色", accentColorPresets, a.themePrefs.Accent, string(a.th.Accent), HitSettingsAccentColor, HitSettingsAccentCustom)
	lines = append(lines, accLines...)
	hits = append(hits, accHits...)
	textLines, textHits := a.settingsColorControl(len(lines), innerW, "一般色", textColorPresets, a.themePrefs.Text, string(a.th.Text), HitSettingsTextColor, HitSettingsTextCustom)
	lines = append(lines, textLines...)
	hits = append(hits, textHits...)

	preview := t.FaintText(padVisible("预览", 10)) +
		t.Bold(t.Accent, "强调 Aa 中文") + "  " +
		t.TextPlain("一般 Aa 中文") + "  " +
		t.Dim("次要文字")
	add(padVisible(preview, innerW))
	add("  " + t.FaintText("选择色块立即生效；「跟随」表示使用当前主题的默认文字色。"))
	themeName := "深色"
	if !a.baseDark {
		themeName = "浅色"
	}
	themeLine := "  " + t.FaintText(padVisible("基础主题", 10)) + t.Dim(themeName) +
		t.FaintText("   外观配置  ") + t.FaintText(truncate(prettyText(a.themePath), max(8, innerW-30)))
	add(padVisible(themeLine, innerW))
	if a.themeConfigErr != nil {
		add("  " + t.BadText("✕ 外观配置：") + t.FaintText(truncate(prettyText(a.themeConfigErr.Error()), max(8, innerW-16))))
	}
	hasCustom := a.themePrefs.Accent != "" || a.themePrefs.Text != ""
	line, rowHits = a.settingsActions(len(lines), innerW, "颜色设置",
		[]actionButton{{label: "恢复默认", kind: HitSettingsResetColors, danger: true, disabled: !hasCustom}})
	add(line)
	hits = append(hits, rowHits...)

	// ---- 下载 ----
	section("下载")
	dirLabelW := 10
	modifyLabel := "修改"
	modifyW := displayWidth(modifyLabel) + 2
	pathW := max(8, innerW-dirLabelW-modifyW-3)
	dirLine := t.FaintText(padVisible("下载目录", dirLabelW))
	dirX := dirLabelW
	path := truncate(prettyText(a.downloadDir), pathW)
	dirLine += t.AccentText(path)
	dirLine = padVisible(dirLine, innerW-modifyW-1)
	modifyX := innerW - modifyW
	dirLine += " " + lipgloss.NewStyle().Foreground(t.Muted).Padding(0, 1).Render(modifyLabel)
	add(padVisible(dirLine, innerW))
	addHit(len(lines)-1, dirX, max(1, pathW), 1, HitSettingsDownloadDir, "", 0)
	addHit(len(lines)-1, modifyX, modifyW, 1, HitSettingsDownloadDir, "", 0)

	cwd, _ := os.Getwd()
	if cwd == "" {
		cwd = "."
	}
	dirOptions := []chipOption{
		{"~/Music", "~/Music"},
		{"~/Downloads", "~/Downloads"},
		{cwd, "当前目录"},
	}
	line, rowHits = a.settingsChips(len(lines), innerW, "常用位置", dirOptions, a.downloadDir, HitSettingsDirPreset)
	add(line)
	hits = append(hits, rowHits...)

	line, rowHits = a.settingsChips(len(lines), innerW, "下载音质", qualityOptions, a.quality, HitSettingsQualityChip)
	add(line)
	hits = append(hits, rowHits...)

	// ---- 播放器 ----
	section("播放器")
	vol := a.state.Volume
	if vol <= 0 {
		vol = a.cfg.Player.DefaultVolume
	}
	labelW := 10
	var vb strings.Builder
	vb.WriteString(t.FaintText(padVisible("音量", labelW)))
	cursor := labelW
	minus := lipgloss.NewStyle().Foreground(t.Muted).Background(t.PanelBg).Padding(0, 1).Render("−")
	plus := lipgloss.NewStyle().Foreground(t.Muted).Background(t.PanelBg).Padding(0, 1).Render("+")
	vb.WriteString(minus)
	addHit(len(lines), cursor, displayWidth(minus), 1, HitSettingsVolumeMinus, "", 0)
	cursor += displayWidth(minus)
	value := fmt.Sprintf("  %d%%  ", vol)
	vb.WriteString(t.Bold(t.Text, value))
	cursor += displayWidth(value)
	vb.WriteString(plus)
	addHit(len(lines), cursor, displayWidth(plus), 1, HitSettingsVolumePlus, "", 0)
	cursor += displayWidth(plus)
	vb.WriteString(t.FaintText("   （播放器实时音量）"))
	add(padVisible(vb.String(), innerW))

	repeatOptions := []chipOption{{"off", "关"}, {"one", "单曲"}, {"all", "全部"}}
	repeat := a.state.Repeat
	if repeat == "" {
		repeat = "off"
	}
	line, rowHits = a.settingsChips(len(lines), innerW, "循环模式", repeatOptions, repeat, HitSettingsRepeat)
	add(line)
	hits = append(hits, rowHits...)

	shuffleOptions := []chipOption{{"off", "关"}, {"on", "开"}}
	shuffle := "off"
	if a.state.Shuffle {
		shuffle = "on"
	}
	line, rowHits = a.settingsChips(len(lines), innerW, "随机播放", shuffleOptions, shuffle, HitSettingsShuffle)
	add(line)
	hits = append(hits, rowHits...)

	// ---- 关于 ----
	section("关于")
	add("  " + t.FaintText("ALX TUI") + t.Dim(fmt.Sprintf("  v%s", Version)))
	backend := a.client.Bin
	if a.alxVersion != "" {
		backend = "Lx-Cli " + a.alxVersion + "  ·  " + backend
	}
	add("  " + t.FaintText("后端") + "  " + t.Dim(truncate(prettyText(backend), max(10, innerW-12))))
	if a.configFile != "" {
		add("  " + t.FaintText("配置") + "  " + t.Dim(truncate(prettyText(a.configFile), max(10, innerW-12))))
	}
	if a.profile != nil {
		add("  " + t.FaintText("配置根") + "  " + t.Dim(truncate(prettyText(a.profile.Root), max(10, innerW-12))))
	}
	if a.backendErr != nil {
		add("  " + t.BadText("后端信息读取失败: ") + t.FaintText(truncate(prettyText(a.backendErr.Error()), max(10, innerW-22))))
	}
	add("  " + t.FaintText("快捷键") + "  " + t.Dim("1-4 切页 · Tab 下一屏 · Space 播放/暂停 · 右键歌曲菜单"))

	// Scroll window.
	maxScroll := max(0, len(lines)-contentH)
	a.settingsScroll = clamp(a.settingsScroll, 0, maxScroll)
	start := a.settingsScroll
	end := min(len(lines), start+contentH)
	visible := make([]string, 0, contentH)
	for i := start; i < end; i++ {
		visible = append(visible, spaces(innerX)+padVisible(lines[i], innerW))
	}
	for len(visible) < contentH {
		visible = append(visible, spaces(innerX)+spaces(innerW))
	}
	visible = visible[:contentH]

	// Register only the visible slices of each hit.
	for _, h := range hits {
		top := max(h.y, start)
		bottom := min(h.y+h.h, start+contentH)
		if bottom <= top {
			continue
		}
		x := h.x
		if x >= innerW {
			continue
		}
		w := min(h.w, innerW-x)
		if w <= 0 {
			continue
		}
		a.addHit(h.kind, Rect{X: innerX + x, Y: r.Y + 2 + (top - start), W: w, H: bottom - top}, h.value, h.index, alx.Song{})
	}
	a.settingsArea = Rect{X: innerX, Y: r.Y + 2, W: innerW, H: contentH}

	scrollHint := "鼠标点击设置项"
	if maxScroll > 0 {
		scrollHint = fmt.Sprintf("↑↓ 滚动 · %d%%", int(float64(start)/float64(maxScroll)*100+0.5))
	}
	header := composeLR(t.Bold(t.Accent, "设置"), t.FaintText(scrollHint), innerW)
	out := []string{spaces(innerX) + padVisible(header, innerW), spaces(innerX) + t.Sep(innerW)}
	out = append(out, visible...)
	if len(out) > r.H {
		out = out[:r.H]
	}
	return out
}

func (a *App) settingsChips(y, width int, label string, options []chipOption, selected string, kind HitKind) (string, []settingsHit) {
	t := a.th
	labelW := 10
	var b strings.Builder
	b.WriteString(t.FaintText(padVisible(label, labelW)))
	x := labelW
	var hits []settingsHit
	for i, opt := range options {
		chip := t.ActiveChip(opt.label, opt.value == selected)
		chipW := displayWidth(chip)
		if x+chipW > width {
			break
		}
		b.WriteString(chip)
		hits = append(hits, settingsHit{y: y, x: x, w: chipW, h: 1, kind: kind, value: opt.value, index: i})
		x += chipW
		if i < len(options)-1 {
			b.WriteString(" ")
			x++
		}
	}
	return padVisible(b.String(), width), hits
}

// settingsActions renders a left label and right-aligned clickable actions.
func (a *App) settingsActions(y, width int, left string, specs []actionButton) (string, []settingsHit) {
	t := a.th
	total := buttonWidths(specs)
	x := width - total
	if x < 0 {
		x = 0
	}
	var b strings.Builder
	b.WriteString(padVisible(left, max(0, x-1)))
	if x > 0 {
		b.WriteString(" ")
	}
	var hits []settingsHit
	cursor := x
	for i, s := range specs {
		style := lipgloss.NewStyle().Foreground(t.Muted).Padding(0, 1)
		switch {
		case s.disabled:
			style = lipgloss.NewStyle().Foreground(t.Faint).Padding(0, 1)
		case s.primary:
			style = lipgloss.NewStyle().Background(t.AccentStrong).Foreground(t.OnAccent).Bold(true).Padding(0, 1)
		case s.danger:
			style = lipgloss.NewStyle().Foreground(t.Bad).Padding(0, 1)
		}
		rendered := style.Render(s.label)
		b.WriteString(rendered)
		if !s.disabled {
			hits = append(hits, settingsHit{y: y, x: cursor, w: displayWidth(rendered), h: 1, kind: s.kind, value: s.value, index: s.index})
		}
		cursor += displayWidth(rendered)
		if i < len(specs)-1 {
			b.WriteString(" ")
			cursor++
		}
	}
	return padVisible(b.String(), width), hits
}

func (a *App) saveDownloadDir(dir string) tea.Cmd {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		a.showToast("下载目录不能为空", toastError)
		return nil
	}
	a.downloadDir = dir
	return cmdSetConfig(a.client, "download.output_dir", dir, 0)
}

func (a *App) saveQuality(q string) tea.Cmd {
	if q == "" {
		return nil
	}
	a.quality = q
	return cmdSetConfig(a.client, "source.default_quality", q, 0)
}

func (a *App) saveDefaultSource(code string) tea.Cmd {
	if code == "" {
		return nil
	}
	a.searchSource = code
	return cmdSetConfig(a.client, "source.default_source", code, 0)
}

// colorPreset is one clickable color block in the appearance settings.
type colorPreset struct {
	value string
	label string
}

var accentColorPresets = []colorPreset{
	{"#A78BFA", "紫"},
	{"#60A5FA", "蓝"},
	{"#22D3EE", "青"},
	{"#34D399", "绿"},
	{"#FBBF24", "橙"},
	{"#F472B6", "粉"},
}

var textColorPresets = []colorPreset{
	{"", "跟随"},
	{"#E5E7EB", "浅"},
	{"#94A3B8", "灰"},
	{"#FFFFFF", "白"},
	{"#1F2937", "深"},
}

// settingsColorControl builds one color row (wrapping when necessary) plus a
// current-value line, and registers local hit boxes for every swatch.
func (a *App) settingsColorControl(y, width int, label string, presets []colorPreset, prefValue, effective string, kind, customKind HitKind) ([]string, []settingsHit) {
	t := a.th
	const labelW = 10
	var out []string
	var hits []settingsHit
	var b strings.Builder
	b.WriteString(t.FaintText(padVisible(label, labelW)))
	x := labelW
	lineIdx := 0
	flush := func() {
		out = append(out, padVisible(b.String(), width))
		b.Reset()
		lineIdx++
	}
	wrap := func(chipW int) {
		if x+chipW > width && x > labelW {
			flush()
			b.WriteString(spaces(labelW))
			x = labelW
		}
	}
	addGap := func() {
		if x < width {
			b.WriteString(" ")
			x++
		}
	}

	prefNorm := ""
	if prefValue != "" {
		prefNorm, _ = normalizeHex(prefValue)
	}
	isTextControl := kind == HitSettingsTextColor
	for _, preset := range presets {
		active := false
		switch {
		case preset.value == "":
			active = prefNorm == ""
		case prefNorm != "":
			active = strings.EqualFold(prefNorm, preset.value)
		case !isTextControl:
			// When no custom accent is stored, show which preset is the
			// current theme default. For "一般色", leaving the preference
			// empty means "follow theme", not "this particular preset".
			active = strings.EqualFold(effective, preset.value)
		}
		marker := " "
		if active {
			marker = t.AccentText("▸")
		}
		swatch := t.FaintText("◌")
		if preset.value != "" {
			swatch = lipgloss.NewStyle().Foreground(lipgloss.Color(preset.value)).Render("■")
		}
		name := ""
		if preset.label != "" {
			name = " " + t.MutedText(preset.label)
			if active {
				name = " " + t.Bold(t.Text, preset.label)
			}
		}
		chip := " " + marker + swatch + name + " "
		chipW := displayWidth(chip)
		wrap(chipW)
		b.WriteString(chip)
		hits = append(hits, settingsHit{y: y + lineIdx, x: x, w: chipW, h: 1, kind: kind, value: preset.value})
		x += chipW
		addGap()
	}

	customChip := " " + t.AccentText("自定义…") + " "
	customW := displayWidth(customChip)
	wrap(customW)
	b.WriteString(customChip)
	hits = append(hits, settingsHit{y: y + lineIdx, x: x, w: customW, h: 1, kind: customKind, value: "custom"})
	x += customW
	flush()

	// Current effective color (with a small swatch).
	curSwatch := lipgloss.NewStyle().Foreground(lipgloss.Color(effective)).Render("■")
	desc := effective
	if prefNorm == "" {
		if label == "一般色" {
			desc = "跟随主题 · " + effective
		} else {
			desc = "主题默认 · " + effective
		}
	}
	current := t.FaintText(padVisible("当前", labelW)) + curSwatch + " " + t.Dim(desc)
	out = append(out, padVisible(current, width))
	return out, hits
}
