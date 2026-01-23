---
name: sync-troubleshooting
description: |
  Troubleshooting guide for gz-git sync and workspace operations.
  Use when:
  - Sync fails with network/auth errors
  - Repositories show diverged/conflict status
  - Working tree has uncommitted changes blocking sync
  - Health check reports errors or warnings
  - Understanding sync status output and recommendations
allowed-tools: Bash, Read, Grep
---

# Sync Troubleshooting Guide

This skill provides solutions for common gz-git sync and workspace issues.

## Quick Diagnosis

```bash
# Check repository health
gz-git sync status --target ~/repos

# Verbose output with details
gz-git sync status --target ~/repos --verbose

# Skip fetch for quick local check
gz-git sync status --skip-fetch
```

---

## Health Status Reference

| Icon | Status      | Meaning                          | Action Required |
| ---- | ----------- | -------------------------------- | --------------- |
| ✓    | healthy     | Up-to-date, clean working tree   | None            |
| ⚠    | warning     | Diverged, behind, or ahead       | Pull or push    |
| ✗    | error       | Conflicts or dirty + behind      | Manual resolve  |
| ⊘    | unreachable | Network timeout or auth failed   | Fix connection  |

---

## Network Issues

### Timeout (`⊘ timeout`)

**Symptom**:
```
⊘ project-api (main)   timeout   fetch failed (30s timeout)
```

**Causes**:
- Slow network connection
- Large repository
- Server overload

**Solutions**:

```bash
# Increase timeout
gz-git sync status --timeout 120s

# Skip fetch for quick check
gz-git sync status --skip-fetch

# Test connectivity manually
git ls-remote origin

# Check if server is responding
curl -I https://gitlab.company.com
```

### Unreachable (`⊘ unreachable`)

**Symptom**:
```
⊘ project-api (main)   unreachable   remote unreachable
```

**Causes**:
- DNS resolution failed
- Server down
- Firewall blocking
- VPN not connected

**Solutions**:

```bash
# Check DNS
nslookup gitlab.company.com

# Check network connectivity
ping gitlab.company.com

# Check if VPN is needed
# (connect to VPN if required)

# Test SSH connection
ssh -T git@gitlab.company.com

# Test HTTPS
curl -I https://gitlab.company.com
```

### Authentication Failed (`⊘ auth-failed`)

**Symptom**:
```
⊘ project-api (main)   auth-failed   authentication failed
```

**Causes**:
- Invalid or expired token
- SSH key not loaded
- Wrong credentials
- Token scope insufficient

**Solutions**:

```bash
# Check token is set
echo $GITLAB_TOKEN

# Test token manually
curl -H "PRIVATE-TOKEN: $GITLAB_TOKEN" \
  "https://gitlab.company.com/api/v4/user"

# Regenerate token if expired
# (go to GitLab/GitHub settings)

# Check SSH key
ssh-add -l
ssh -T git@gitlab.company.com

# Load SSH key
eval $(ssh-agent)
ssh-add ~/.ssh/id_rsa
```

**Token Scope Requirements**:

| Provider | Required Scopes            |
| -------- | -------------------------- |
| GitHub   | `repo`, `read:org`         |
| GitLab   | `api`, `read_repository`   |
| Gitea    | `read:repository`          |

---

## Divergence Issues

### Behind Remote (`⚠ N↓ behind`)

**Symptom**:
```
⚠ project-api (main)   warning   5↓ behind
  → Pull 5 commits from upstream
```

**Cause**: Remote has new commits you don't have.

**Solutions**:

```bash
# Pull changes
cd project-api
git pull

# Or with rebase
git pull --rebase

# Bulk pull all repos
gz-git pull .
```

### Ahead of Remote (`⚠ N↑ ahead`)

**Symptom**:
```
⚠ project-api (main)   warning   3↑ ahead
  → Push 3 commits to upstream
```

**Cause**: You have local commits not pushed.

**Solutions**:

```bash
# Push changes
cd project-api
git push

# Bulk push all repos
gz-git push .
```

### Diverged (`⚠ N↑ N↓ diverged`)

**Symptom**:
```
⚠ project-api (develop)   warning   2↑ 3↓ diverged
  → Diverged: 2 ahead, 3 behind. Use 'git pull --rebase'
```

**Cause**: Both local and remote have new commits.

**Solutions**:

```bash
cd project-api

# Option 1: Rebase (linear history)
git pull --rebase
# Resolve any conflicts
git push

# Option 2: Merge (preserve history)
git pull
# Resolve any conflicts
git push

# Option 3: Force push (discard remote - DANGEROUS!)
git push --force-with-lease
```

### No Upstream (`⚠ no-upstream`)

**Symptom**:
```
⚠ project-api (feature/new)   warning   no-upstream
  → Set upstream with: git push -u origin feature/new
```

**Cause**: Local branch has no tracking remote branch.

**Solutions**:

```bash
# Set upstream
git push -u origin feature/new

# Or track existing remote branch
git branch --set-upstream-to=origin/feature/new
```

### Conflict (`✗ conflict`)

**Symptom**:
```
✗ project-api (main)   error   conflict
  → Resolve merge conflicts in 3 files
```

**Cause**: Merge or rebase resulted in conflicts.

**Solutions**:

```bash
cd project-api

# Check conflict status
git status

# For merge conflicts:
# 1. Edit conflicted files
# 2. Mark as resolved
git add <resolved-files>
git commit

# For rebase conflicts:
# 1. Edit conflicted files
# 2. Continue rebase
git add <resolved-files>
git rebase --continue

# Or abort and try different approach
git merge --abort
# or
git rebase --abort
```

---

## Working Tree Issues

