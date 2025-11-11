#!/bin/bash

# Simple test for code search API
# Usage: ./test_search_simple.sh [include_code]
# Example: ./test_search_simple.sh true

INCLUDE_CODE="${1:-false}"

curl -X POST http://localhost:8181/api/v1/searchSimilarCode \
  -H "Content-Type: application/json" \
  -d '{
    "repo_name": "bot-go",
    "code_snippet": "func ProcessFile(ctx context.Context, filePath string) error {\n\tsourceCode, err := readFile(filePath)\n\tif err != nil {\n\t\treturn fmt.Errorf(\"failed to read: %w\", err)\n\t}\n\treturn nil\n}",
    "language": "go",
    "limit": 5,
    "include_code": '"$INCLUDE_CODE"'
  }' | jq '.'
