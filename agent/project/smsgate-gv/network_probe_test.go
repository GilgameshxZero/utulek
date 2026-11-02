package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func TestLiveNetworkMessageIdentity(t *testing.T) {
	if os.Getenv("GV_LIVE_NETWORK") != "1" {
		t.Skip("explicit opt-in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	home := os.Getenv("SMSGATE_GV_HOME")
	c, e := readConfig(home)
	if e != nil {
		t.Fatal(e)
	}
	number := os.Getenv("GV_REMOTE_NUMBER")
	if !phonePattern.MatchString(number) {
		t.Fatal("explicit remote required")
	}
	threadID := "t." + number
	var samples [][]VoiceMessage
	var uiMatched bool
	for session := 0; session < 2; session++ {
		b, e := startBrowser(ctx, home, c)
		if e != nil {
			t.Fatal(e)
		}
		func() {
			defer b.close()
			var mu sync.Mutex
			var ids []network.RequestID
			chromedp.ListenTarget(b.ctx, func(ev any) {
				if r, ok := ev.(*network.EventResponseReceived); ok && strings.Contains(r.Response.URL, "/voice/v1/voiceclient/api2thread/list") && r.Type == network.ResourceTypeXHR {
					mu.Lock()
					ids = append(ids, r.RequestID)
					mu.Unlock()
				}
			})
			if e = b.run(network.Enable(), network.SetCacheDisabled(true), emulation.SetTimezoneOverride("UTC"), chromedp.Navigate(b.base+"messages")); e != nil {
				t.Fatal(e)
			}
			var ready bool
			if e = b.run(chromedp.Poll(`document.querySelectorAll('gv-thread-list-item').length>0`, &ready, chromedp.WithPollingTimeout(20*time.Second))); e != nil {
				t.Fatal(e)
			}
			var selected bool
			js := `(()=>{const m=[...document.querySelectorAll('gv-thread-list-item')].filter(e=>(e.querySelector('.participants')?.innerText||'').replace(/\D/g,'').endsWith(` + literal(number[len(number)-10:]) + `));if(m.length!==1)return false;m[0].querySelector('[role="button"]').click();return true})()`
			if e = b.run(chromedp.Evaluate(js, &selected)); e != nil || !selected {
				t.Fatal("target missing", e)
			}
			rounds := 1
			if session == 0 {
				rounds = 2
			}
			for round := 0; round < rounds; round++ {
				if e = b.run(chromedp.Poll(`document.querySelectorAll('gv-message-item').length>0&&new URL(location.href).searchParams.get('itemId')===`+literal(threadID), &ready, chromedp.WithPollingTimeout(20*time.Second))); e != nil {
					t.Fatal(e)
				}
				mu.Lock()
				candidates := append([]network.RequestID(nil), ids...)
				mu.Unlock()
				var thread []json.RawMessage
				for i := len(candidates) - 1; i >= 0; i-- {
					var body []byte
					if e = b.run(chromedp.ActionFunc(func(ctx context.Context) error {
						var err error
						body, err = network.GetResponseBody(candidates[i]).Do(ctx)
						return err
					})); e != nil {
						continue
					}
					if thread, e = voiceThread(body, threadID); e == nil {
						break
					}
				}
				if thread == nil {
					t.Fatal("target thread response not captured")
				}
				rows, e := decodeVoiceThread(thread, c.Number)
				if e != nil {
					t.Fatal(e)
				}
				sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
				samples = append(samples, rows)
				if e = writeJSON(filepath.Join(".obj", fmt.Sprintf("network_thread_sample_%d.json", len(samples))), thread); e != nil {
					t.Fatal(e)
				}
				if session == 0 && round == 0 {
					type visible struct {
						Text  string `json:"text"`
						Out   bool   `json:"out"`
						Label string `json:"label"`
					}
					var dom []visible
					js = `(()=>[...document.querySelectorAll('gv-message-item')].map(e=>({text:[...e.querySelectorAll('gv-annotation.content')].map(x=>x.innerText).join('\n'),out:!!e.querySelector('.full-container.outgoing'),label:e.querySelector('.container > .cdk-visually-hidden')?.innerText||''})))()`
					if e = b.run(chromedp.Evaluate(js, &dom)); e != nil {
						t.Fatal(e)
					}
					if len(dom) != len(rows) {
						t.Fatalf("network=%d DOM=%d", len(rows), len(dom))
					}
					for _, m := range rows {
						stamp, e := time.Parse(time.RFC3339Nano, m.Timestamp)
						if e != nil {
							t.Fatal(e)
						}
						matched := false
						for _, d := range dom {
							if strings.TrimSpace(d.Text) == strings.TrimSpace(m.Text) && d.Out == m.Outgoing && strings.Contains(strings.Join(strings.Fields(d.Label), " "), stamp.Format("Monday, January 2 2006, 3:04 PM")) {
								matched = true
								break
							}
						}
						if !matched {
							t.Fatal("network text/direction/time does not match visible message")
						}
					}
					uiMatched = true
				}
				if round+1 < rounds {
					mu.Lock()
					ids = nil
					mu.Unlock()
					if e = b.run(chromedp.Reload()); e != nil {
						t.Fatal(e)
					}
				}
			}
		}()
	}
	if len(samples) != 3 || !reflect.DeepEqual(samples[0], samples[1]) || !reflect.DeepEqual(samples[0], samples[2]) {
		t.Fatal("native IDs/content/timestamps changed across reload or browser restart")
	}
	incoming, outgoing, media := 0, 0, 0
	byText := map[string][]string{}
	for _, m := range samples[0] {
		if m.Outgoing {
			outgoing++
		} else {
			incoming++
		}
		if len(m.AttachmentIDs) > 0 {
			media++
		}
		byText[m.Text] = append(byText[m.Text], m.ID)
	}
	repeated := 0
	for _, ids := range byText {
		if len(ids) > 1 {
			repeated += len(ids)
		}
	}
	result := map[string]any{"checkedAt": time.Now().UTC().Format(time.RFC3339), "scope": "authorized Android conversation; no sends", "endpoint": "/voice/v1/voiceclient/api2thread/list", "messages": len(samples[0]), "incoming": incoming, "outgoing": outgoing, "media": media, "native_ids_unique": true, "utc_millisecond_timestamps": true, "stable_across_reload": true, "stable_across_browser_restart": true, "ui_text_direction_time_matched": uiMatched, "identical_text_messages_with_distinct_ids": repeated, "records": samples[0]}
	if e = writeJSON(filepath.Join(".obj", "network_identity_results.json"), result); e != nil {
		t.Fatal(e)
	}
	t.Logf("%d unique native IDs, %d incoming/%d outgoing/%d media; %d repeated-text messages remain distinct; reload/restart/UI comparisons passed", len(samples[0]), incoming, outgoing, media, repeated)
}
