# Versioning Approaches for Code Analysis

This document outlines different approaches to implement versioned code graph, vectors, and n-grams with a base version (v1) and multiple ephemeral versions (v2+).

## Requirements

- **Base Version (v1)**: Built from `--build-index` command, reflects repository state at that point
- **Ephemeral Versions (v2+)**: Reflect state at specific commits or uncommitted/unstaged files
- **Multiple Simultaneous Versions**: Support multiple ephemeral versions at once
- **Optimization Goals**:
  - Fast ephemeral version creation
  - Minimal database size

---

## Approach 1: Git-Based Snapshot with Delta Processing

### Concept
Store base version (v1) fully, ephemeral versions (v2+) as deltas from base or parent commit.

### Structure

**Base Version (v1)**: Full index built from `--build-index` command
- Store commit SHA as metadata
- Complete graph/vectors/ngrams in DB

**Ephemeral Versions (v2+)**: Delta-based
- Identify changed files using `git diff --name-status <base-commit> <target-commit>`
- Only process changed/added files
- Mark deleted files' nodes as inactive rather than deleting

### Database Schema

```sql
-- Nodes/Chunks
Nodes: {
  id,
  version_id,
  active,
  data...
}

-- Version tracking
Versions: {
  version_id,
  commit_sha,
  parent_version_id,
  is_ephemeral,
  created_at
}

-- File tracking per version
FileVersions: {
  file_path,
  version_id,
  status  -- 'unchanged', 'modified', 'added', 'deleted'
}
```

### Query Pattern

```sql
-- Get all active nodes for version X
SELECT * FROM Nodes
WHERE version_id IN (
  SELECT version_id FROM Versions
  WHERE version_id = X OR is_ancestor_of(X)
)
AND active = true
```

### Metrics

- **Processing Speed**: ⭐⭐⭐⭐⭐ (Only process changed files)
- **Database Size**: ⭐⭐⭐⭐ (Shared nodes for unchanged files)

### Pros
- Minimal processing for ephemeral versions
- Git does the heavy lifting for diff detection
- Natural version lineage (parent-child relationships)
- Easy to understand version history

### Cons
- Complex query logic (need to merge base + deltas)
- Requires git repository
- "Inactive" nodes accumulate over time (need garbage collection)

---

## Approach 2: Copy-on-Write with Shared References

### Concept
Base version owns all data, ephemeral versions reference base with overrides.

### Structure

**Base Version**: Standard full index

**Ephemeral Versions**:
- Shadow table/collection with only modifications
- Query checks ephemeral first, falls back to base
- File hash-based deduplication

### Database Schema

```sql
-- Base tables (version 1)
base_nodes: {id, data, file_path, ...}
base_chunks: {id, data, file_path, ...}
base_ngrams: {file_path, ngram_data, ...}

-- Ephemeral tables (unified with version_id)
ephemeral_nodes: {
  version_id,
  node_id,
  action,      -- 'add', 'modify', 'delete'
  data         -- NULL if action='delete'
}

ephemeral_chunks: {
  version_id,
  chunk_id,
  action,
  data
}

ephemeral_ngrams: {
  version_id,
  file_path,
  action,
  data
}

-- File content hash tracking (deduplication)
file_hashes: {
  version_id,
  file_path,
  content_hash
}
```

### Query Pattern

```javascript
// Get node in version X:
function getNode(nodeId, versionId) {
  // 1. Check ephemeral layer first
  ephemeralNode = query("SELECT * FROM ephemeral_nodes WHERE version_id=? AND node_id=?", versionId, nodeId)

  if (ephemeralNode) {
    if (ephemeralNode.action === 'delete') {
      return null  // Node deleted in this version
    }
    return ephemeralNode.data  // Modified or added
  }

  // 2. Fall back to base
  return query("SELECT * FROM base_nodes WHERE id=?", nodeId)
}
```

### Processing Logic

```python
def create_ephemeral_version(commit_sha, base_version_id):
    # 1. Diff files against base
    changed_files = git_diff(base_version, commit_sha)

    for file_path, status in changed_files:
        content = read_file(file_path, commit_sha)
        content_hash = hash(content)

        # 2. Check if we've processed this content before
        existing = query("SELECT * FROM file_hashes WHERE content_hash=?", content_hash)

        if existing:
            # 3. Copy reference (no processing needed!)
            copy_reference(existing, new_version_id)
        else:
            # 4. Process new/changed file
            process_file(file_path, content, new_version_id)
```

