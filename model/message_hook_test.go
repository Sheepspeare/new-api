package model

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Property 10: Database Compatibility
// Validates: Requirements 10.1, 10.2, 10.3, 10.7
// This test verifies that MessageHook model works identically across SQLite, MySQL, and PostgreSQL

func TestMessageHookDatabaseCompatibility(t *testing.T) {
	// Test with SQLite (in-memory)
	t.Run("SQLite", func(t *testing.T) {
		db := setupTestDB(t, "sqlite")
		defer cleanupTestDB(db)
		testMessageHookCRUD(t, db)
		testMessageHookFilters(t, db)
		testMessageHookStatistics(t, db)
	})

	// Note: MySQL and PostgreSQL tests require running database instances
	// These should be run in CI/CD environment with proper database setup
	// For local development, SQLite tests provide basic validation
}

func setupTestDB(t *testing.T, dbType string) *gorm.DB {
	var db *gorm.DB
	var err error

	switch dbType {
	case "sqlite":
		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		if err != nil {
			t.Fatalf("Failed to connect to SQLite: %v", err)
		}
	// Add MySQL and PostgreSQL cases when testing in CI/CD
	default:
		t.Fatalf("Unsupported database type: %s", dbType)
	}

	// Run migration
	err = db.AutoMigrate(&MessageHook{})
	if err != nil {
		t.Fatalf("Failed to migrate MessageHook table: %v", err)
	}

	// Set global DB for model functions
	DB = db

	return db
}

func cleanupTestDB(db *gorm.DB) {
	sqlDB, _ := db.DB()
	if sqlDB != nil {
		sqlDB.Close()
	}
}

// testMessageHookCRUD tests Create, Read, Update, Delete operations
func testMessageHookCRUD(t *testing.T, db *gorm.DB) {
	// Property: CRUD operations should work identically across all databases

	// Create
	hook := generateRandomMessageHook()
	err := CreateMessageHook(hook)
	if err != nil {
		t.Fatalf("Failed to create message hook: %v", err)
	}
	if hook.Id == 0 {
		t.Error("Expected hook ID to be set after creation")
	}
	if hook.CreatedTime == 0 {
		t.Error("Expected CreatedTime to be set")
	}
	if hook.UpdatedTime == 0 {
		t.Error("Expected UpdatedTime to be set")
	}

	// Read
	retrieved, err := GetMessageHook(hook.Id)
	if err != nil {
		t.Fatalf("Failed to get message hook: %v", err)
	}
	if retrieved.Name != hook.Name {
		t.Errorf("Expected name %s, got %s", hook.Name, retrieved.Name)
	}
	if retrieved.Type != hook.Type {
		t.Errorf("Expected type %d, got %d", hook.Type, retrieved.Type)
	}
	if retrieved.Enabled != hook.Enabled {
		t.Errorf("Expected enabled %v, got %v", hook.Enabled, retrieved.Enabled)
	}

	// Update
	retrieved.Name = "Updated Name"
	retrieved.Description = "Updated Description"
	retrieved.Enabled = !retrieved.Enabled
	err = UpdateMessageHook(retrieved)
	if err != nil {
		t.Fatalf("Failed to update message hook: %v", err)
	}

	updated, err := GetMessageHook(hook.Id)
	if err != nil {
		t.Fatalf("Failed to get updated message hook: %v", err)
	}
	if updated.Name != "Updated Name" {
		t.Errorf("Expected updated name, got %s", updated.Name)
	}
	if updated.Description != "Updated Description" {
		t.Errorf("Expected updated description, got %s", updated.Description)
	}
	if updated.UpdatedTime <= hook.UpdatedTime {
		t.Error("Expected UpdatedTime to be updated")
	}

	// Delete
	err = DeleteMessageHook(hook.Id)
	if err != nil {
		t.Fatalf("Failed to delete message hook: %v", err)
	}

	_, err = GetMessageHook(hook.Id)
	if err == nil {
		t.Error("Expected error when getting deleted hook")
	}
}

