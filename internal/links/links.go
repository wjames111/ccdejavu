// Package links records which accounts the user chose to keep in sync.
package links

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Account is one linked account: the email the user typed and the app's account ID.
type Account struct {
	Email string `json:"email"`
	ID    string `json:"id"`
}

// Group is a set of accounts that share one chat list.
type Group struct {
	Accounts []Account `json:"accounts"`
}

// Links is everything the user has linked.
type Links struct {
	Groups []Group `json:"groups"`
}

// Link joins a and b, adding one to the other's group or merging two groups.
func (l *Links) Link(a, b Account) error {
	a.Email, b.Email = normal(a.Email), normal(b.Email)
	if a.ID == b.ID {
		return errors.New("pick two different accounts")
	}
	if a.Email == b.Email {
		return errors.New("the two emails are the same")
	}
	for _, x := range []Account{a, b} {
		if err := l.check(x); err != nil {
			return err
		}
	}
	ga, gb := l.find(a.ID), l.find(b.ID)
	switch {
	case ga < 0 && gb < 0:
		l.Groups = append(l.Groups, Group{Accounts: []Account{a, b}})
	case gb < 0:
		l.Groups[ga].Accounts = append(l.Groups[ga].Accounts, b)
	case ga < 0:
		l.Groups[gb].Accounts = append(l.Groups[gb].Accounts, a)
	case ga != gb:
		l.Groups[ga].Accounts = append(l.Groups[ga].Accounts, l.Groups[gb].Accounts...)
		l.Groups = slices.Delete(l.Groups, gb, gb+1)
	}
	return nil
}

// check refuses an email already tied to another account, or an account already tied to another email.
func (l *Links) check(x Account) error {
	for _, g := range l.Groups {
		for _, have := range g.Accounts {
			if have.Email == x.Email && have.ID != x.ID {
				return fmt.Errorf("%s is already linked to a different account (%s)", x.Email, short(have.ID))
			}
			if have.ID == x.ID && have.Email != x.Email {
				return fmt.Errorf("that account is already linked as %s", have.Email)
			}
		}
	}
	return nil
}

func (l *Links) find(id string) int {
	return slices.IndexFunc(l.Groups, func(g Group) bool {
		return slices.ContainsFunc(g.Accounts, func(a Account) bool { return a.ID == id })
	})
}

// Unlink removes the account with this email; a group left with one account is dropped.
func (l *Links) Unlink(email string) bool {
	email = normal(email)
	for i, g := range l.Groups {
		j := slices.IndexFunc(g.Accounts, func(a Account) bool { return a.Email == email })
		if j < 0 {
			continue
		}
		l.Groups[i].Accounts = slices.Delete(g.Accounts, j, j+1)
		if len(l.Groups[i].Accounts) < 2 {
			l.Groups = slices.Delete(l.Groups, i, i+1)
		}
		return true
	}
	return false
}

// EmailFor returns the linked email for an account ID, or "" if it isn't linked.
func (l *Links) EmailFor(id string) string {
	for _, g := range l.Groups {
		for _, a := range g.Accounts {
			if a.ID == id {
				return a.Email
			}
		}
	}
	return ""
}

// Load reads the links file. A missing file means nothing is linked.
func Load(path string) (Links, error) {
	var l Links
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return l, err
	}
	if err := json.Unmarshal(raw, &l); err != nil {
		return l, fmt.Errorf("reading %s: %w", path, err)
	}
	return l, nil
}

// Save writes the links file atomically.
func Save(path string, l Links) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	body, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".links-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(body, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func normal(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
