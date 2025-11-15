package codegraph

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestKuzuDatabase_BasicFunctionality(t *testing.T) {
	logger := zap.NewNop()

	// Create in-memory database
	db, err := NewKuzuDatabase(":memory:", logger)
	if err != nil {
		t.Fatalf("Failed to create Kuzu database: %v", err)
	}
	defer db.Close(context.Background())

	ctx := context.Background()

	// Test connectivity
	err = db.VerifyConnectivity(ctx)
	if err != nil {
		t.Fatalf("Failed to verify connectivity: %v", err)
	}

	// Test simple query without parameters
	records, err := db.ExecuteRead(ctx, "RETURN 1 as test", nil)
	if err != nil {
		t.Fatalf("Failed to execute simple query: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	if records[0]["test"] != int64(1) {
		t.Fatalf("Expected test=1, got %v", records[0]["test"])
	}
}

func TestKuzuDatabase_SingleRecordOperations(t *testing.T) {
	logger := zap.NewNop()

	db, err := NewKuzuDatabase(":memory:", logger)
	if err != nil {
		t.Fatalf("Failed to create Kuzu database: %v", err)
	}
	defer db.Close(context.Background())

	ctx := context.Background()

	// Test ExecuteReadSingle
	record, err := db.ExecuteReadSingle(ctx, "RETURN 'hello' as greeting, 42 as number", nil)
	if err != nil {
		t.Fatalf("Failed to execute single read: %v", err)
	}

	if record["greeting"] != "hello" {
		t.Fatalf("Expected greeting='hello', got %v", record["greeting"])
	}

	if record["number"] != int64(42) {
		t.Fatalf("Expected number=42, got %v", record["number"])
	}
}

func TestKuzuDatabase_ErrorHandling(t *testing.T) {
	logger := zap.NewNop()

	db, err := NewKuzuDatabase(":memory:", logger)
	if err != nil {
		t.Fatalf("Failed to create Kuzu database: %v", err)
	}
	defer db.Close(context.Background())

	ctx := context.Background()

	// Test query that returns no results
	_, err = db.ExecuteReadSingle(ctx, "RETURN 1 WHERE false", nil)
	if err == nil {
		t.Fatal("Expected error for query with no results, got nil")
	}
}