### Metrics

- **Processing Speed**: ⭐⭐⭐⭐⭐ (Only process new/changed files)
- **Database Size**: ⭐⭐⭐⭐⭐ (Maximum sharing, minimal duplication)

### Pros
- Excellent deduplication (same file content = same hash)
- Simple to implement
- Works with or without git
- Can track content across file renames
- Easy cleanup (delete ephemeral rows)

### Cons
- Query complexity increases (need merge logic)
- Need merge logic at read time
- May need indexes on (version_id, node_id) for performance

---

## Approach 3: Multi-Version Collections with Smart Indexing

### Concept
Separate namespace per version, but share underlying vector embeddings and graph nodes via content-addressing.

### Structure

**Base Version**: `repo_v1_graph`, `repo_v1_vectors`, `repo_v1_ngrams`

**Ephemeral Versions**: `repo_v2_graph`, `repo_v2_vectors`, `repo_v2_ngrams`

**Shared Storage**: Content-addressed pool for vectors and parsed ASTs

### Implementation

```sql
-- Shared content-addressed storage (global pool)
code_embeddings: {
  content_hash,
  embedding_vector,  -- 768-dimensional vector
  ref_count         -- For garbage collection
}

ast_cache: {
  file_path,
  content_hash,
  parsed_ast,       -- Serialized AST
  ref_count
}

-- Per-version collections reference shared storage
repo_v1_chunks: {
  chunk_id,
  file_path,
  content_hash_ref,  -- FK to code_embeddings
  metadata
}

repo_v2_chunks: {
  chunk_id,
  file_path,
  content_hash_ref,
  metadata
}

-- Neo4j example
(:CodeGraph_v1)-[:HAS_NODE]->(:Node {content_hash: "abc123"})
(:CodeGraph_v2)-[:HAS_NODE]->(:Node {content_hash: "abc123"})  // Same node!
```

### Processing Logic

```python
def create_version(version_id, files):
    for file_path, content in files:
        # 1. Hash content
        content_hash = hash(content)

        # 2. Check shared storage
        embedding = query("SELECT * FROM code_embeddings WHERE content_hash=?", content_hash)

        if embedding:
            # 3. Just create reference
            insert(f"repo_{version_id}_chunks", {
                "file_path": file_path,
                "content_hash_ref": content_hash
            })
            increment_ref_count(content_hash)
        else:
            # 4. Process and store in shared pool
            ast = parse(content)
            vector = embed(content)

            insert("code_embeddings", {
                "content_hash": content_hash,
                "embedding_vector": vector,
                "ref_count": 1
            })
            insert("ast_cache", {
                "content_hash": content_hash,
                "parsed_ast": ast,
                "ref_count": 1
            })

            # 5. Create reference
            insert(f"repo_{version_id}_chunks", {
                "file_path": file_path,
                "content_hash_ref": content_hash
            })
```

### Cleanup

```python
def delete_version(version_id):
    # 1. Get all content hashes used by this version
    hashes = query(f"SELECT content_hash_ref FROM repo_{version_id}_chunks")

    # 2. Drop the version collections
    drop_collection(f"repo_{version_id}_chunks")
    drop_collection(f"repo_{version_id}_graph")

    # 3. Decrement ref counts
    for hash in hashes:
        decrement_ref_count(hash)

        # 4. Garbage collect if ref_count = 0
        if get_ref_count(hash) == 0:
            delete("code_embeddings WHERE content_hash=?", hash)
            delete("ast_cache WHERE content_hash=?", hash)
```

### Metrics

- **Processing Speed**: ⭐⭐⭐⭐⭐ (Hash-based skip, no reprocessing)
- **Database Size**: ⭐⭐⭐⭐ (Deduplication via content-addressing)

### Pros
- Clean isolation between versions (no query complexity)
- Fast ephemeral version creation (just create references)
- Easy cleanup (drop namespace/collection)
- Efficient for vector databases (Qdrant collections)
- Natural fit for separate graphs in Neo4j

### Cons
- Database size grows with unique content across versions
- Need garbage collection for shared pool
- Reference counting overhead
- Multiple collections/namespaces to manage

---

