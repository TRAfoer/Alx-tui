package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"alxtui/internal/alx"
)

type dialogKind int

const (
	dialogNewPlaylist dialogKind = iota
	dialogRenamePlaylist
	dialogAddSource
	dialogDownloadDir
	dialogDeletePlaylist
	dialogRemoveSource
	dialogAccentColor
	dialogTextColor
	dialogImportPlaylist
	dialogImportPlaylistName
)

type dialogHitAction int

const (
	dialogHitInput dialogHitAction = iota
	dialogHitCancel
	dialogHitConfirm
)

type dialogHit struct {
	rect   Rect
	action dialogHitAction
}

type dialogState struct {
	kind         dialogKind
	seq          int
	title        string
	label        string
	placeholder  string
	hint         string
	input        textinput.Model
	hasInput     bool
	busy         bool
	err          string
	target       string
	pending      *alx.Song
	confirmFocus int // 0 cancel, 1 confirm
	hits         []dialogHit
	x, y, w, h   int
}

func (a *App) closeDialog() {
	a.dlg = nil
}

func (a *App) newDialogInput(value string) textinput.Model {
	t := a.th
	in := textinput.New()
	in.Prompt = "› "
	in.Placeholder = ""
	in.CharLimit = 240
	in.SetValue(value)
	in.CursorEnd()
	styleDialogInput(&in, t)
	in.Focus()
	return in
}

func (a *App) openNewPlaylistDialog(pending *alx.Song) tea.Cmd {
	a.menu = nil
	a.blurSearch()
	a.dialogSeq++
	in := a.newDialogInput("")
	hint := "输入歌单名称，Enter 创建，Esc 取消。"
	if pending != nil {
		hint = "创建后会把《" + truncate(prettyText(pending.Name), 18) + "》加入新歌单。"
	}
	a.dlg = &dialogState{
		kind:        dialogNewPlaylist,
		seq:         a.dialogSeq,
		title:       "新建歌单",
		label:       "歌单名称",
		placeholder: "例如：通勤循环",
		hint:        hint,
		input:       in,
		hasInput:    true,
		pending:     pending,
	}
	return in.Focus()
}

func (a *App) openRenamePlaylistDialog() tea.Cmd {
	name := a.selectedPlaylistName()
	if name == "" {
		return nil
	}
	a.menu = nil
	a.blurSearch()
	a.dialogSeq++
	in := a.newDialogInput(name)
	a.dlg = &dialogState{
		kind:        dialogRenamePlaylist,
		seq:         a.dialogSeq,
		title:       "重命名歌单",
		label:       "新名称",
		placeholder: "新的歌单名称",
		hint:        "原有歌曲不会丢失。Enter 保存，Esc 取消。",
		input:       in,
		hasInput:    true,
		target:      name,
	}
	return in.Focus()
}

func (a *App) openImportPlaylistDialog() tea.Cmd {
	a.menu = nil
	a.blurSearch()
	a.dialogSeq++
	in := a.newDialogInput("")
	in.CharLimit = 4096
	a.dlg = &dialogState{
		kind:        dialogImportPlaylist,
		seq:         a.dialogSeq,
		title:       "导入歌单链接",
		label:       "网易云 / QQ 歌单链接或分享文本",
		placeholder: "https://music.163.com/playlist?id=…",
		hint:        "粘贴歌单链接或完整分享文本，Enter 下一步，Esc 取消。",
		input:       in,
		hasInput:    true,
	}
	return in.Focus()
}

func (a *App) openAddSourceDialog() tea.Cmd {
	a.menu = nil
	a.blurSearch()
	a.dialogSeq++
	in := a.newDialogInput("")
	a.dlg = &dialogState{
		kind:        dialogAddSource,
		seq:         a.dialogSeq,
		title:       "添加音源",
		label:       "脚本路径或 URL",
		placeholder: "/path/to/source.js 或 https://…",
		hint:        "支持本地 .js 音源脚本，或远程脚本地址。",
		input:       in,
		hasInput:    true,
	}
	return in.Focus()
}

