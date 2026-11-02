package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"
)

func main() {
	runtime.LockOSThread()
	path := flag.String("config", configPath(), "private JSON configuration path")
	theme := flag.String("theme", "", "auto, light, or dark")
	noCache := flag.Bool("no-cache", false, "keep message history in memory only")
	demo := flag.Bool("demo", false, "offline sample; sends and disk state disabled")
	showVersion := flag.Bool("version", false, "print version")
	check := flag.Bool("check", false, "read gateway history and report counts; no sends")
	flag.Parse()
	if *showVersion {
		fmt.Println("smsgatec", version, "native", runtime.GOOS+"/"+runtime.GOARCH)
		return
	}
	c, err := loadConfig(expand(*path))
	if *demo {
		c = Config{Poll: 10, Timeout: 20, Theme: "auto"}
		err = nil
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot load configuration:", err)
		os.Exit(2)
	}
	if *theme != "" {
		c.Theme = *theme
	}
	if *noCache {
		c.Cache = false
	}
	if err = c.validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	app, err := newApp(c, expand(*path), *demo)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot open message cache:", err)
		os.Exit(2)
	}
	defer app.close()
	if *check {
		if !*demo {
			started := time.Now()
			messages, err := app.Gateway.syncSince(context.Background(), app.knownOutgoing(), app.SyncSince)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return
			}
			if err = app.cacheMessages(messages); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return
			}
			app.SyncSince = started
			app.saveSyncCursor()
			if contacts, err := readContacts(context.Background(), c.VCFDir, ""); err == nil {
				app.Contacts = contacts
			}
		}
		fmt.Println(app.summary())
		return
	}
	if err = app.run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