## Approach 4: Temporal Database with Validity Ranges

### Concept
Single unified database with temporal validity columns (bitemporal design).

### Structure

```sql
nodes: {
  id,
  data,
  valid_from_version,
  valid_to_version,      -- NULL = still valid
  created_at,
  deleted_at             -- NULL = not deleted
}

chunks: {
  id,
  data,
  valid_from_version,
  valid_to_version
}

ngrams: {
  file_path,
  data,
  valid_from_version,
  valid_to_version
}
```

### Query Pattern

```sql
-- Get data for version X
SELECT * FROM nodes
WHERE valid_from_version <= X
  AND (valid_to_version IS NULL OR valid_to_version > X)
  AND deleted_at IS NULL

-- Get data valid between versions 5 and 10
SELECT * FROM nodes
WHERE valid_from_version <= 10
  AND (valid_to_version IS NULL OR valid_to_version >= 5)
```

### Processing Logic

```python
def create_version(new_version_id, base_version_id):
    changed_files = diff(base_version_id, new_version_id)

    for file_path, status in changed_files:
        if status == 'modified':
            # 1. Close validity of old records
            update("""
                UPDATE nodes
                SET valid_to_version = ?
                WHERE file_path = ?
                  AND valid_to_version IS NULL
            """, new_version_id, file_path)

            # 2. Insert new records
            new_nodes = parse_and_analyze(file_path)
            for node in new_nodes:
                insert("nodes", {
                    **node,
                    "valid_from_version": new_version_id,
                    "valid_to_version": None
                })

        elif status == 'deleted':
            # Set deleted_at
            update("""
                UPDATE nodes
                SET deleted_at = NOW()
                WHERE file_path = ?
                  AND valid_to_version IS NULL
            """, file_path)
```

### Metrics

- **Processing Speed**: ⭐⭐⭐ (Need to invalidate old records)
- **Database Size**: ⭐⭐⭐ (Keeps historical data)

### Pros
- Full version history (temporal queries)
- Standard database feature (PostgreSQL, SQL Server support)
- Simple queries (just add version predicates)
- Can query historical state

### Cons
- Slower than delta approaches (more rows to scan)
- Database grows with every version
- Need periodic compaction/archival
- Indexes on validity columns required

---

## Approach 5: Hybrid: Base + Uncommitted Layer + Commit Cache

### Concept
Three-tier system optimized for the common case (analyzing current working directory).

### Tiers

**1. Base (v1)**: Full index from last `--build-index`
- PostgreSQL/Neo4j for graph
- Qdrant for vectors
- File-based for n-grams

**2. Uncommitted Layer**: Hot cache for working directory changes
- Redis or in-memory cache
- Only stores modified files since base
- TTL-based expiration (e.g., 1 hour)

**3. Commit Cache**: LRU cache of recent commits
- Stores deltas for frequently accessed commits
- Disk-based cache (e.g., 100 most recent commits)
- Evicts least recently used

### Structure

```
┌─────────────────────────────────────┐
│   Uncommitted Layer (Redis)         │
│   - Hot modifications               │
│   - TTL: 1 hour                     │
│   - In-memory speed                 │
└─────────────────────────────────────┘
              ↓
┌─────────────────────────────────────┐
│   Commit Cache (Disk LRU)           │
│   - Recent 100 commits              │
│   - Delta storage                   │
│   - Fast retrieval                  │
└─────────────────────────────────────┘
              ↓
┌─────────────────────────────────────┐
│   Base Version (Database)           │
│   - Full index                      │
│   - Persistent                      │
│   - Source of truth                 │
└─────────────────────────────────────┘
```

### Processing Logic

