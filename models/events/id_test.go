package events

import (
	"testing"
	"time"

	"github.com/gofrs/uuid"
	sqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newEventsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Event{}); err != nil {
		t.Fatal(err)
	}
	return db
}

// The producer mints the ID at construction, so a copy broadcast before or
// instead of the stored row never carries the nil UUID.
func TestNewEventMintsDistinctIDs(t *testing.T) {
	a, b := NewEvent().Build(), NewEvent().Build()
	if a.ID == uuid.Nil || b.ID == uuid.Nil {
		t.Fatalf("NewEvent minted a nil ID: %s, %s", a.ID, b.ID)
	}
	if a.ID == b.ID {
		t.Errorf("two NewEvent calls minted the same ID %s", a.ID)
	}
	if a.ID == a.OperationID {
		t.Errorf("ID and OperationID share the value %s", a.ID)
	}
}

// Both GORM write paths the ecosystem uses keep the builder's ID: Create, and
// Save (Meshery Server's EventsPersister), which updates by primary key first
// and inserts when no row matches.
func TestPersistedEventKeepsBuilderID(t *testing.T) {
	db := newEventsDB(t)
	for name, write := range map[string]func(*Event) error{
		"create": func(ev *Event) error { return db.Create(ev).Error },
		"save":   func(ev *Event) error { return db.Save(ev).Error },
	} {
		t.Run(name, func(t *testing.T) {
			ev := NewEvent().FromOwner(userID).FromSystem(systemID).WithDescription(name).Build()
			minted := ev.ID
			if err := write(ev); err != nil {
				t.Fatal(err)
			}
			if ev.ID != minted {
				t.Errorf("persist replaced ID %s with %s", minted, ev.ID)
			}
			var got Event
			if err := db.First(&got, "id = ?", minted).Error; err != nil {
				t.Fatalf("stored row not found by the builder's ID: %v", err)
			}
			if got.Description != name {
				t.Errorf("stored row description = %q, want %q", got.Description, name)
			}
		})
	}
}

// An event constructed without the builder still gets an ID at persist time.
func TestBeforeCreateAssignsNilID(t *testing.T) {
	db := newEventsDB(t)
	ev := &Event{OperationID: opID, Status: Unread, Description: "no builder"}
	if err := db.Create(ev).Error; err != nil {
		t.Fatal(err)
	}
	if ev.ID == uuid.Nil {
		t.Fatal("BeforeCreate left the ID nil")
	}
	var got Event
	if err := db.First(&got, "id = ?", ev.ID).Error; err != nil {
		t.Fatalf("stored row not found by the assigned ID: %v", err)
	}
}

// A reused builder yields distinct events: each Build has its own non-nil ID
// and persists as its own row through both GORM write paths (Meshery Server's
// EventsPersister uses Save, which updates by primary key).
func TestReusedBuilderPersistsDistinctRows(t *testing.T) {
	for name, write := range map[string]func(*gorm.DB, *Event) error{
		"create": func(db *gorm.DB, ev *Event) error { return db.Create(ev).Error },
		"save":   func(db *gorm.DB, ev *Event) error { return db.Save(ev).Error },
	} {
		t.Run(name, func(t *testing.T) {
			db := newEventsDB(t)
			b := NewEvent().FromOwner(userID).FromSystem(systemID).WithCategory("connection")
			first := b.WithDescription("context a failed").Build()
			second := b.WithDescription("import receipt").Build()
			if first.ID == uuid.Nil || second.ID == uuid.Nil || first.ID == second.ID {
				t.Fatalf("reused builder IDs = %s, %s; want distinct non-nil", first.ID, second.ID)
			}
			if err := write(db, first); err != nil {
				t.Fatal(err)
			}
			if err := write(db, second); err != nil {
				t.Fatal(err)
			}
			var rows []Event
			if err := db.Order("description").Find(&rows).Error; err != nil {
				t.Fatal(err)
			}
			if len(rows) != 2 {
				t.Fatalf("stored %d rows, want 2", len(rows))
			}
			if rows[0].ID != first.ID || rows[0].Description != "context a failed" ||
				rows[1].ID != second.ID || rows[1].Description != "import receipt" {
				t.Errorf("stored rows = %+v", rows)
			}
		})
	}
}

// Mutating the builder after Build, including in-place metadata writes, never
// reaches an event already built.
func TestBuildIsolatesBuiltEventFromBuilder(t *testing.T) {
	b := NewEvent().WithDescription("before").
		WithMetadata(map[string]interface{}{"nested": map[string]interface{}{"k": "v"}, "list": []interface{}{"a"}})
	ev := b.WithEventError(EventError{Code: "first"}).Build()

	b.WithDescription("after").WithEventError(EventError{Code: "second"})
	b.WithMetadata(map[string]interface{}{"replaced": true})

	if ev.Description != "before" {
		t.Errorf("Description = %q, want before", ev.Description)
	}
	if got := ev.Metadata[MetadataErrorKey].(EventError).Code; got != "first" {
		t.Errorf("metadata.error code = %q, want first", got)
	}
	if _, ok := ev.Metadata["replaced"]; ok {
		t.Error("WithMetadata after Build reached the built event")
	}

	next := b.WithMetadata(map[string]interface{}{"nested": map[string]interface{}{"k": "v"}}).Build()
	next.Metadata["nested"].(map[string]interface{})["k"] = "changed"
	again := b.Build()
	if got := again.Metadata["nested"].(map[string]interface{})["k"]; got != "v" {
		t.Errorf("nested metadata write on a built event reached the builder: %v", got)
	}
}

// Each Build stamps CreatedAt with its own build time, so events from a reused
// builder order by when each was built, not when the builder was created.
func TestReusedBuilderStampsEachBuildTime(t *testing.T) {
	b := NewEvent().WithCategory("pattern")
	time.Sleep(2 * time.Millisecond)
	before := time.Now()
	first := b.WithDescription("OCI Image built").Build()
	time.Sleep(2 * time.Millisecond)
	second := b.WithDescription("OCI Artifact temporarily saved").Build()
	if first.CreatedAt.Before(before) {
		t.Errorf("first CreatedAt %s predates its Build at %s", first.CreatedAt, before)
	}
	if !second.CreatedAt.After(first.CreatedAt) {
		t.Errorf("second CreatedAt %s is not after first %s", second.CreatedAt, first.CreatedAt)
	}
}
