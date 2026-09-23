package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wjames111/ccdejavu/internal/accounts"
	"github.com/wjames111/ccdejavu/internal/links"
)

func newLinkCommand(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "link <email> <email>",
		Short: "Link two accounts so their chats stay in sync",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := g.paths()
			if err != nil {
				return err
			}
			for _, e := range args {
				if !strings.Contains(e, "@") {
					return fmt.Errorf("%q doesn't look like an email", e)
				}
			}
			sums, err := accounts.Summarize(p)
			if err != nil {
				return err
			}
			if len(sums) < 2 {
				return fmt.Errorf("found %s on this Mac; sign each account into the Claude desktop app once, then try again", plural(len(sums), "account"))
			}
			l, err := links.Load(p.Links())
			if err != nil {
				return err
			}
			w, in := cmd.OutOrStdout(), bufio.NewScanner(cmd.InOrStdin())
			var picked []links.Account
			for _, email := range args {
				email = strings.ToLower(strings.TrimSpace(email))
				s, err := pick(w, in, email, sums, picked)
				if err != nil {
					return err
				}
				picked = append(picked, links.Account{Email: email, ID: s.ID})
			}
			fmt.Fprintln(w, "\nLinked accounts share one chat list. Continuing a chat under the other account sends that conversation to that account.")
			if !ask(w, in, fmt.Sprintf("Link %s and %s? [y/N] ", picked[0].Email, picked[1].Email), false) {
				fmt.Fprintln(w, "Nothing changed.")
				return nil
			}
			if err := l.Link(picked[0], picked[1]); err != nil {
				return err
			}
			if err := links.Save(p.Links(), l); err != nil {
				return err
			}
			fmt.Fprintln(w, "Linked. Run `ccdejavu sync` to sync now, or `ccdejavu install` to keep them in sync in the background.")
			return nil
		},
	}
}

func newUnlinkCommand(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "unlink <email>",
		Short: "Stop syncing an account (chats already copied stay put)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := g.paths()
			if err != nil {
				return err
			}
			l, err := links.Load(p.Links())
			if err != nil {
				return err
			}
			if !l.Unlink(args[0]) {
				return fmt.Errorf("%s isn't linked", args[0])
			}
			if err := links.Save(p.Links(), l); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Unlinked %s. Chats already copied stay where they are.\n", args[0])
			return nil
		},
	}
}

// pick asks which account belongs to email, offering only accounts that could be it.
func pick(w io.Writer, in *bufio.Scanner, email string, sums []accounts.Summary, picked []links.Account) (accounts.Summary, error) {
	var candidates []accounts.Summary
	for _, s := range sums {
		taken := slices.ContainsFunc(picked, func(a links.Account) bool { return a.ID == s.ID })
		if taken || (s.Email != "" && s.Email != email) {
			continue
		}
		candidates = append(candidates, s)
	}
	if len(candidates) == 0 {
		return accounts.Summary{}, fmt.Errorf("no account on this Mac could be %s", email)
	}
	if i := slices.IndexFunc(candidates, func(s accounts.Summary) bool { return s.Email == email }); i >= 0 {
		s := candidates[i]
		fmt.Fprintf(w, "\nFound %s (%s, %s).\n", email, short(s.ID), plural(s.Chats, "chat"))
		if ask(w, in, "Use it? [Y/n] ", true) {
			return s, nil
		}
	}
	fmt.Fprintf(w, "\nWhich account is %s?\n", email)
	for i, s := range candidates {
		label := s.Email
		if label == "" {
			label = "email not found on this Mac"
		}
		mark := ""
		if s.ID == sums[0].ID && !s.LastActive.IsZero() {
			mark = "  · most recently active"
		}
		fmt.Fprintf(w, "  %d) %s  %s  %s%s\n", i+1, short(s.ID), label, plural(s.Chats, "chat"), mark)
		if len(s.Recent) > 0 {
			quoted := make([]string, len(s.Recent))
			for j, t := range s.Recent {
				quoted[j] = strconv.Quote(t)
			}
			fmt.Fprintf(w, "     %s\n", strings.Join(quoted, ", "))
		}
	}
	for {
		fmt.Fprint(w, "> ")
		if !in.Scan() {
			return accounts.Summary{}, errors.New("no answer given")
		}
		n, err := strconv.Atoi(strings.TrimSpace(in.Text()))
		if err == nil && n >= 1 && n <= len(candidates) {
			return candidates[n-1], nil
		}
		fmt.Fprintf(w, "Enter a number from 1 to %d.\n", len(candidates))
	}
}

// ask reads a yes/no answer; an empty answer means def.
func ask(w io.Writer, in *bufio.Scanner, prompt string, def bool) bool {
	fmt.Fprint(w, prompt)
	if !in.Scan() {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(in.Text())) {
	case "":
		return def
	case "y", "yes":
		return true
	}
	return false
}