```python
class HybridVersionManager:
    def __init__(self):
        self.base = Database()           # PostgreSQL/Neo4j
        self.uncommitted = Redis()       # In-memory cache
        self.commit_cache = LRUCache(100)  # Disk cache

    def get_for_commit(self, commit_sha):
        # 1. Check commit cache
        if commit_sha in self.commit_cache:
            return self.commit_cache.get(commit_sha)

        # 2. Diff against base
        changed_files = git_diff(self.base.commit_sha, commit_sha)

        # 3. Process only changed files
        delta = process_files(changed_files)

        # 4. Store in commit cache
        self.commit_cache.put(commit_sha, delta)

        # 5. Return merged view
        return merge(self.base, delta)

    def get_for_uncommitted(self):
        # 1. Check Redis cache
        cache_key = f"uncommitted:{current_workspace_hash()}"

        if self.uncommitted.exists(cache_key):
            return self.uncommitted.get(cache_key)

        # 2. Diff working directory vs HEAD
        changed_files = git_diff_working_dir()

        # 3. Process changed files
        delta = process_files(changed_files)

        # 4. Cache in Redis with TTL
        self.uncommitted.setex(cache_key, 3600, delta)

        # 5. Return merged view
        return merge(self.base, delta)

    def invalidate_uncommitted(self, file_path):
        # Invalidate cache when file changes
        self.uncommitted.delete(f"uncommitted:*:{file_path}")
```

### Cache Management

```python
class CommitCache:
    def __init__(self, max_size=100):
        self.max_size = max_size
        self.cache_dir = "./cache/commits"
        self.lru = LRU(max_size)

    def get(self, commit_sha):
        # Update LRU
        self.lru.touch(commit_sha)

        # Load from disk
        path = f"{self.cache_dir}/{commit_sha}.json"
        return load_json(path)

    def put(self, commit_sha, data):
        # Evict if full
        if len(self.lru) >= self.max_size:
            evicted = self.lru.pop_lru()
            os.remove(f"{self.cache_dir}/{evicted}.json")

        # Add to cache
        self.lru.add(commit_sha)
        path = f"{self.cache_dir}/{commit_sha}.json"
        save_json(path, data)
```

### Metrics

- **Processing Speed**: ⭐⭐⭐⭐⭐ (In-memory for uncommitted, disk cache for commits)
- **Database Size**: ⭐⭐⭐⭐⭐ (Bounded cache, aggressive eviction)

### Pros
- Optimal for IDE integration (sub-second responses)
- Fast for active development
- Bounded memory usage (controlled cache size)
- Handles both commits and uncommitted efficiently

### Cons
- Complex architecture (3 layers to manage)
- Cache invalidation complexity
- Need background cleanup/eviction
- Requires git integration

---

## Recommendation

### Choose **Approach 2 (Copy-on-Write)** if:
- ✅ Primary use case: Compare specific commits
- ✅ Multiple ephemeral versions exist simultaneously
- ✅ Need deterministic behavior
- ✅ Database queries are more common than writes
- ✅ Want to support non-git workflows

### Choose **Approach 5 (Hybrid)** if:
- ✅ Primary use case: IDE/LSP integration with live code
- ✅ Frequently analyzing uncommitted changes
- ✅ Need sub-second response for current workspace
- ✅ Can tolerate cache misses
- ✅ Have git repository available

### Hybrid of Both (Best of Both Worlds):

```
┌──────────────────────────────────────────┐
│  Tier 3: Uncommitted (Redis)             │
│  - Working directory changes             │
│  - TTL: 1 hour                           │
│  - Speed: <100ms                         │
└──────────────────────────────────────────┘
                  ↓
┌──────────────────────────────────────────┐
│  Tier 2: Recent Commits (Copy-on-Write)  │
│  - Last 100 commits                      │
│  - Delta storage in DB                   │
│  - Speed: <500ms                         │
└──────────────────────────────────────────┘
                  ↓
┌──────────────────────────────────────────┐
│  Tier 1: Base (Full Index)               │
│  - From --build-index                    │
│  - Complete graph/vectors/ngrams         │
│  - Speed: <2s                            │
└──────────────────────────────────────────┘
```

**This gives you:**
- ✅ Fast uncommitted file analysis (Redis, <100ms)
- ✅ Efficient commit comparison (CoW deltas, <500ms)
- ✅ Stable base version (full index)
- ✅ Bounded memory/disk (LRU eviction + TTL)
- ✅ Works with or without git

---

## Implementation Priorities

1. **Phase 1**: Implement Approach 2 (Copy-on-Write) for commits
   - Extend existing DB schema with ephemeral tables
   - Implement diff detection
   - Add merge logic to queries

2. **Phase 2**: Add Approach 5's uncommitted layer
   - Add Redis cache
   - Implement file watching
   - Add TTL-based invalidation

3. **Phase 3**: Optimize with content-addressing (Approach 3)
   - Add hash-based deduplication
   - Reduce processing time further

This staged approach allows incremental development and testing.