func (a *App) openDownloadDirDialog() tea.Cmd {
	a.menu = nil
	a.blurSearch()
	a.dialogSeq++
	in := a.newDialogInput(a.downloadDir)
	a.dlg = &dialogState{
		kind:        dialogDownloadDir,
		seq:         a.dialogSeq,
		title:       "下载目录",
		label:       "保存歌曲的位置",
		placeholder: "~/Music",
		hint:        "支持绝对路径或以 ~/ 开头的路径。目录不存在时会由下载器创建。",
		input:       in,
		hasInput:    true,
	}
	return in.Focus()
}

func (a *App) openAccentColorDialog() tea.Cmd {
	a.menu = nil
	a.blurSearch()
	a.dialogSeq++
	value := a.themePrefs.Accent
	if value == "" {
		value = string(a.th.Accent)
	}
	in := a.newDialogInput(value)
	a.dlg = &dialogState{
		kind:        dialogAccentColor,
		seq:         a.dialogSeq,
		title:       "自定义强调色",
		label:       "十六进制颜色",
		placeholder: "#A78BFA",
		hint:        "支持 #RGB 或 #RRGGBB；用于标签、选中项、进度条等强调元素。",
		input:       in,
		hasInput:    true,
	}
	return in.Focus()
}

func (a *App) openTextColorDialog() tea.Cmd {
	a.menu = nil
	a.blurSearch()
	a.dialogSeq++
	in := a.newDialogInput(a.themePrefs.Text)
	a.dlg = &dialogState{
		kind:        dialogTextColor,
		seq:         a.dialogSeq,
		title:       "自定义一般色",
		label:       "十六进制颜色",
		placeholder: string(a.th.Text),
		hint:        "作为正文和歌单/菜单的默认文字色；留空表示跟随主题。",
		input:       in,
		hasInput:    true,
	}
	return in.Focus()
}

func (a *App) openDeletePlaylistConfirm() tea.Cmd {
	name := a.selectedPlaylistName()
	if name == "" || name == "Favorites" {
		a.showToast("Favorites 是默认歌单，不能删除", toastError)
		return nil
	}
	a.menu = nil
	a.blurSearch()
	a.dialogSeq++
	a.dlg = &dialogState{
		kind:         dialogDeletePlaylist,
		seq:          a.dialogSeq,
		title:        "删除歌单",
		hint:         "歌曲文件不会被删除，只会移除这个歌单。",
		target:       name,
		confirmFocus: 0,
	}
	return nil
}

func (a *App) openRemoveSourceConfirm() tea.Cmd {
	src := a.selectedSource()
	if src == nil {
		a.showToast("请先选择一个自定义音源", toastInfo)
		return nil
	}
	a.menu = nil
	a.blurSearch()
	a.dialogSeq++
	a.dlg = &dialogState{
		kind:         dialogRemoveSource,
		seq:          a.dialogSeq,
		title:        "移除音源",
		hint:         "只会移除本地注册记录，不会删除原始脚本文件。",
		target:       src.ID,
		confirmFocus: 0,
	}
	a.dlg.label = src.Name
	return nil
}

func (a *App) confirmLabel() string {
	if a.dlg == nil {
		return "确定"
	}
	switch a.dlg.kind {
	case dialogNewPlaylist:
		return "创建"
	case dialogRenamePlaylist:
		return "保存"
	case dialogImportPlaylist:
		return "下一步"
	case dialogImportPlaylistName:
		return "导入"
	case dialogAddSource:
		return "添加"
	case dialogDownloadDir:
		return "保存"
	case dialogDeletePlaylist:
		return "删除"
	case dialogRemoveSource:
		return "移除"
	case dialogAccentColor, dialogTextColor:
		return "应用"
	default:
		return "确定"
	}
}

