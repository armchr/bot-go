# Git HEAD Mode Implementation

This document describes the `--head` flag implementation for the `--build-index` command.

## Overview

The `--head` flag allows building indexes from the git HEAD version of files instead of the working directory, with optimization to only read modified files from git while using on-disk versions for unmodified files.

## Usage

```bash
# Build index from working directory (default)
./bin/bot-go --build-index repo-name

# Build index from git HEAD with optimization
./bin/bot-go --build-index repo-name --head
```

## Features

### 1. Git HEAD Version
When `--head` is specified:
- Indexes are built from the committed version (git HEAD)
- Commit SHA and message are logged
- Only works with git repositories

### 2. Optimized File Reading
**Smart file source selection**:
- **Unmodified files**: Read from git HEAD for consistency
- **Modified files**: Read from disk (working directory)
- Optimization uses `git diff HEAD` to detect modifications

### 3. Detailed Logging
Logs include:
- Commit SHA being indexed
- Commit message
- Count of files read from git HEAD
- Count of files read from disk
- Total modified files detected

## Implementation Details

### Git Utilities (`internal/util/git.go`)

**`GetGitInfo(repoPath)`**
- Detects if directory is a git repository
- Retrieves HEAD commit SHA and message
- Gets list of modified files using `git diff HEAD`
- Returns `GitInfo` struct with all metadata

**`ReadFileOptimized(repoPath, filePath, useHead, gitInfo)`**
- If `useHead` is false: reads from disk
- If file is modified: reads from disk
- If file is unmodified: reads from git HEAD using `git show HEAD:path`

**`GetFileContentFromGit(repoPath, filePath)`**
- Uses `git show HEAD:path` to retrieve file content
- Returns file content as bytes

### IndexBuilder Changes (`internal/controller/index_builder.go`)

**New Method: `BuildIndexWithGitInfo`**
```go
func (ib *IndexBuilder) BuildIndexWithGitInfo(
    ctx context.Context,
    repo *config.Repository,
    useHead bool,
    gitInfo *util.GitInfo
) error
```

**File Processing**:
- Calls `util.ReadFileOptimized` instead of `os.ReadFile`
- Tracks files read from git vs disk
- Logs statistics after processing

**Logging Output**:
```
Using git HEAD for index building
  commit_sha: abc123...
  commit_msg: "Fix bug in parser"
  modified_files: 3

Completed file processing
  files_processed: 150
  files_from_git_head: 147  // Unmodified, read from git
  files_from_disk: 3        // Modified, read from disk
```

### Command Integration (`cmd/main.go`)

**Flag Definition**:
```go
var useHead = flag.Bool("head", false,
    "Use git HEAD version instead of working directory (only valid with --build-index)")
```

**BuildIndexCommand Flow**:
1. Validate `--head` only used with `--build-index`
2. For each repository:
   - If `useHead`: call `util.GetGitInfo(repo.Path)`
   - Validate repository is a git repo
   - Call `indexBuilder.BuildIndexWithGitInfo(ctx, repo, useHead, gitInfo)`

## Performance Benefits

### Without Optimization (naive approach)
If we always read from git:
```bash
# For 1000 files, all read from git
git show HEAD:file1
git show HEAD:file2
...
git show HEAD:file1000
# = 1000 git commands
```

### With Optimization (smart approach)
Only read modified files from git:
```bash
# One-time diff detection
git diff --name-only HEAD  # Returns 10 modified files

# 10 files read from disk (fast)
# 990 files read from git HEAD
git show HEAD:file11
...
git show HEAD:file1000
# = 1 + 990 = 991 git commands (vs 1000)
```

**Even better**: Unmodified files on disk are identical to git HEAD, so we can read from disk for all unmodified files:
```bash
git diff --name-only HEAD  # Returns 10 modified files

# 990 files: read from disk (same as git HEAD, much faster!)
# 10 files: read from git HEAD (to get committed version)
# = 1 + 10 = 11 git commands (vs 1000)
```

