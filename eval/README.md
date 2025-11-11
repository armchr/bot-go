# Vector Search Evaluation Test Cases

This directory contains test cases for evaluating the quality of vector search for code chunks. Each test case contains code snippets and metadata describing their similarity relationships.

## Test Case Structure

Each test case is in its own subdirectory with:
- **Code snippet files**: The actual code samples extracted from the codebase
- **metadata.json**: Describes the test case, including:
  - Test case ID and name
  - Snippet source locations (repo, file path, line numbers)
  - Similarity relationships (similar_pairs and different_pairs)
  - Expected behavior notes

## Test Cases Overview

### Similar Code Pairs (Positive Tests)

These test cases contain code that SHOULD rank highly in similarity searches:

**Test Case 01: LSP Client Initialization Pattern**
- 3 near-identical constructor functions for different language servers
- Location: `bot-go/pkg/lsp/`
- Similarity: Very High
- Difference: Only function names and config paths differ

**Test Case 02: TODO/FIXME Comment Detection**
- 2 identical static analysis rules across Go and Python analyzers
- Location: `armchair/code_reviewer/internal/analyzer/static/`
- Similarity: Very High
- Difference: Only comment text differs

**Test Case 03: Type Conversion Helper Functions**
- 2 type conversion functions (int64 vs int32)
- Location: `bot-go/internal/service/code_graph.go`
- Similarity: Very High
- Difference: Only target type differs, could be generified

**Test Case 06: Debug Print Statement Detection**
- 2 similar patterns detecting print statements (Python vs JavaScript)
- Location: `armchair/code_reviewer/internal/analyzer/static/`
- Similarity: High
- Difference: Different language-specific checks

### Different Code Samples (Negative Tests)

These test cases contain code that SHOULD NOT rank highly in similarity searches:

**Test Case 04: Different Search Methods**
- Bloom filter operations vs Vector similarity search
- Location: `bot-go/internal/util/` vs `bot-go/internal/service/`
- Difference: Completely different data structures and algorithms

**Test Case 05: Different Algorithms**
- Concurrent directory walking vs Git diff parsing
- Location: `bot-go/internal/util/` vs `armchair/code_reviewer/internal/parser/`
- Difference: Concurrent systems programming vs text parsing

**Test Case 07: Different External Integrations**
- LLM API integration vs Graph database queries
- Location: `armchair/code_reviewer/internal/analyzer/llm/` vs `bot-go/internal/service/`
- Difference: External AI service vs local database, different concerns

## Building and Running Evaluation

### Build the Evaluation Tool

```bash
# Build the evaluation binary
make build-eval

# This creates bin/run_eval
```

### Running Evaluation

**Evaluate a single test case:**
```bash
make run-eval TEST=eval/test_case_01_lsp_client_init OUTPUT=results_01.json
```

**Evaluate all test cases:**
```bash
make run-eval TEST=eval OUTPUT=all_results.json
```

**Direct command (after building):**
```bash
# Single test case
bin/run_eval -test eval/test_case_01_lsp_client_init -output results_01.json

# All test cases
bin/run_eval -test eval -output all_results.json

# With custom config files
bin/run_eval -test eval -output results.json \
  -app config/app.yaml -source config/source.yaml
```

### Output

The evaluation tool generates:

1. **JSON file** (`eval_results.json` by default) with detailed results:
   - Similarity scores for each snippet pair
   - Test case metadata
   - Aggregate statistics

2. **Console output** with summary:
   - Total test cases evaluated
   - Average similarity scores for "similar" vs "different" types
   - High/low similarity counts
   - Detailed scores for each test case

### Example Output

```
=== EVALUATION SUMMARY ===

Total Test Cases:     7
Similar Test Cases:   12  (snippet pairs)
Different Test Cases: 3   (snippet pairs)

Average Similarity Score (Similar):   0.9234
Average Similarity Score (Different): 0.3421

High Similarity Count (>0.85): 11
Low Similarity Count (<0.50):  3

=== TEST CASE DETAILS ===

LSP Client Initialization Pattern (similar):
  snippet_go_client.go                     1.0000
  snippet_python_client.go                 0.9534
  snippet_typescript_client.go             0.9421
...
```

### Prerequisites

Before running evaluation:

1. **Ollama must be running** with the configured embedding model:
   ```bash
   # Check config/app.yaml for ollama settings
   ollama pull nomic-embed-text  # or your configured model
   ```

2. **Configuration files** must exist:
   - `config/app.yaml` - with Ollama URL, model, and dimension
   - `config/source.yaml` - repository configuration

### Interpreting Results

1. Generate embeddings for all snippets in each test case
2. For similar pairs:
   - Query with snippet A
   - Verify snippet B ranks in top K results
   - Calculate precision/recall
3. For different pairs:
   - Query with snippet A
   - Verify snippet B does NOT rank in top K results
   - Calculate false positive rate

### Metrics to Calculate

- **Precision@K**: For similar pairs, what % of top K results are truly similar?
- **Recall@K**: For similar pairs, what % appear in top K?
- **False Positive Rate**: For different pairs, how often do they incorrectly appear in top K?
- **Mean Reciprocal Rank (MRR)**: Average rank of first relevant result

### Expected Results

**Good Vector Search Performance:**
- Similar pairs should rank in top 3-5 results
- Different pairs should NOT appear in top 10 results
- MRR should be > 0.5 for similar pairs

**Poor Vector Search Performance:**
- Similar pairs rank below position 10
- Different pairs appear in top 5 results
- High false positive rate (>20%)

## Test Case Statistics

- **Total test cases**: 7
- **Similar pairs**: 4 test cases (7 snippet pairs total)
- **Different pairs**: 3 test cases (3 snippet pairs total)
- **Languages covered**: Go (all snippets are from Go codebases)
- **Repositories**: bot-go and armchair

## Notes

- All snippets are real production code from the bot-go and armchair codebases
- Test cases cover common patterns: constructors, static analysis rules, type conversions, and different domains (concurrency, parsing, databases, AI)
- Metadata includes exact line numbers and file paths for reproducibility
- Some test cases have multiple similar snippets (e.g., Test Case 01 has 3 snippets that are all similar to each other)

## Future Improvements

- Add cross-language similarity tests (e.g., same algorithm in Go vs Python)
- Add refactoring tests (same logic, different structure)
- Add comment-vs-code tests (does code match its docstring?)
- Add more granular similarity levels (identical, near-identical, similar, somewhat-similar)
