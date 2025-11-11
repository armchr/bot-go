#!/bin/bash

# Test script for /api/v1/searchSimilarCode endpoint
# Make sure bot-go server is running before executing this script

BASE_URL="http://localhost:8181"

echo "=================================="
echo "Testing Code Search API"
echo "=================================="
echo ""

# Test 1: Search for similar error handling function
echo "Test 1: Search for error handling patterns"
echo "-------------------------------------------"
curl -X POST "${BASE_URL}/api/v1/searchSimilarCode" \
  -H "Content-Type: application/json" \
  -d '{
    "repo_name": "bot-go",
    "code_snippet": "func handleError(ctx context.Context, err error) {\n\tlogger.Error(\"error occurred\", zap.Error(err))\n\treturn fmt.Errorf(\"failed: %w\", err)\n}",
    "language": "go",
    "limit": 5
  }' | jq '.'
echo ""
echo ""

# Test 2: Search for HTTP handler patterns
echo "Test 2: Search for HTTP handler patterns"
echo "-----------------------------------------"
curl -X POST "${BASE_URL}/api/v1/searchSimilarCode" \
  -H "Content-Type: application/json" \
  -d '{
    "repo_name": "bot-go",
    "code_snippet": "func (rc *RepoController) HandleRequest(c *gin.Context) {\n\tvar request model.Request\n\tif err := c.ShouldBindJSON(&request); err != nil {\n\t\tc.JSON(http.StatusBadRequest, gin.H{\"error\": err.Error()})\n\t\treturn\n\t}\n}",
    "language": "go",
    "limit": 5
  }' | jq '.'
echo ""
echo ""

# Test 3: Search for database query patterns
echo "Test 3: Search for database/vector DB operations"
echo "------------------------------------------------"
curl -X POST "${BASE_URL}/api/v1/searchSimilarCode" \
  -H "Content-Type: application/json" \
  -d '{
    "repo_name": "bot-go",
    "code_snippet": "func SearchDatabase(ctx context.Context, query string) ([]Result, error) {\n\tresults, err := db.Query(ctx, query)\n\tif err != nil {\n\t\treturn nil, fmt.Errorf(\"query failed: %w\", err)\n\t}\n\treturn results, nil\n}",
    "language": "go",
    "limit": 5
  }' | jq '.'
echo ""
echo ""

# Test 4: Search for loop patterns
echo "Test 4: Search for loop patterns (if threshold allows)"
echo "------------------------------------------------------"
curl -X POST "${BASE_URL}/api/v1/searchSimilarCode" \
  -H "Content-Type: application/json" \
  -d '{
    "repo_name": "bot-go",
    "code_snippet": "for i := 0; i < len(items); i++ {\n\titem := items[i]\n\tif err := processItem(item); err != nil {\n\t\tlogger.Error(\"failed\", zap.Error(err))\n\t\tcontinue\n\t}\n\tresults = append(results, item)\n}",
    "language": "go",
    "limit": 5
  }' | jq '.'
echo ""
echo ""

# Test 5: Search with invalid language (should fail)
echo "Test 5: Invalid language (should return 400)"
echo "---------------------------------------------"
curl -X POST "${BASE_URL}/api/v1/searchSimilarCode" \
  -H "Content-Type: application/json" \
  -d '{
    "repo_name": "bot-go",
    "code_snippet": "print(\"hello world\")",
    "language": "ruby",
    "limit": 5
  }' | jq '.'
echo ""
echo ""

# Test 6: Search with custom collection name
echo "Test 6: Search with custom collection name"
echo "-------------------------------------------"
curl -X POST "${BASE_URL}/api/v1/searchSimilarCode" \
  -H "Content-Type: application/json" \
  -d '{
    "repo_name": "bot-go",
    "collection_name": "bot-go",
    "code_snippet": "func ProcessFile(ctx context.Context, filePath string) error {\n\treturn nil\n}",
    "language": "go",
    "limit": 3
  }' | jq '.'
echo ""
echo ""

echo "=================================="
echo "All tests completed!"
echo "=================================="
