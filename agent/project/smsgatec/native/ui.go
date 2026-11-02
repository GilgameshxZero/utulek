package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type drawLine struct {
	Y, X int
	Text string
	Attr int
}
type point struct{ Y, X int }
type frame struct {
	Lines         []drawLine
	Cursor        *point
	Width, Height int
}

func (f *frame) add(y, x int, text string, attr int) {
	if y < 0 || y >= f.Height || x < 0 || x >= f.Width-1 {
		return
	}
	f.Lines = append(f.Lines, drawLine{y, x, clip(text, f.Width-x-1), attr})
}
func (a *App) footer(w int, hints string) string {
	if time.Now().Before(a.StatusUntil) {
		hints = a.Status
	}
	// Background refresh is deliberately silent.
	if (a.View == "index" || a.View == "contacts" || a.View == "thread") && (a.Search || a.Query != "") {
		field := clip("/"+a.Query+func() string {
			if a.Search {
				return "▏"
			}
			return ""
		}(), max(3, w-24))
		return field + " · " + hints
	}
	if a.View == "thread" {
		budget := min(18, w/5)
		if w < 35 {
			budget = 3
		}
		return clip(a.contact(a.Number).Name, budget) + " · " + hints
	}
	if a.Demo {
		return hints + " [demo]"
	}
	return hints
}
func (a *App) history(w int) []drawLine {
	sending := a.Jobs["send"]
	if a.HistoryCache != nil && a.HistoryRevision == a.Cache.revision && a.HistoryWidth == w && a.HistoryNumber == a.Number && a.HistoryQuery == a.Query && a.HistorySending == sending {
		return a.HistoryCache
	}

	lines := []drawLine{}
	query := strings.ToLower(a.Query)
	for _, m := range sortedMessages(a.Cache.records, a.Number) {
		if query != "" && !strings.Contains(strings.ToLower(m.Text), query) {
			continue
		}
		sender := a.contact(m.Number).Name
		if m.Direction == "out" {
			sender = "You"
		}
		title := sender + " · " + m.dateLabel()
		if strings.HasPrefix(m.Type, "MMS") {
			title += " · MMS"
		}
		if m.Direction == "out" {
			state := m.State
			if a.Sending != nil && a.Sending.ID == m.ID && a.Jobs["send"] {
				state = "Sending…"
			}
			title += " · " + state
		}
		lines = append(lines, drawLine{Text: title, Attr: attrBold})
		if m.Subject != "" {
			for _, line := range wrapped("Subject: "+m.Subject, w) {
				lines = append(lines, drawLine{Text: line})
			}
		}
		for _, line := range wrapped(m.Text, w) {
			lines = append(lines, drawLine{Text: line})
		}
		for _, part := range m.Media {
			label := "[attachment: " + safeName(part.Name, part.ContentType) + " · " + part.ContentType
			if part.Size > 0 {
				label += fmt.Sprintf(" · %d KB", (part.Size+1023)/1024)
			}
			label += "]"
			for _, line := range wrapped(label, w) {
				lines = append(lines, drawLine{Text: line, Attr: attrDim})
			}
		}
		lines = append(lines, drawLine{})
	}
	if len(lines) == 0 {
		lines = append(lines, drawLine{Text: "No messages yet.", Attr: attrDim})
	}
	a.HistoryCache = lines
	a.HistoryRevision = a.Cache.revision
	a.HistoryWidth = w
	a.HistoryNumber = a.Number
	a.HistoryQuery = a.Query
	a.HistorySending = sending
	return lines
}
func visualEditor(body []rune, cursor, limit int) ([]string, int, int) {
	lines := []string{}
	line := ""
	row, col := 0, 0
	for i, r := range body {
		if i == cursor {
			row, col = len(lines), width(line)
		}
		if r == '\n' {
			lines = append(lines, line)
			line = ""
			continue
		}
		if line != "" && width(line)+runeWidth(r) > limit {
			lines = append(lines, line)
			line = ""
			if i == cursor {
				row, col = len(lines), 0
			}
		}
		line += string(r)
	}
	if cursor == len(body) {
		row, col = len(lines), width(line)
	}
	return append(lines, line), row, col
}
func (a *App) render(w, h int) frame {
	f := frame{Width: w, Height: h}
	if w < 25 || h < 8 {
		f.add(0, 0, "Enlarge terminal (25×8)", 0)
		f.add(h-1, 0, "q quit", attrReverse)
		return f
	}
	hints := ""
	switch a.View {
	case "accounts":
		if a.Selected < a.Top {
			a.Top = a.Selected
		}
		if a.Selected >= a.Top+h-1 {
			a.Top = a.Selected - h + 2
		}
		for i := a.Top; i < len(a.Config.Accounts) && i < a.Top+h-1; i++ {
			item := a.Config.Accounts[i]
			label := item.Name
			if item.ID == a.Config.ActiveAccount {
				label += " · current"
			}
			attr := 0
			if i == a.Selected {
				attr = attrReverse
			}
			f.add(i-a.Top, 0, label, attr)
		}
		hints = "Accounts · Enter select · Esc back"
	case "index", "contacts":
		rows := a.entries()
		a.Selected = max(0, min(a.Selected, len(rows)))
		if a.Selected < a.Top {
			a.Top = a.Selected
		}
		if a.Selected >= a.Top+h-1 {
			a.Top = a.Selected - h + 2
		}
		for position := a.Top; position <= len(rows) && position < a.Top+h-1; position++ {
			row := position - a.Top
			attr := 0
			if position == a.Selected {
				attr = attrReverse
			}
			if position == 0 {
				label := "+ New conversation…"
				if a.View == "contacts" {
					label = "+ Enter a phone number…"
					if strings.HasPrefix(a.Query, "+") {
						label = "+ Use " + a.Query
					}
				}
				f.add(row, 0, label, attr|attrBold)
				continue
			}
			number := rows[position-1]
			c := a.contact(number)
			right := number
			if c.Kind != "" {
				right = c.Kind + " " + right
			}
			nameWidth := max(8, w-min(width(right), w/2)-3)
			f.add(row, 0, clip(c.Name, nameWidth), attr)
			f.add(row, nameWidth+2, right, attr|attrDim)
		}
		hints = "j/k Enter open · n new · y accounts · / · q quit"
		if a.View == "contacts" {
			hints = "j/k Enter open · / · q back"
		}
		if a.Search {
			hints = "Enter open · Esc cancel"
		}
	case "thread":
		editorRows := 0
		metaRows := 0
		if a.Subject != "" {
			metaRows++
		}
		if len(a.DraftFiles) > 0 {
			metaRows++
		}
		if a.Phase != "read" || len(a.Body) > 0 || metaRows > 0 {
			editorRows = min(h-3, min(5, max(2+metaRows, h/5)))
		}
		historyRows := h - 1
		if editorRows > 0 {
			historyRows -= editorRows + 1
		}
		lines := a.history(w - 1)
		a.Scroll = min(a.Scroll, max(0, len(lines)-historyRows))
		start := max(0, len(lines)-historyRows-a.Scroll)
		for i := 0; i < historyRows && start+i < len(lines); i++ {
			line := lines[start+i]
			f.add(i, 0, line.Text, line.Attr)
		}
		if editorRows > 0 {
			separator := h - editorRows - 2
			f.add(separator, 0, strings.Repeat("─", w-1), attrDim)
			visual, row, col := visualEditor(a.Body, a.Cursor, w-1)
			bodyRows := max(1, editorRows-metaRows)
			top := max(0, row-bodyRows+1)
			for i := top; i < len(visual) && i < top+bodyRows; i++ {
				f.add(separator+1+i-top, 0, visual[i], 0)
			}
			metaRow := h - 2 - metaRows + 1
			if a.Subject != "" {
				f.add(metaRow, 0, "Subject: "+a.Subject, attrDim)
				metaRow++
			}
			if len(a.DraftFiles) > 0 {
				f.add(metaRow, 0, fmt.Sprintf("[%d attachment(s)] %s", len(a.DraftFiles), filepath.Base(a.DraftFiles[len(a.DraftFiles)-1])), attrDim)
			}
			if a.Phase == "compose" {
				f.Cursor = &point{min(separator+bodyRows, separator+1+row-top), min(w-1, col)}
			}
		}
		hints = "Enter write · q back · o open · s save · /"
		if a.Phase == "compose" {
			hints = "Enter/Esc review · Shift+Enter newline"
		}
		if a.Phase == "review" {
			hints = "y/Enter send · e edit · a attach · s subject · q back"
		}
		if a.Search {
			hints = "Enter keep · Esc cancel"
		}
	case "help":
		instructions := []string{
			"j/k or arrows select recipients and scroll history. Enter opens.",
			"y in a list opens Accounts; Enter selects an account.",
			"n opens New conversation: search khard or enter a number.",
			"/ searches. Esc cancels and restores the previous position.",
			"Enter accepts search. q clears the filter before going back.",
			"Conversation: Enter writes. Compose: Enter or Esc finishes.",
			"Shift+Enter adds a newline (F4 is the portable fallback).",
			"Review: y or Enter sends; e edits. E uses $VISUAL/$EDITOR.",
			"MMS: a attaches a file; d removes the last draft attachment.",
			"MMS review: s sets/clears the subject; attachment or subject selects MMS.",
			"Conversation: o chooses an attachment to open; s saves one.",
			"q goes back, preserving selection, filter, and scroll position.",
			"q at the unfiltered recipient index quits. g/G first/last.",
			"r/F5 refreshes silently; I/F6 imports old incoming SMS.",
			"c/F2 connection; t automatic/light/dark appearance.",
		}
		row := 0
		for _, line := range instructions {
			for _, part := range wrapped(line, w-1) {
				if row < h-1 {
					f.add(row, 0, part, 0)
					row++
				}
			}
		}
		hints = "q/Esc/b/? back"
	case "picker":
		p := a.Picker
		count := len(p.Choices)
		if p.Kind == "files" {
			count = len(p.Files)
		}
		p.Selected = max(0, min(p.Selected, max(0, count-1)))
		if p.Selected < p.Top {
			p.Top = p.Selected
		}
		if p.Selected >= p.Top+h-1 {
			p.Top = p.Selected - h + 2
		}
		for i := p.Top; i < count && i < p.Top+h-1; i++ {
			label := ""
			if p.Kind == "files" {
				label = p.Files[i].Name
			} else {
				choice := p.Choices[i]
				label = choice.Message.dateLabel() + " · " + safeName(choice.Part.Name, choice.Part.ContentType)
				if choice.Part.Size > 0 {
					label += fmt.Sprintf(" · %d KB", (choice.Part.Size+1023)/1024)
				}
			}
			attr := 0
			if i == p.Selected {
				attr = attrReverse
			}
			f.add(i-p.Top, 0, label, attr)
		}
		if p.Kind == "files" {
			hints = "Enter attach · h parent · / filter · q back"
		} else if p.Action == "save" {
			hints = "Enter save · o open · q back"
		} else {
			hints = "Enter open · s save · q back"
		}
	}
	f.add(h-1, 0, a.footer(w, hints), attrReverse)
	return f
}
func paint(f frame) {
	terminalClear()
	for _, line := range f.Lines {
		put(line.Y, line.X, line.Text, line.Attr)
	}
	if f.Cursor != nil {
		terminalCursor(f.Cursor.Y, f.Cursor.X)
	}
	terminalFlush()
}
func (a *App) prompt(title, initial string, secret bool) (string, bool) {
	body := []rune(initial)
	cursor := len(body)
	for a.Running {
		a.drain()
		h, w := terminalSize()
		f := frame{Width: w, Height: h}
		f.add(0, 0, title, attrBold)
		shown := string(body)
		before := string(body[:cursor])
		if secret {
			shown = strings.Repeat("*", len(body))
			before = strings.Repeat("*", cursor)
		}
		left := max(0, width(before)-w+2)
		if left > 0 {
			runes := []rune(shown)
			for len(runes) > 0 && left > 0 {
				left -= runeWidth(runes[0])
				runes = runes[1:]
			}
			shown = string(runes)
		}
		f.add(2, 0, shown, 0)
		f.add(h-1, 0, "Enter accept · Esc cancel", attrReverse)
		f.Cursor = &point{min(h-2, 2), min(w-2, width(before))}
		paint(f)
		key := terminalRead()
		switch key {
		case 27:
			return "", false
		case 10, 13, keyEnter:
			return string(body), true
		case 8, 127, keyBackspace:
			if cursor > 0 {
				body = append(body[:cursor-1], body[cursor:]...)
				cursor--
			}
		case keyDelete:
			if cursor < len(body) {
				body = append(body[:cursor], body[cursor+1:]...)
			}
		case keyLeft:
			cursor = max(0, cursor-1)
		case keyRight:
			cursor = min(len(body), cursor+1)
		case keyHome:
			cursor = 0
		case keyEnd:
			cursor = len(body)
		default:
			if key >= 32 && key < 0x110000 {
				r := rune(key)
				tail := append([]rune{}, body[cursor:]...)
				body = append(body[:cursor], r)
				body = append(body, tail...)
				cursor++
			}
		}
	}
	return "", false
}
func (a *App) run() error {
	if err := terminalInit(); err != nil {
		return err
	}
	defer terminalEnd()
	terminalTheme(a.Config.Theme)
	a.Prompt = a.prompt
	if !a.Demo {
		a.contacts("")
		a.sync()
		if !a.Config.ready() {
			a.note("c configures the gateway")
		}
	}
	a.Dirty = true
	lastH, lastW := 0, 0
	statusVisible := false
	for a.Running {
		a.tick()
		h, w := terminalSize()
		visible := time.Now().Before(a.StatusUntil)
		if a.Dirty || h != lastH || w != lastW || visible != statusVisible {
			paint(a.render(w, h))
			a.Dirty = false
			lastH, lastW = h, w
			statusVisible = visible
		}
		key := terminalRead()
		if key != -1 {
			a.key(key, h)
			a.Dirty = true
		}
	}
	return nil
}