### Dirty Working Tree (`✗ dirty`)

**Symptom**:
```
✗ project-api (main)   error   dirty + 5↓ behind
  → Commit or stash 3 modified files, then pull
```

**Cause**: Uncommitted changes blocking sync.

**Solutions**:

```bash
cd project-api

# Check what's changed
git status

# Option 1: Commit changes
git add .
git commit -m "WIP: save changes"
git pull

# Option 2: Stash changes
git stash
git pull
git stash pop

# Option 3: Discard changes (CAREFUL!)
git checkout .
git clean -fd
```

### Rebase in Progress

**Symptom**:
```
✗ project-api (main)   error   rebase-in-progress
  → Complete or abort rebase: git rebase --continue or --abort
```

**Solutions**:

```bash
cd project-api

# Check status
git status

# Continue rebase (after resolving conflicts)
git add .
git rebase --continue

# Or abort rebase
git rebase --abort
```

### Merge in Progress

**Symptom**:
```
✗ project-api (main)   error   merge-in-progress
  → Complete or abort merge: git commit or git merge --abort
```

**Solutions**:

```bash
cd project-api

# Check status
git status

# Complete merge (after resolving conflicts)
git add .
git commit

# Or abort merge
git merge --abort
```

---

## Sync Strategy Issues

### Reset Strategy Discards Changes

**Symptom**: Local changes lost after sync with `--strategy reset`.

**Prevention**:

```bash
# Check status before sync
gz-git sync status

# Use pull strategy instead
gz-git sync from-forge --strategy pull ...

# Or stash before reset
gz-git stash save .
gz-git sync from-forge --strategy reset ...
gz-git stash pop .
```

### Pull Strategy Conflicts

**Symptom**: Merge conflicts with `--strategy pull`.

**Solutions**:

```bash
# Resolve conflicts manually
cd project-api
git status
# Edit files, then:
git add .
git commit

# Or use reset strategy (discard local)
gz-git sync from-forge --strategy reset ...
```

---

## Repository Issues

### Invalid Repository

**Symptom**:
```
✗ project-api   error   failed to open repository
```

**Causes**:
- Directory not a git repo
- Corrupted `.git` directory
- Permission issues

**Solutions**:

```bash
# Check if it's a git repo
cd project-api
git status

# If corrupted, re-clone
cd ..
rm -rf project-api
git clone <url> project-api

# Check permissions
ls -la project-api/.git
```

### Repository Not Found

**Symptom**:
```
Error: repository not found: project-api
```

**Solutions**:

```bash
# Check config file paths
cat .gz-git.yaml

# Verify directory exists
ls -la project-api

# Re-sync from forge
gz-git sync from-forge --org myorg --target .
```

---

## Bulk Operation Issues

### Partial Failures

**Symptom**: Some repos succeed, others fail.

```
✓ project-core    healthy
✗ project-api     error    dirty
⊘ project-web     timeout
```

**Solutions**:

```bash
# Fix individual repos
cd project-api
git stash

# Retry sync
gz-git workspace sync

# Or sync specific repos only
gz-git sync from-forge --include "project-api|project-web" ...
```

### Too Many Parallel Operations

**Symptom**: Timeouts or rate limiting.

**Solutions**:

```bash
# Reduce parallelism
gz-git sync from-forge --parallel 2 ...
gz-git workspace sync -j 2

# Increase timeout
gz-git sync status --timeout 120s
```

---

## Quick Fix Commands

### Reset Everything (Nuclear Option)

```bash
# WARNING: Discards all local changes!

# Single repo
cd project-api
git fetch origin
git reset --hard origin/main
git clean -fd

# All repos (via gz-git)
gz-git sync from-forge --strategy reset --cleanup-orphans ...
```

### Sync Fresh Start

```bash
# Remove and re-clone everything
rm -rf ~/repos/*
gz-git sync from-forge --org myorg --target ~/repos
```

### Fix Common Issues in Bulk

```bash
# Fetch all remotes
gz-git fetch --prune .

# Stash all dirty repos
gz-git stash save . -m "pre-sync stash"

# Pull all repos
gz-git pull .

# Pop stashes
gz-git stash pop .
```

---

## Diagnostic Commands Summary

| Task                    | Command                                    |
| ----------------------- | ------------------------------------------ |
| Check health            | `gz-git sync status --target DIR`          |
| Verbose health          | `gz-git sync status -v`                    |
| Skip fetch (fast)       | `gz-git sync status --skip-fetch`          |
| Increase timeout        | `gz-git sync status --timeout 120s`        |
| JSON output             | `gz-git sync status -f json`               |
| Check single repo       | `gz-git info /path/to/repo`                |
| Test SSH                | `ssh -T git@github.com`                    |
| Test token              | `curl -H "PRIVATE-TOKEN: $TOKEN" URL`      |

---

## Error Message Quick Reference

| Error Message                    | Likely Cause           | Quick Fix                    |
| -------------------------------- | ---------------------- | ---------------------------- |
| `fetch timeout`                  | Slow network           | `--timeout 120s`             |
| `authentication failed`          | Bad token/key          | Check `$TOKEN`, `ssh-add`    |
| `remote unreachable`             | Network/DNS issue      | Check VPN, DNS               |
| `dirty + behind`                 | Local changes          | `git stash`, then pull       |
| `diverged`                       | Both changed           | `git pull --rebase`          |
| `conflict`                       | Merge conflict         | Resolve manually             |
| `rebase-in-progress`             | Incomplete rebase      | `--continue` or `--abort`    |
| `no-upstream`                    | No tracking branch     | `git push -u origin BRANCH`  |
| `failed to open repository`      | Not a git repo         | Re-clone                     |
