package events

import (
	"testing"

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

func TestWithIDReplacesMintedID(t *testing.T) {
	id := uuid.Must(uuid.FromString("55555555-5555-4555-8555-555555555555"))
	if got := NewEvent().WithID(id).Build().ID; got != id {
		t.Errorf("ID = %s, want %s", got, id)
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

// Meshery Server reuses one builder for several events and persists each by
// value with Save; every built event keeps its own ID and its own row.
func TestReusedBuilderBuildsDistinctEvents(t *testing.T) {
	db := newEventsDB(t)
	b := NewEvent().FromOwner(userID).FromSystem(systemID)
	first := b.WithDescription("first").Build()
	second := b.WithDescription("second").Build()
	if first.ID == uuid.Nil || second.ID == uuid.Nil || first.ID == second.ID {
		t.Fatalf("reused builder IDs: %s, %s", first.ID, second.ID)
	}
	if first.Description != "first" {
		t.Errorf("a later builder call changed an already built event: %q", first.Description)
	}
	for _, ev := range []Event{*first, *second} {
		if err := db.Save(&ev).Error; err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	if err := db.Model(&Event{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("stored %d rows, want 2", count)
	}
	id := uuid.Must(uuid.FromString("66666666-6666-4666-8666-666666666666"))
	if got := b.WithID(id).Build().ID; got != id {
		t.Errorf("WithID on a reused builder: ID = %s, want %s", got, id)
	}
	if got := b.Build().ID; got == id || got == uuid.Nil {
		t.Errorf("Build after a pinned ID returned %s", got)
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
