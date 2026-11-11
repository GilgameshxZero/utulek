package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/chromedp/chromedp"
)

func freeLoopbackPort() (int, error) {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return 0, e
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func remoteLoginArgs(profile string, port int, address string) []string {
	return []string{
		"--headless=new",
		"--remote-debugging-address=127.0.0.1",
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--user-data-dir=" + profile,
		"--no-first-run",
		"--no-default-browser-check",
		address,
	}
}

func waitForDevTools(ctx context.Context, endpoint string, exited <-chan error) error {
	client := http.Client{Timeout: 2 * time.Second}
	defer client.CloseIdleConnections()
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		response, e := client.Get(endpoint + "/json/version")
		if e == nil {
			response.Body.Close()
			if response.StatusCode == 200 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e := <-exited:
			if e == nil {
				e = errors.New("browser exited")
			}
			return fmt.Errorf("remote Chromium exited before DevTools was ready: %w", e)
		case <-ticker.C:
		}
	}
}

func stopBrowserProcess(cmd *exec.Cmd, exited <-chan error) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-exited
	}
}

// remoteLogin runs headless Chromium with a loopback-only DevTools endpoint. The
// user reaches it through an SSH tunnel and signs in with Chrome's own DevTools
// screencast, so no X server is needed on the server.
func remoteLogin(ctx context.Context, home string, c Config, port int, timeout time.Duration) error {
	if c.Browser == "" {
		return errors.New("install Chromium or Google Chrome first")
	}
	profile := filepath.Join(home, "profile")
	if e := privateDir(profile); e != nil {
		return e
	}
	if port == 0 {
		var e error
		if port, e = freeLoopbackPort(); e != nil {
			return e
		}
	}
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	address := fmt.Sprintf("https://voice.google.com/u/%d/settings", c.Account)
	cmd := exec.Command(c.Browser, remoteLoginArgs(profile, port, address)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if e := cmd.Start(); e != nil {
		return e
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	defer stopBrowserProcess(cmd, exited)

	if e := waitForDevTools(ctx, endpoint, exited); e != nil {
		return e
	}
	host, _ := os.Hostname()
	user := os.Getenv("USER")
	if user == "" {
		user = "user"
	}
	fmt.Printf("Remote login ready (headless Chromium on 127.0.0.1:%d).\n", port)
	fmt.Printf("1) On your local machine: ssh -L %d:127.0.0.1:%d %s@%s\n", port, port, user, host)
	fmt.Printf("2) Open http://127.0.0.1:%d in Chrome, choose the Voice page, and use DevTools screencast.\n", port)
	fmt.Println("Sign in, complete MFA, and leave the page on Voice settings. This command exits when the configured number is visible.")

	loginCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(loginCtx, endpoint)
	defer allocCancel()
	tabCtx, tabCancel := chromedp.NewContext(allocCtx)
	defer tabCancel()

	for {
		var number string
		_ = chromedp.Run(tabCtx, chromedp.Evaluate(accountNumberJS, &number))
		if number == c.Number {
			break
		}
		select {
		case <-loginCtx.Done():
			return errors.New("remote login timed out")
		case e := <-exited:
			if e == nil {
				e = errors.New("browser exited")
			}
			return fmt.Errorf("remote login browser exited: %w", e)
		case <-time.After(2 * time.Second):
		}
	}
	tabCancel()
	allocCancel()
	stopBrowserProcess(cmd, exited)

	loginConfig := c
	loginConfig.CookieSource = filepath.Join(profile, "Default")
	b, e := startBrowser(ctx, home, loginConfig)
	if e != nil {
		return e
	}
	defer b.close()
	fmt.Println("Login saved; configured Google Voice sender verified headlessly.")
	return nil
}
