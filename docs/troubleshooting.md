# Troubleshooting

## Common Issues

### "Run `tpd setup` first"

You need to configure tpd before using it in any project.

```bash
tpd setup
```

This creates `~/.config/tpd/config.json` with your thoughts repo location and
username.

### "Not in a git repository"

tpd requires your code to be in a git repository.

```bash
git init
tpd init
```

### Symlinks Not Working

**Symptoms:** `thoughts/` directory exists but contains broken symlinks or no
files.

**Check symlink targets:**

```bash
ls -la thoughts/
```

**Common causes:**

1. Thoughts repo moved or deleted - update config with `tpd config --edit`
2. Project not initialized - run `tpd init`
3. Central directories don't exist - `tpd sync` creates them

### Sync Conflicts

**Symptoms:** `tpd sync` fails with rebase errors.

**Resolution:**

```bash
cd ~/thoughts              # Go to thoughts repo
git status                 # See conflicting files
# Edit files to resolve conflicts
git add .
git rebase --continue
cd -                       # Return to project
tpd sync                   # Retry
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
tpd init --force
```

**Check config:**

```bash
tpd config
# Verify autoSyncInWorktrees is true (if in a worktree)
```

### Searchable Directory Issues

**Symptoms:** `searchable/` is empty or missing files.

**Rebuild it:**

```bash
tpd sync
```

**Cross-filesystem warning:** If your thoughts repo and project are on different
filesystems, hard links won't work. tpd will warn about this. Consider moving
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

**Fix the mapping:**

```bash
tpd config --edit
# Edit repoMappings to fix the path
```

**Or reinitialize:**

```bash
tpd uninit --force
tpd init --name correct-name
```

### Profile Not Found

**Symptoms:** `tpd init --profile foo` fails.

**List available profiles:**

```bash
tpd profile list
```

**Create the profile:**

```bash
tpd profile create foo --repo ~/foo-thoughts
```

## Diagnostic Commands

```bash
# Check current configuration
tpd config --json

# Check status and mappings
tpd status

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

1. Check `tpd status` output for clues
2. Verify config with `tpd config --json`
3. Try `tpd init --force` to reinitialize
4. Check the [User Guide](guide.md) for correct usage