func (a *App) submitDialog() tea.Cmd {
	d := a.dlg
	if d == nil || d.busy {
		return nil
	}
	if d.hasInput {
		value := strings.TrimSpace(d.input.Value())
		if d.kind == dialogImportPlaylistName {
			d.busy = true
			d.err = ""
			return cmdImportPlaylist(a.client, d.target, value, d.seq)
		}
		if d.kind == dialogAccentColor || d.kind == dialogTextColor {
			if d.kind == dialogAccentColor && value == "" {
				d.err = "强调色不能为空"
				return nil
			}
			if value != "" {
				normalized, ok := normalizeHex(value)
				if !ok {
					d.err = "颜色格式无效，应为 #RRGGBB 或 #RGB"
					return nil
				}
				value = normalized
			}
			if d.kind == dialogAccentColor {
				a.themePrefs.Accent = value
			} else {
				a.themePrefs.Text = value
			}
			a.applyTheme()
			a.closeDialog()
			a.themeConfigErr = nil
			return a.saveThemeCmd()
		}
		if value == "" {
			d.err = "内容不能为空"
			return nil
		}
		switch d.kind {
		case dialogImportPlaylist:
			d.target = value
			d.kind = dialogImportPlaylistName
			d.label = "本地歌单名称（可选）"
			d.placeholder = "留空使用原歌单名称"
			d.hint = "Enter 导入为本地歌单；如有同名歌单，可修改名称后重试。"
			d.err = ""
			d.input = a.newDialogInput("")
			return d.input.Focus()
		case dialogNewPlaylist:
			d.busy = true
			d.err = ""
			return cmdCreatePlaylist(a.client, value, d.pending, d.seq)
		case dialogRenamePlaylist:
			if value == d.target {
				d.err = "新名称与原名称相同"
				return nil
			}
			d.busy = true
			d.err = ""
			return cmdRenamePlaylist(a.client, d.target, value, d.seq)
		case dialogAddSource:
			d.busy = true
			d.err = ""
			return cmdAddSource(a.client, value, d.seq)
		case dialogDownloadDir:
			d.busy = true
			d.err = ""
			return cmdSetConfig(a.client, "download.output_dir", value, d.seq)
		}
		return nil
	}
	// Confirmation dialogs.
	d.busy = true
	d.err = ""
	switch d.kind {
	case dialogDeletePlaylist:
		return cmdDeletePlaylist(a.client, d.target, d.seq)
	case dialogRemoveSource:
		return cmdRemoveSource(a.client, d.target, d.seq)
	}
	return nil
}

func (a *App) handleDialogKey(msg tea.KeyMsg) tea.Cmd {
	d := a.dlg
	if d == nil {
		return nil
	}
	if d.busy {
		if msg.String() == "esc" {
			a.closeDialog()
		}
		return nil
	}
	if d.hasInput {
		switch msg.String() {
		case "esc":
			a.closeDialog()
			return nil
		case "enter":
			return a.submitDialog()
		default:
			var cmd tea.Cmd
			d.input, cmd = d.input.Update(msg)
			return cmd
		}
	}
	switch msg.String() {
	case "esc", "n":
		a.closeDialog()
		return nil
	case "left", "right", "tab", "shift+tab":
		if d.confirmFocus == 0 {
			d.confirmFocus = 1
		} else {
			d.confirmFocus = 0
		}
		return nil
	case "enter":
		if d.confirmFocus == 1 {
			return a.submitDialog()
		}
		a.closeDialog()
		return nil
	case "y", "Y":
		return a.submitDialog()
	}
	return nil
}

