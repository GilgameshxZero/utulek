package main

import (
	"errors"
	"fmt"
	"time"
)

type Account struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Server   string `json:"server"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (c Config) account(id string) (Config, error) {
	for _, account := range c.Accounts {
		if account.ID == id {
			c.ActiveAccount = id
			c.Server, c.Username, c.Password = account.Server, account.Username, account.Password
			return c, nil
		}
	}
	return c, errors.New("unknown account")
}
func (a *App) accountsKey(key int) {
	switch key {
	case 'j', keyDown:
		a.Selected = min(len(a.Config.Accounts)-1, a.Selected+1)
	case 'k', keyUp:
		a.Selected = max(0, a.Selected-1)
	case 'q', 'b', 27, 'y':
		a.restore(a.AccountBase)
	case 10, 13, keyEnter:
		if len(a.Config.Accounts) == 0 {
			return
		}
		id := a.Config.Accounts[a.Selected].ID
		if id == a.Config.ActiveAccount {
			a.restore(a.AccountBase)
			return
		}
		if a.Jobs["send"] {
			a.note("waiting for send result")
			return
		}
		c, err := a.Config.account(id)
		if err != nil {
			a.note(err.Error())
			return
		}
		next, err := newApp(c, a.ConfigPath, a.Demo)
		if err != nil {
			a.note(err.Error())
			return
		}
		if err = c.save(a.ConfigPath); err != nil {
			next.close()
			a.note("cannot save account selection")
			return
		}
		a.close() // Wait for all old workers before replacing account state.
		*a = *next
		a.Prompt = a.prompt
		a.contacts("")
		a.sync()
		a.Dirty = true
		a.note(fmt.Sprintf("Account: %s", c.Accounts[a.accountIndex()].Name))
	}
}
func (a *App) accountIndex() int {
	for i, v := range a.Config.Accounts {
		if v.ID == a.Config.ActiveAccount {
			return i
		}
	}
	return 0
}

func (a *App) pushRetryInterval() time.Duration {
	if a.Config.PushRetry > 0 {
		return time.Duration(a.Config.PushRetry) * time.Second
	}
	return 300 * time.Second
}
