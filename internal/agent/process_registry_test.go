package agent

import (
	"testing"
	"time"
)

func TestProcessRegistryPersistsRecords(t *testing.T) {
	root := t.TempDir()
	registry, err := NewProcessRegistry(root)
	if err != nil { t.Fatal(err) }
	want := ProcessRecord{
		SiteID: "demo", PID: 1234, ProcessGroup: 1234, Port: 3000,
		Runtime: "node", WorkingDir: "/tmp/demo", LogPath: "/tmp/demo.log",
		StartedAt: time.Unix(100, 0).UTC(),
	}
	if err := registry.Save(want); err != nil { t.Fatal(err) }

	reloaded, err := NewProcessRegistry(root)
	if err != nil { t.Fatal(err) }
	got, ok := reloaded.Get("demo")
	if !ok { t.Fatal("record missing after reload") }
	if got != want { t.Fatalf("record mismatch: got %#v, want %#v", got, want) }

	if err := reloaded.Delete("demo"); err != nil { t.Fatal(err) }
	reloaded, err = NewProcessRegistry(root)
	if err != nil { t.Fatal(err) }
	if _, ok := reloaded.Get("demo"); ok { t.Fatal("record still exists after delete") }
}