// testMessageHookFilters tests JSON filter storage and retrieval
func testMessageHookFilters(t *testing.T, db *gorm.DB) {
	// Property: TEXT columns for JSON filters should work across all databases

	hook := &MessageHook{
		Name:         "Filter Test Hook",
		Type:         1,
		Content:      "test content",
		FilterUsers:  `[1, 2, 3]`,
		FilterModels: `["gpt-4", "gpt-3.5-turbo"]`,
		FilterTokens: `[100, 200]`,
	}

	err := CreateMessageHook(hook)
	if err != nil {
		t.Fatalf("Failed to create hook with filters: %v", err)
	}

	retrieved, err := GetMessageHook(hook.Id)
	if err != nil {
		t.Fatalf("Failed to retrieve hook: %v", err)
	}

	if retrieved.FilterUsers != hook.FilterUsers {
		t.Errorf("FilterUsers mismatch: expected %s, got %s", hook.FilterUsers, retrieved.FilterUsers)
	}
	if retrieved.FilterModels != hook.FilterModels {
		t.Errorf("FilterModels mismatch: expected %s, got %s", hook.FilterModels, retrieved.FilterModels)
	}
	if retrieved.FilterTokens != hook.FilterTokens {
		t.Errorf("FilterTokens mismatch: expected %s, got %s", hook.FilterTokens, retrieved.FilterTokens)
	}

	// Test empty filters
	hook2 := &MessageHook{
		Name:         "Empty Filter Hook",
		Type:         1,
		Content:      "test",
		FilterUsers:  "",
		FilterModels: "",
		FilterTokens: "",
	}

	err = CreateMessageHook(hook2)
	if err != nil {
		t.Fatalf("Failed to create hook with empty filters: %v", err)
	}

	retrieved2, err := GetMessageHook(hook2.Id)
	if err != nil {
		t.Fatalf("Failed to retrieve hook with empty filters: %v", err)
	}

	if retrieved2.FilterUsers != "" {
		t.Error("Expected empty FilterUsers")
	}
}

// testMessageHookStatistics tests statistics update functionality
func testMessageHookStatistics(t *testing.T, db *gorm.DB) {
	// Property: Statistics updates should be atomic and work across all databases

	hook := generateRandomMessageHook()
	err := CreateMessageHook(hook)
	if err != nil {
		t.Fatalf("Failed to create hook: %v", err)
	}

	// Initial statistics should be zero
	if hook.CallCount != 0 || hook.SuccessCount != 0 || hook.ErrorCount != 0 {
		t.Error("Expected initial statistics to be zero")
	}

	// Update statistics - success
	err = UpdateMessageHookStats(hook.Id, true, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to update stats: %v", err)
	}

	retrieved, _ := GetMessageHook(hook.Id)
	if retrieved.CallCount != 1 {
		t.Errorf("Expected CallCount 1, got %d", retrieved.CallCount)
	}
	if retrieved.SuccessCount != 1 {
		t.Errorf("Expected SuccessCount 1, got %d", retrieved.SuccessCount)
	}
	if retrieved.ErrorCount != 0 {
		t.Errorf("Expected ErrorCount 0, got %d", retrieved.ErrorCount)
	}
	if retrieved.AvgDuration != 100.0 {
		t.Errorf("Expected AvgDuration 100.0, got %f", retrieved.AvgDuration)
	}

	// Update statistics - failure
	err = UpdateMessageHookStats(hook.Id, false, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to update stats: %v", err)
	}

	retrieved, _ = GetMessageHook(hook.Id)
	if retrieved.CallCount != 2 {
		t.Errorf("Expected CallCount 2, got %d", retrieved.CallCount)
	}
	if retrieved.SuccessCount != 1 {
		t.Errorf("Expected SuccessCount 1, got %d", retrieved.SuccessCount)
	}
	if retrieved.ErrorCount != 1 {
		t.Errorf("Expected ErrorCount 1, got %d", retrieved.ErrorCount)
	}
	// Average should be (100 + 200) / 2 = 150
	if retrieved.AvgDuration != 150.0 {
		t.Errorf("Expected AvgDuration 150.0, got %f", retrieved.AvgDuration)
	}
}

