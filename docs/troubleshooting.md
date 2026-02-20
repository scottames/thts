# Troubleshooting

## Common Issues

### "Run `thts setup` first"

You need to configure thts before using it in any project.

```bash
thts setup
```

This creates `~/.config/thts/config.json` with your thoughts repo location and
username.

### "Not in a git repository"

thts requires your code to be in a git repository.

```bash
git init
thts init
```

### Symlinks Not Working

**Symptoms:** `thoughts/` directory exists but contains broken symlinks or no
files.

**Check symlink targets:**

```bash
ls -la thoughts/
```

**Common causes:**

1. Thoughts repo moved or deleted - update config with `thts config --edit`
2. Project not initialized - run `thts init`
3. Central directories don't exist - `thts sync` creates them

### Sync Conflicts

**Symptoms:** `thts sync` fails with rebase errors.

**Resolution:**

```bash
cd ~/thoughts              # Go to thoughts repo
git status                 # See conflicting files
# Edit files to resolve conflicts
git add .
git rebase --continue
cd -                       # Return to project
thts sync                   # Retry
```

### Auto-Sync Not Working

**Symptoms:** Thoughts don't sync after commits.

**Check hooks are installed:**

```bash
ls -la .git/hooks/post-commit
cat .git/hooks/post-commit
```

**Reinstall hooks:**

```bash
thts init --force
```

**Check config:**

```bash
thts config
# Verify autoSyncInWorktrees is true (if in a worktree)
```

### Searchable Directory Issues

**Symptoms:** `searchable/` is empty or missing files.

**Rebuild it:**

```bash
thts sync
```

**Cross-filesystem warning:** If your thoughts repo and project are on different
filesystems, hard links won't work. thts will warn about this. Consider moving
your thoughts repo to the same filesystem.

### Permission Denied

**Symptoms:** Can't write to thoughts directory or repo.

**Check permissions:**

```bash
ls -la ~/thoughts
ls -la thoughts/
```

**Common causes:**

1. Thoughts repo owned by different user
2. Directory permissions too restrictive

### Thoughts Accidentally Staged

**Symptoms:** Git shows `thoughts/` files as staged.

The pre-commit hook should prevent this, but if it happens:

```bash
git reset HEAD thoughts/
```

**Verify gitignore:**

```bash
grep thoughts .gitignore
# Or check .git/info/exclude depending on your gitIgnore setting
```

### Wrong Project Name

**Symptoms:** Project mapped to wrong directory in thoughts repo.

**Reinitialize with explicit name:**

```bash
thts uninit --all --force
thts init --name correct-name
```

Use `--all` so the shared repo mapping is removed before reinitializing.

### Uninit Didn't Fully Detach Repository

**Symptoms:** `thts uninit` removed local `thoughts/` but repo still appears mapped.

`thts uninit` removes local setup only. To fully detach the repository mapping:

```bash
thts uninit --all
```

### Profile Not Found

**Symptoms:** `thts init --profile foo` fails.

**List available profiles:**

```bash
thts profile list
```

**Create the profile:**

```bash
thts profile create foo --repo ~/foo-thoughts
```

## Diagnostic Commands

```bash
# Check current configuration
thts config --json

# Check status and mappings
thts status

# Verify symlinks
ls -la thoughts/

# Check git hooks
ls -la .git/hooks/

# Check thoughts repo status
git -C ~/thoughts status

# Verify hard links (same inode = working)
stat thoughts/{user}/file.md thoughts/searchable/{user}/file.md
```

## Getting Help

If you're still stuck:

1. Check `thts status` output for clues
2. Verify config with `thts config --json`
3. Try `thts init --force` to reinitialize
4. Check the [User Guide](guide.md) for correct usage
