package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const usage = `smsgate_gv 0.1.0 (Linux)

Commands:
  init [--number +15551234567]  Create private configuration; prompts for number
  auth-check                  Verify headless authentication without reading or sending messages
  login                       Open a separate Chromium window for sign-in / MFA
  login --remote              Sign in through loopback DevTools over SSH (headless server)
  serve                       Run the HTTP API with a headless browser
  status                      Read the running service's authenticated health
  doctor                      Check configuration, state, permissions and service health
  credentials                 Show local API credentials explicitly
  paths                       Show configuration and browser profile paths
  version                     Show version

State: $SMSGATE_GV_HOME, or $XDG_STATE_HOME/smsgate_gv,
       or ~/.local/state/smsgate_gv
The service and login share an exclusive profile lock. Stop serve before login.
`

func main() {
	syscall.Umask(0077)
	if e := mainErr(); e != nil {
		fmt.Fprintln(os.Stderr, "Error:", e)
		os.Exit(1)
	}
}
func mainErr() error {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		return nil
	}
	cmd := os.Args[1]
	home, e := filepath.Abs(homeDir())
	if e != nil {
		return e
	}
	if cmd == "help" || cmd == "--help" || cmd == "-h" {
		fmt.Print(usage)
		return nil
	}
	if cmd == "version" {
		fmt.Println("smsgate_gv 0.1.0")
		return nil
	}
	if cmd == "paths" {
		fmt.Println("Configuration:", filepath.Join(home, "config.json"))
		fmt.Println("Profile:", filepath.Join(home, "profile"))
		fmt.Println("Messages:", filepath.Join(home, "messages.json"))
		return nil
	}
	if cmd == "init" {
		fs := flag.NewFlagSet("init", flag.ContinueOnError)
		number := fs.String("number", "", "Google Voice number in international format")
		if e := fs.Parse(os.Args[2:]); e != nil {
			return e
		}
		lock, e := lockHome(home)
		if e != nil {
			return e
		}
		defer lock.Close()
		path := filepath.Join(home, "config.json")
		if _, e := os.Stat(path); e == nil {
			return errors.New("configuration already exists; edit it directly instead of overwriting credentials")
		}
		if *number == "" {
			fmt.Print("Your Google Voice number (for example +15551234567): ")
			line, e := bufio.NewReader(os.Stdin).ReadString('\n')
			if e != nil {
				return e
			}
			*number = strings.TrimSpace(line)
		}
		if !phonePattern.MatchString(*number) {
			return errors.New("number must include + and country code")
		}
		browser := findBrowser()
		if browser == "" {
			return errors.New("install Chromium or Google Chrome first")
		}
		c := Config{Listen: "0.0.0.0:8081", Username: "smsgate", Password: randomID(), Number: *number, Browser: browser, PollSeconds: 300, MaxThreads: 1000, HistoryMessages: 1000, CacheLimitMB: 1024, LogMaxMB: 10, CookieSource: defaultCookieSource(), AuthRetrySeconds: 300}
		if e := writeJSON(path, c); e != nil {
			return e
		}
		fmt.Println("Created", path)
		fmt.Println("Existing Chromium cookies will be imported automatically. Run auth-check to verify.")
		return nil
	}
	c, e := readConfig(home)
	if e != nil {
		return e
	}
	switch cmd {
	case "doctor":
		failed := false
		for _, check := range runDoctor(home, c) {
			fmt.Println(check.String())
			if check.Status == "FAIL" {
				failed = true
			}
		}
		if failed {
			return errors.New("doctor found one or more failures")
		}
		return nil
	case "credentials":
		fmt.Printf("Server: http://%s\nUsername: %s\nPassword: %s\n", c.Listen, c.Username, c.Password)
		return nil
	case "status":
		client := http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		address := c.Listen
		if host, port, err := net.SplitHostPort(address); err == nil {
			if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
				address = net.JoinHostPort("127.0.0.1", port)
			}
		}
		req, e := http.NewRequest("GET", "http://"+address+"/health", nil)
		if e != nil {
			return e
		}
		req.SetBasicAuth(c.Username, c.Password)
		resp, e := client.Do(req)
		if e != nil {
			return errors.New("service unreachable; start serve first")
		}
		defer resp.Body.Close()
		var v any
		if e := json.NewDecoder(resp.Body).Decode(&v); e != nil {
			return e
		}
		b, _ := json.MarshalIndent(v, "", "  ")
		fmt.Println(string(b))
		if resp.StatusCode != 200 {
			return errors.New("service is not ready")
		}
		return nil
	case "login", "serve", "auth-check":
		lock, e := lockHome(home)
		if e != nil {
			return e
		}
		defer lock.Close()
		if removed, e := cleanupScratch(home); e != nil {
			return e
		} else if removed > 0 {
			fmt.Fprintf(os.Stderr, "Removed %d stale browser profile(s) from a previous run.\n", removed)
		}
		if e := privateDir(filepath.Join(home, "profile")); e != nil {
			return e
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if cmd == "auth-check" {
			started := time.Now()
			e := checkAuthentication(ctx, home, c, func(message string) { fmt.Fprintln(os.Stderr, message) })
			if errors.Is(e, context.DeadlineExceeded) {
				return errors.New("authentication check timed out (90-second maximum); no message was sent")
			}
			if e != nil {
				return e
			}
			fmt.Printf("Headless authentication verified for configured Google Voice number (%.1fs).\n", time.Since(started).Seconds())
			return nil
		}
		if cmd == "serve" {
			return serve(ctx, home, c)
		}
		loginFlags := flag.NewFlagSet("login", flag.ContinueOnError)
		remote := loginFlags.Bool("remote", false, "sign in through a loopback DevTools endpoint over SSH")
		port := loginFlags.Int("port", 9222, "loopback DevTools port for --remote (0 picks a free port)")
		timeout := loginFlags.Duration("timeout", 15*time.Minute, "remote login timeout")
		if e := loginFlags.Parse(os.Args[2:]); e != nil {
			return e
		}
		if *remote {
			return remoteLogin(ctx, home, c, *port, *timeout)
		}
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return errors.New("login needs a graphical desktop (DISPLAY or WAYLAND_DISPLAY); this command can be run from a terminal on your desktop")
		}
		fmt.Println("Sign in in the dedicated Chromium window, complete MFA, and close that window when finished. No Google password is read by the CLI.")
		child := exec.CommandContext(ctx, c.Browser, "--user-data-dir="+filepath.Join(home, "profile"), "--no-first-run", "--no-default-browser-check", fmt.Sprintf("https://voice.google.com/u/%d/settings", c.Account))
		// Chromium emits browsing-related diagnostics; do not collect or print them.
		if e := child.Run(); e != nil {
			return errors.New("login browser exited unsuccessfully; close it normally to save the session")
		}
		loginConfig := c
		loginConfig.CookieSource = filepath.Join(home, "profile", "Default")
		b, e := startBrowser(ctx, home, loginConfig)
		if e != nil {
			return e
		}
		defer b.close()
		fmt.Println("Login saved; configured Google Voice sender verified headlessly.")
		return nil
	default:
		return fmt.Errorf("unknown command %q; use help", cmd)
	}
}