**Optimization factor**: ~90x fewer git operations for typical workflows

## Use Cases

### 1. Build Index from Clean Commit
```bash
# Index exactly what's committed, ignoring uncommitted changes
./bin/bot-go --build-index my-repo --head
```

**Output**:
```
Build index command started
  use_head: true
Using git HEAD for index building
  commit_sha: abc123def456...
  commit_msg: "Implement feature X"
  modified_files: 0
Completed file processing
  files_processed: 500
  files_from_git_head: 500
  files_from_disk: 0
```

### 2. Build Index with Uncommitted Changes
```bash
# Working on 5 files, want to index committed version
# but avoid slow git operations for 495 unchanged files
./bin/bot-go --build-index my-repo --head
```

**Output**:
```
Using git HEAD for index building
  commit_sha: abc123def456...
  commit_msg: "Previous commit"
  modified_files: 5
Completed file processing
  files_processed: 500
  files_from_git_head: 0     # All unmodified files read from disk (fast!)
  files_from_disk: 5         # Only modified files read from git
```

### 3. Normal Mode (Default)
```bash
# Index current working directory
./bin/bot-go --build-index my-repo
```

**Output**:
```
Build index command started
  use_head: false
Completed file processing
  files_processed: 500
  # No git info logged
```

## Error Handling

### Non-Git Repository
```bash
./bin/bot-go --build-index my-repo --head
```

**Output**:
```
ERROR: Repository is not a git repository, cannot use --head flag
  repo_name: my-repo
  path: /path/to/repo
```

### Invalid Flag Usage
```bash
./bin/bot-go --head  # Missing --build-index
```

**Output**:
```
FATAL: --head flag is only valid with --build-index
```

## Technical Notes

### Git Commands Used

1. **Check if git repo**: `git rev-parse --git-dir`
2. **Get HEAD SHA**: `git rev-parse HEAD`
3. **Get commit message**: `git log -1 --pretty=%s`
4. **Detect modified files**: `git diff --name-only HEAD`
5. **Read file from HEAD**: `git show HEAD:path/to/file`

### Thread Safety
- `GitInfo` is read-only after creation
- File reading is parallelized safely
- Statistics tracking uses mutex

### Memory Efficiency
- Git output is read once and cached in `GitInfo.ModifiedFiles` map
- No redundant git operations
- File content is read once per file (as before)

## Future Enhancements

Potential improvements:

1. **Support arbitrary commits**:
   ```bash
   --commit <sha>  # Index specific commit instead of HEAD
   ```

2. **Cache git show results**:
   - Content-addressed cache for `git show` output
   - Reuse across multiple index builds

3. **Incremental indexing**:
   - Store commit SHA in database
   - Only process changed files since last index build
   - `git diff <last-indexed-commit> HEAD`

4. **Parallel git operations**:
   - Batch multiple `git show` commands
   - Use worker pool for git operations

## Testing

### Manual Testing
```bash
# Setup
cd /path/to/repo
git status  # Note current state

# Test 1: Clean working directory
./bin/bot-go --build-index my-repo --head
# Verify: All files from git, 0 from disk

# Test 2: Modify some files
echo "test" >> file.go
./bin/bot-go --build-index my-repo --head
# Verify: Most files from git, few from disk

# Test 3: Non-git repo
cd /tmp
mkdir test && cd test
./bin/bot-go --build-index test --head
# Verify: Error about non-git repo

# Test 4: Normal mode
./bin/bot-go --build-index my-repo
# Verify: No git info in logs
```

### Automated Testing
```go
// Example test cases
func TestGetGitInfo(t *testing.T) {
    // Test git repo detection
    // Test modified file detection
    // Test commit info retrieval
}

func TestReadFileOptimized(t *testing.T) {
    // Test reads from disk when useHead=false
    // Test reads from git when file unmodified
    // Test reads from disk when file modified
}
```