func (a *App) handleDialogMouse(msg tea.MouseMsg) tea.Cmd {
	d := a.dlg
	if d == nil {
		return nil
	}
	if msg.Action == tea.MouseActionMotion {
		for _, h := range d.hits {
			if h.action == dialogHitCancel || h.action == dialogHitConfirm {
				if h.rect.Contains(msg.X, msg.Y) {
					if h.action == dialogHitCancel {
						d.confirmFocus = 0
					} else {
						d.confirmFocus = 1
					}
				}
			}
		}
		return nil
	}
	if msg.Button != tea.MouseButtonLeft || msg.Action != tea.MouseActionPress {
		return nil
	}
	for _, h := range d.hits {
		if !h.rect.Contains(msg.X, msg.Y) {
			continue
		}
		switch h.action {
		case dialogHitCancel:
			a.closeDialog()
			return nil
		case dialogHitConfirm:
			return a.submitDialog()
		case dialogHitInput:
			if d.hasInput {
				return d.input.Focus()
			}
		}
	}
	return nil
}

func (a *App) renderDialogOverlay(base []string) []string {
	d := a.dlg
	if d == nil {
		return base
	}
	t := a.th
	screenW, screenH := a.width, a.height
	width := clamp(54, 70, screenW-6)
	if width > screenW-2 {
		width = screenW - 2
	}
	if width < 36 {
		return base
	}
	contentW := width - 6

	d.hits = d.hits[:0]
	lines := make([]string, 0, 12)

	panelText := func(content string, fg lipgloss.Color) string {
		style := lipgloss.NewStyle().Background(t.PanelBg).Foreground(fg)
		return style.Render("│  " + padVisible(content, contentW) + "  │")
	}

	// Top border with title.
	title := truncate(d.title, width-8)
	label := "─ " + title + " "
	fill := width - 2 - displayWidth(label)
	if fill < 0 {
		fill = 0
	}
	lines = append(lines, lipgloss.NewStyle().
		Background(t.PanelBg).
		Foreground(t.Accent).
		Bold(true).
		Render("╭"+label+strings.Repeat("─", fill)+"╮"))
	lines = append(lines, panelText("", t.PanelFg))

	var localHits []dialogHit
	addLocal := func(rect Rect, action dialogHitAction) {
		localHits = append(localHits, dialogHit{rect: rect, action: action})
	}

	if d.hasInput {
		lines = append(lines, panelText(t.MutedText(d.label), t.Muted))
		inputW := max(8, contentW-4)
		promptW := displayWidth(d.input.Prompt)
		d.input.Width = max(4, inputW-promptW-1)
		view := d.input.View()
		if d.input.Value() == "" {
			cur := lipgloss.NewStyle().Background(t.InputBg).Foreground(t.Accent).Render("▏")
			hint := lipgloss.NewStyle().Background(t.InputBg).Foreground(t.Faint).Render(d.placeholder)
			prompt := lipgloss.NewStyle().Background(t.InputBg).Foreground(t.Accent2).Render("› ")
			view = prompt + cur + hint
		}
		view = truncate(view, inputW)
		view = padVisible(view, inputW)
		lineStyle := lipgloss.NewStyle().
			Background(t.InputBg).
			Foreground(t.Text)
		left := lipgloss.NewStyle().Background(t.PanelBg).Foreground(t.Border).Render("│  ")
		right := lipgloss.NewStyle().Background(t.PanelBg).Foreground(t.Border).Render("  │")
		pad := spaces(max(0, contentW-inputW-2))
		lines = append(lines, left+lineStyle.Render(" "+view+" ")+lineStyle.Render(pad)+right)
		addLocal(Rect{X: 3, Y: len(lines) - 1, W: inputW + 2, H: 1}, dialogHitInput)
	} else {
		message := d.target
		if d.kind == dialogDeletePlaylist {
			message = "确定要删除歌单「" + prettyText(d.target) + "」吗？"
		} else if d.kind == dialogRemoveSource {
			message = "确定要移除音源「" + prettyText(d.label) + "」吗？"
		}
		for _, l := range wrapText(message, contentW) {
			lines = append(lines, panelText(t.TextPlain(l), t.Text))
		}
	}

	if d.hint != "" {
		for _, l := range wrapText(d.hint, contentW) {
			lines = append(lines, panelText(t.FaintText(l), t.Faint))
		}
	}
	if d.err != "" {
		for _, l := range wrapText("✕ "+prettyText(d.err), contentW) {
			lines = append(lines, panelText(t.BadText(l), t.Bad))
		}
	}
	if d.busy {
		lines = append(lines, panelText(a.th.Spinner(a.spinner)+" 正在处理…", t.Accent2))
	}
	lines = append(lines, panelText("", t.PanelFg))

	// Buttons row.
	cancelLabel := " 取消"
	confirmLabel := " " + a.confirmLabel()
	cancelSelected := false
	confirmSelected := false
	if d.hasInput {
		confirmSelected = true
	} else if d.confirmFocus == 0 {
		cancelSelected = true
	} else {
		confirmSelected = true
	}
	if cancelSelected {
		cancelLabel = "▸取消"
	}
	if confirmSelected {
		confirmLabel = "▸" + a.confirmLabel()
	}
	cancelW := displayWidth(cancelLabel) + 2
	confirmW := displayWidth(confirmLabel) + 2
	gap := 2
	total := cancelW + gap + confirmW
	prefixW := contentW - total
	if prefixW < 0 {
		prefixW = 0
	}
	buttonStyle := func(label string, selected, danger bool) string {
		style := lipgloss.NewStyle().Background(t.PanelBg).Foreground(t.Muted).Padding(0, 1)
		if selected {
			style = lipgloss.NewStyle().Background(t.AccentStrong).Foreground(t.OnAccent).Bold(true).Padding(0, 1)
			if danger {
				style = lipgloss.NewStyle().Background(t.Bad).Foreground(t.White).Bold(true).Padding(0, 1)
			}
		}
		return style.Render(label)
	}
	dangerConfirm := d.kind == dialogDeletePlaylist || d.kind == dialogRemoveSource
	cancelBtn := buttonStyle(cancelLabel, cancelSelected, false)
	confirmBtn := buttonStyle(confirmLabel, confirmSelected, dangerConfirm)

	leftBorder := lipgloss.NewStyle().Background(t.PanelBg).Foreground(t.Border).Render("│  ")
	rightBorder := lipgloss.NewStyle().Background(t.PanelBg).Foreground(t.Border).Render("  │")
	buttonPrefix := lipgloss.NewStyle().Background(t.PanelBg).Render(spaces(prefixW))
	buttonGap := lipgloss.NewStyle().Background(t.PanelBg).Render(spaces(gap))
	buttonsLine := leftBorder + buttonPrefix + cancelBtn + buttonGap + confirmBtn + rightBorder
	lines = append(lines, buttonsLine)

	buttonY := len(lines) - 1
	buttonX := 3 + prefixW
	if !d.busy {
		addLocal(Rect{X: buttonX, Y: buttonY, W: cancelW, H: 1}, dialogHitCancel)
		addLocal(Rect{X: buttonX + cancelW + gap, Y: buttonY, W: confirmW, H: 1}, dialogHitConfirm)
	}

	bottomFill := width - 2
	lines = append(lines, lipgloss.NewStyle().Background(t.PanelBg).Foreground(t.Border).Render("╰"+strings.Repeat("─", bottomFill)+"╯"))

	// Compute placement and translate local hit coordinates.
	h := len(lines)
	x := (screenW - width) / 2
	y := (screenH - h) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if y+h > screenH {
		y = max(0, screenH-h)
	}
	d.x, d.y, d.w, d.h = x, y, width, h
	for _, hit := range localHits {
		r := hit.rect
		r.X += d.x
		r.Y += d.y
		d.hits = append(d.hits, dialogHit{rect: r, action: hit.action})
	}
	if d.hasInput {
		d.input.Width = max(4, contentW-4-displayWidth(d.input.Prompt)-1)
	}
	return overlayLines(base, lines, x, y, screenW)
}
