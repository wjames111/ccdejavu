# ccdejavu

Keeps your Claude desktop app chats with you when you switch accounts.

If you use more than one Claude account, say work and personal, or several seats on a team plan, the desktop app keeps a separate chat list for each one. Sign in with a different account and your Code tab and Cowork chats seem to vanish, even though they're still on your Mac. ccdejavu runs in the background and keeps those lists matched across every account signed in on this Mac, as long as the accounts are in the same org.

macOS only.

## Install

1. Install with Homebrew: `brew install wjames111/tap/ccdejavu`, or with Go: `go install github.com/wjames111/ccdejavu/cmd/ccdejavu@latest`
2. Optional: see what would change first with `ccdejavu sync --dry-run`
3. Run `ccdejavu install`. It backs up the app's chat folders, syncs once, and starts a background job (a LaunchAgent) that keeps syncing. The backup takes about 30 seconds
4. Restart the Claude desktop app (Cmd+Q, then reopen) so it shows the synced chats. The app reads its chat list when it opens
5. Check things over with `ccdejavu status` and `ccdejavu doctor`

## Commands

| Command | What it does |
|---|---|
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

ccdejavu copies only what it knows is a chat: `local_<id>.json`, a Cowork chat's `local_<id>/` folder, and `deleted_<id>` markers. Everything else is left alone, including account caches, scheduled tasks, the account settings inside each Cowork chat's `.claude/` folder and temporary uploads, and anything a future app version adds. The conversations themselves live in `~/.claude/projects/` and are already shared by every account, so ccdejavu doesn't touch them.

## How changes carry over

- **New and updated chats:** the newest copy wins. Only one account is signed in at a time, so there are no real conflicts.
- **Deleted chats:** the delete carries over. The chat is moved to `~/.ccdejavu/trash/`, never erased.
- **Timing:** about 2 seconds after a change, plus a full check every 5 minutes.

## Safety

- The first sync backs up both folders to `~/.ccdejavu/backups/` before changing anything. If the backup fails, nothing syncs.
- Every copy goes to a temp file first and is then renamed into place, so the app never reads half a chat.
- Before syncing a folder, ccdejavu checks every chat file in it. If anything looks unfamiliar, that folder is skipped and `ccdejavu status` says why.
- ccdejavu never empties its trash. Delete `~/.ccdejavu/trash/` yourself once you're sure.

## Limits

- Only the Mac desktop app's Code tab and Cowork chats. claude.ai chats live on Anthropic's servers.
- Only accounts on this Mac, and only within the same org.
- Scheduled tasks stay per account.
- The app's folder layout isn't a public API. An app update could change it; ccdejavu is built to stop, not guess, when that happens.

## Upgrading

```sh
brew upgrade ccdejavu
```

Or, if you installed with Go: `go install github.com/wjames111/ccdejavu/cmd/ccdejavu@latest`

Then run `ccdejavu install` again. Re-running it matters: the background job records the binary's full path, so it needs to point at the new one. `ccdejavu doctor` warns if the job is running an old version.

## Development

```sh
task test     # go test -race ./...
task lint     # golangci-lint
task build    # ./ccdejavu
task watch    # run the watcher in the foreground with debug logs
```

## License

MIT, see [LICENSE](LICENSE).
