# ccdejavu

Keeps your Claude desktop app chats with you when you switch accounts.

If you use more than one Claude account, say work and personal, or several seats on a team plan, the desktop app keeps a separate chat list for each one. Sign in with a different account and your Code tab and Cowork chats seem to vanish, even though they're still on your Mac. ccdejavu runs in the background and keeps chats matched across any accounts you link, in any org. Nothing syncs until you link accounts by email.

macOS only.

## Install

1. Install with Homebrew: `brew install wjames111/tap/ccdejavu`, or with Go: `go install github.com/wjames111/ccdejavu/cmd/ccdejavu@latest`
2. Link the accounts you want kept in sync: `ccdejavu link you@work.com you@home.com`
3. Optional: see what would change first with `ccdejavu sync --dry-run`
4. Run `ccdejavu install`. It backs up the app's chat folders, syncs once, and starts a background job (a LaunchAgent) that keeps syncing. The backup takes about 30 seconds
5. Restart the Claude desktop app (Cmd+Q, then reopen) so it shows the synced chats. The app reads its chat list when it opens
6. Check things over with `ccdejavu status` and `ccdejavu doctor`

## Linking accounts

Run `ccdejavu link <email> <email>` and answer its questions. The app doesn't store emails where ccdejavu can read them, so it asks you to help match each email to an account on this Mac:

- If ccdejavu already knows an account's email, from a Cowork chat or from the Claude Code CLI's own settings, it proposes that account and asks you to confirm.
- Otherwise it lists the candidate accounts with their chat counts and their most recent chats that exist only in that account, so you can tell them apart. If two accounts already share every chat, it says so.

For example:

```
$ ccdejavu link you@work.com you@home.com

Found you@work.com (3f1a9c2e, 181 chats).
Use it? [Y/n] y

Which account is you@home.com?
  1) 7b40d8e1  email not found on this Mac  83 chats
     only here: "Trip planning", "Budget spreadsheet", "Birthday ideas"
> 1

Linked accounts share one chat list. Continuing a chat under the other account sends that conversation to that account.
Link you@work.com and you@home.com? [y/N] y
Linked. Run `ccdejavu sync` to sync now, or `ccdejavu install` to keep them in sync in the background.
```

To add a third account, run `ccdejavu link` again with one email already in the group plus the new one, for example `ccdejavu link you@work.com you@new.com`. All the linked accounts end up sharing one chat list.

Run `ccdejavu unlink <email>` to stop syncing an account. Chats it already has stay where they are; it just stops getting new copies.

Continuing a chat under a different account sends that conversation to that account from then on. `link` shows this note before asking you to confirm.

## Commands

| Command | What it does |
|---|---|
| `ccdejavu link` | Link two accounts so their chats stay in sync |
| `ccdejavu unlink` | Stop syncing an account (chats already copied stay put) |
| `ccdejavu install` | Back up, sync once, and start the background job |
| `ccdejavu sync` | Sync once, now. `--dry-run` shows what would change and changes nothing |
| `ccdejavu status` | Accounts, chat counts, last sync, anything skipped, trash size |
| `ccdejavu doctor` | Check the app's folders and the background job |
| `ccdejavu uninstall` | Stop and remove the background job. Your chats stay where they are |
| `ccdejavu watch` | What the background job runs. You don't need to run it yourself |

## What it syncs

In `~/Library/Application Support/Claude/`, the app keeps one folder per account and org:

- `claude-code-sessions/<account>/<org>/` for Code tab chats
- `local-agent-mode-sessions/<account>/<org>/` for Cowork chats

Linked accounts share one combined chat list across all their orgs: every org folder of every account in the group gets the same chats. ccdejavu copies only what it knows is a chat: `local_<id>.json`, a Cowork chat's `local_<id>/` folder, and `deleted_<id>` markers. Everything else is left alone, including account caches, scheduled tasks, the account settings inside each Cowork chat's `.claude/` folder and temporary uploads, and anything a future app version adds. The conversations themselves live in `~/.claude/projects/` and are already shared by every account, so ccdejavu doesn't touch them.

## How changes carry over

- **New and updated chats:** the newest copy wins. Only one account is signed in at a time, so there are no real conflicts.
- **Deleted chats:** the delete carries over. The chat is moved to `~/.ccdejavu/trash/`, never erased.
- **Timing:** about 2 seconds after a change, plus a full check every 5 minutes.

## Safety

- The first sync backs up both folders to `~/.ccdejavu/backups/` before changing anything. A backup is also taken before the first sync of newly linked accounts. If the backup fails, nothing syncs.
- Every copy goes to a temp file first and is then renamed into place, so the app never reads half a chat.
- Before syncing a folder, ccdejavu checks every chat file in it. If anything looks unfamiliar, that folder is skipped and `ccdejavu status` says why.
- ccdejavu never empties its trash. Delete `~/.ccdejavu/trash/` yourself once you're sure.

## Limits

- Only the Mac desktop app's Code tab and Cowork chats. claude.ai chats live on Anthropic's servers.
- Only accounts on this Mac, and only the ones you've linked.
- Scheduled tasks stay per account.
- The app's folder layout isn't a public API. An app update could change it; ccdejavu is built to stop, not guess, when that happens.

## Upgrading

```sh
brew upgrade ccdejavu
```

Or, if you installed with Go: `go install github.com/wjames111/ccdejavu/cmd/ccdejavu@latest`

Then run `ccdejavu install` again. Re-running it matters: the background job records the binary's full path, so it needs to point at the new one. `ccdejavu doctor` warns if the job is running an old version.

Since 0.2.0, nothing syncs until you link accounts. If you're upgrading from an older version, run `ccdejavu link <email> <email>` first, then `ccdejavu install`.

## Development

```sh
task test     # go test -race ./...
task lint     # golangci-lint
task build    # ./ccdejavu
task watch    # run the watcher in the foreground with debug logs
```

## License

MIT, see [LICENSE](LICENSE).
