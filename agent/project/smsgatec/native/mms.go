package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type Attachment struct {
	PartID      int64  `json:"partId"`
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	Local       string `json:"local_path,omitempty"`
	Inline      []byte `json:"-"`
}
type WireAttachment struct {
	ContentType string `json:"contentType"`
	Name        string `json:"name,omitempty"`
	Data        []byte `json:"data"`
}
type MMSPayload struct {
	Subject     string           `json:"subject,omitempty"`
	Text        string           `json:"text,omitempty"`
	Attachments []WireAttachment `json:"attachments,omitempty"`
}
type attachmentChoice struct {
	Message Message
	Part    Attachment
}
type picker struct {
	Kind, Action, Directory, Filter string
	Selected, Top                   int
	ReturnView                      string
	Choices                         []attachmentChoice
	Files                           []fileChoice
}
type fileChoice struct {
	Path, Name string
	Directory  bool
}

func safeName(name, contentType string) string {
	name = filepath.Base(strings.ReplaceAll(clean(name), "\\", "_"))
	if name == "" || name == "." || strings.HasPrefix(name, ".") {
		name = "attachment"
	}
	if filepath.Ext(name) == "" {
		extensions, _ := mime.ExtensionsByType(contentType)
		if len(extensions) > 0 {
			name += extensions[0]
		}
	}
	return name
}
func mediaFile(root string, part Attachment, data []byte) (string, error) {
	hash := sha256.Sum256(data)
	name := hex.EncodeToString(hash[:]) + "-" + safeName(part.Name, part.ContentType)
	path := filepath.Join(root, name)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	return path, privateWrite(path, data)
}
func (a *App) cacheMessages(messages []Message) error {
	for i := range messages {
		// Refreshes return media metadata; keep already downloaded bytes/paths.
		if old, ok := a.Cache.records[messages[i].key()]; ok {
			messages[i] = mergeMessage(old, messages[i])
		}

		for j := range messages[i].Media {
			part := &messages[i].Media[j]
			if len(part.Inline) > 0 && a.Cache.path != "" {
				path, err := mediaFile(filepath.Join(filepath.Dir(a.Cache.path), "media"), *part, part.Inline)
				if err != nil {
					return err
				}
				part.Local = path
				part.Inline = nil
			}
		}
	}
	return a.Cache.put(messages)
}
func (g *Gateway) sendMMS(ctx context.Context, id, number, text, subject string, parts []WireAttachment) ([]Message, error) {
	var row APIMessage
	payload := MMSPayload{Text: text, Subject: subject, Attachments: parts}
	_, err := g.request(ctx, "POST", "/message", map[string]any{"id": id, "phoneNumbers": []string{number}, "mmsMessage": payload}, &row)
	if err != nil {
		return nil, err
	}
	if row.ID == "" {
		return nil, &APIError{"MMS send outcome unknown", 0, true}
	}
	row.MMS = &payload
	return row.messages(), nil
}
func readMedia(path string) (WireAttachment, error) {
	file, err := os.Open(path)
	if err != nil {
		return WireAttachment{}, errors.New("cannot read attachment")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return WireAttachment{}, errors.New("select a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, 20*1024*1024+1))
	if err != nil {
		return WireAttachment{}, err
	}
	if len(data) > 20*1024*1024 {
		return WireAttachment{}, errors.New("attachment exceeds 20 MiB")
	}
	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	if cut := strings.Index(contentType, ";"); cut >= 0 {
		contentType = contentType[:cut]
	}
	return WireAttachment{contentType, filepath.Base(path), data}, nil
}
func (g *Gateway) download(ctx context.Context, messageID string, part Attachment) ([]byte, error) {
	path := fmt.Sprintf("/inbox/%s/attachments/%d", url.PathEscape(messageID), part.PartID)
	req, err := http.NewRequestWithContext(ctx, "GET", g.config.Server+path, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(g.config.Username, g.config.Password)
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, errors.New("attachment download failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("attachment HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024*1024+1))
	if err != nil || len(data) > 64*1024*1024 {
		return nil, errors.New("attachment is incomplete or exceeds 64 MiB")
	}
	return data, nil
}
func attachmentBytes(ctx context.Context, g *Gateway, choice attachmentChoice) ([]byte, error) {
	if choice.Part.Local != "" {
		return os.ReadFile(choice.Part.Local)
	}
	if len(choice.Part.Inline) > 0 {
		return append([]byte{}, choice.Part.Inline...), nil
	}
	if choice.Message.Direction != "in" {
		return nil, errors.New("outgoing attachment was not cached; gateway cannot download it")
	}
	return g.download(ctx, choice.Message.ID, choice.Part)
}
func saveExclusive(path string, data []byte) error {
	path = expand(path)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		file.Close()
		if !ok {
			os.Remove(path)
		}
	}()
	if err = protectFile(path); err != nil {
		return err
	}
	if _, err = file.Write(data); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	ok = true
	return file.Close()
}
func (a *App) openAttachmentPicker(action string) {
	choices := []attachmentChoice{}
	messages := sortedMessages(a.Cache.records, a.Number)
	for i := len(messages) - 1; i >= 0; i-- {
		for _, part := range messages[i].Media {
			choices = append(choices, attachmentChoice{messages[i], part})
		}
	}
	if len(choices) == 0 {
		a.note("no downloaded attachments in this conversation")
		return
	}
	a.Picker = &picker{Kind: "attachments", Action: action, ReturnView: a.View, Choices: choices}
	a.View = "picker"
}
func (a *App) openFilePicker() {
	directory := a.LastFileDir
	if directory == "" {
		directory = home()
	}
	a.Picker = &picker{Kind: "files", Directory: directory, ReturnView: a.View}
	a.View = "picker"
	a.readDirectory()
}
func (a *App) readDirectory() {
	p := a.Picker
	entries, err := os.ReadDir(p.Directory)
	if err != nil {
		a.note("cannot read directory")
		return
	}
	files := []fileChoice{{filepath.Dir(p.Directory), "../", true}}
	for _, entry := range entries {
		if p.Filter != "" && !strings.Contains(strings.ToLower(entry.Name()), strings.ToLower(p.Filter)) {
			continue
		}
		path := filepath.Join(p.Directory, entry.Name())
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			continue
		}
		name := entry.Name()
		if info.IsDir() {
			name += "/"
		}
		files = append(files, fileChoice{path, name, info.IsDir()})
	}
	sort.Slice(files[1:], func(i, j int) bool {
		a, b := files[i+1], files[j+1]
		if a.Directory != b.Directory {
			return a.Directory
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	p.Files = files
	p.Selected = 0
	p.Top = 0
}
func (a *App) pickerBack() { a.View = a.Picker.ReturnView; a.Picker = nil }
func (a *App) pickerKey(key, height int) {
	p := a.Picker
	count := len(p.Choices)
	if p.Kind == "files" {
		count = len(p.Files)
	}
	switch key {
	case 'q', 'b', 27:
		a.pickerBack()
	case 'j', keyDown:
		p.Selected = min(count-1, p.Selected+1)
	case 'k', keyUp:
		p.Selected = max(0, p.Selected-1)
	case keyPageUp:
		p.Selected = max(0, p.Selected-max(1, height-1))
	case keyPageDown:
		p.Selected = min(count-1, p.Selected+max(1, height-1))
	case 'g':
		p.Selected = 0
	case 'G':
		p.Selected = max(0, count-1)
	case 'h', keyLeft:
		if p.Kind == "files" {
			p.Directory = filepath.Dir(p.Directory)
			a.readDirectory()
		}
	case '/':
		if p.Kind == "files" && a.Prompt != nil {
			query, ok := a.Prompt("Filter filenames or enter full path", p.Filter, false)
			if ok {
				if strings.HasPrefix(query, "/") || strings.HasPrefix(query, "~/") {
					path := expand(query)
					info, err := os.Stat(path)
					if err != nil {
						a.note("path not found")
						return
					}
					if info.IsDir() {
						p.Directory = path
						p.Filter = ""
						a.readDirectory()
						return
					}
					if _, err = readMedia(path); err != nil {
						a.note(err.Error())
						return
					}
					a.DraftFiles = append(a.DraftFiles, path)
					a.LastFileDir = filepath.Dir(path)
					a.pickerBack()
					a.Phase = "review"
					return
				}
				p.Filter = query
				a.readDirectory()
			}
		}
	case 'o':
		if p.Kind == "attachments" {
			p.Action = "open"
		}
	case 's':
		if p.Kind == "attachments" {
			p.Action = "save"
		}
	case 10, 13, keyEnter:
		if count == 0 {
			return
		}
		if p.Kind == "files" {
			entry := p.Files[p.Selected]
			if entry.Directory {
				p.Directory = entry.Path
				p.Filter = ""
				a.readDirectory()
				return
			}
			if _, err := readMedia(entry.Path); err != nil {
				a.note(err.Error())
				return
			}
			a.DraftFiles = append(a.DraftFiles, entry.Path)
			a.LastFileDir = p.Directory
			a.pickerBack()
			a.Phase = "review"
			return
		}
		choice := p.Choices[p.Selected]
		action := p.Action
		destination := ""
		if action == "save" {
			if a.Prompt == nil {
				return
			}
			value, ok := a.Prompt("Save attachment to", filepath.Join(home(), "Downloads", safeName(choice.Part.Name, choice.Part.ContentType)), false)
			if !ok {
				return
			}
			destination = expand(value)
			if _, err := os.Stat(destination); err == nil {
				a.note("file exists; choose another name")
				return
			}
		}
		a.pickerBack()
		gateway := a.Gateway
		root := a.MediaTemp
		if a.Cache.path != "" {
			root = filepath.Join(filepath.Dir(a.Cache.path), "media")
		}
		a.job("attachment", func() result {
			data, err := attachmentBytes(a.Context, gateway, choice)
			if err != nil {
				return result{Err: err}
			}
			path, err := mediaFile(root, choice.Part, data)
			if err != nil {
				return result{Err: err}
			}
			returnResult := result{MediaKey: choice.Message.key(), MediaPart: choice.Part.PartID, MediaPath: path}
			if action == "save" {
				if err = saveExclusive(destination, data); err != nil {
					return result{Err: err}
				}
				returnResult.Notice = "saved " + filepath.Base(destination)
				return returnResult
			}
			returnResult.OpenPath = path
			return returnResult
		})
	}
}
func launchOpener(path string) error {
	name := "xdg-open"
	if runtime.GOOS == "windows" {
		name = "explorer.exe"
	}
	opener, err := exec.LookPath(name)
	if err != nil {
		return errors.New("xdg-open is not installed; use s to save instead")
	}
	cmd := exec.Command(opener, path)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err = cmd.Start(); err != nil {
		return errors.New("cannot open attachment")
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