func TestGetAllMessageHooks(t *testing.T) {
	db := setupTestDB(t, "sqlite")
	defer cleanupTestDB(db)

	// Create multiple hooks
	for i := 0; i < 5; i++ {
		hook := generateRandomMessageHook()
		hook.Name = fmt.Sprintf("Hook %d", i)
		hook.Priority = i
		hook.Enabled = i%2 == 0 // Alternate enabled/disabled
		err := CreateMessageHook(hook)
		if err != nil {
			t.Fatalf("Failed to create hook %d: %v", i, err)
		}
	}

	// Test pagination
	hooks, total, err := GetAllMessageHooks(1, 2, nil)
	if err != nil {
		t.Fatalf("Failed to get hooks: %v", err)
	}
	if total != 5 {
		t.Errorf("Expected total 5, got %d", total)
	}
	if len(hooks) != 2 {
		t.Errorf("Expected 2 hooks in page, got %d", len(hooks))
	}

	// Test enabled filter
	enabled := true
	hooks, total, err = GetAllMessageHooks(0, 0, &enabled)
	if err != nil {
		t.Fatalf("Failed to get enabled hooks: %v", err)
	}
	if total != 3 {
		t.Errorf("Expected 3 enabled hooks, got %d", total)
	}
	for _, hook := range hooks {
		if !hook.Enabled {
			t.Error("Expected only enabled hooks")
		}
	}
}

func TestGetEnabledMessageHooks(t *testing.T) {
	db := setupTestDB(t, "sqlite")
	defer cleanupTestDB(db)

	// Create hooks with different priorities
	priorities := []int{10, 5, 20, 1, 15}
	for i, priority := range priorities {
		hook := generateRandomMessageHook()
		hook.Name = fmt.Sprintf("Hook %d", i)
		hook.Priority = priority
		hook.Enabled = true
		err := CreateMessageHook(hook)
		if err != nil {
			t.Fatalf("Failed to create hook: %v", err)
		}
	}

	// Create one disabled hook
	disabledHook := generateRandomMessageHook()
	disabledHook.Enabled = false
	CreateMessageHook(disabledHook)

	hooks, err := GetEnabledMessageHooks()
	if err != nil {
		t.Fatalf("Failed to get enabled hooks: %v", err)
	}

	if len(hooks) != 5 {
		t.Errorf("Expected 5 enabled hooks, got %d", len(hooks))
	}

	// Verify priority ordering (ascending)
	for i := 1; i < len(hooks); i++ {
		if hooks[i].Priority < hooks[i-1].Priority {
			t.Error("Hooks not sorted by priority")
		}
	}
}

func TestMessageHookUniqueNameConstraint(t *testing.T) {
	db := setupTestDB(t, "sqlite")
	defer cleanupTestDB(db)

	hook1 := generateRandomMessageHook()
	hook1.Name = "Unique Name"
	err := CreateMessageHook(hook1)
	if err != nil {
		t.Fatalf("Failed to create first hook: %v", err)
	}

	hook2 := generateRandomMessageHook()
	hook2.Name = "Unique Name" // Same name
	err = CreateMessageHook(hook2)
	if err == nil {
		t.Error("Expected error when creating hook with duplicate name")
	}
}

// Helper function to generate random MessageHook for property-based testing
func generateRandomMessageHook() *MessageHook {
	rand.Seed(time.Now().UnixNano())
	return &MessageHook{
		Name:        fmt.Sprintf("Hook_%d", rand.Int()),
		Description: fmt.Sprintf("Description %d", rand.Int()),
		Type:        rand.Intn(2) + 1, // 1 or 2
		Content:     fmt.Sprintf("Content %d", rand.Int()),
		Enabled:     rand.Intn(2) == 1,
		Priority:    rand.Intn(100),
		Timeout:     rand.Intn(30000) + 100,
	}
}
