package control

import (
	"testing"

	"veltrix/internal/shared"
)

func TestProjectLogDetailID(t *testing.T) {
	cases := []struct {
		name string
		details map[string]interface{}
		key string
		wantID int64
		wantExists bool
	}{
		{name: "missing", details: map[string]interface{}{}, key: "database_id", wantExists: false},
		{name: "JSON number", details: map[string]interface{}{"database_id": float64(42)}, key: "database_id", wantID: 42, wantExists: true},
		{name: "string number", details: map[string]interface{}{"backup_id": "17"}, key: "backup_id", wantID: 17, wantExists: true},
		{name: "fractional number", details: map[string]interface{}{"database_id": 1.5}, key: "database_id", wantExists: true},
		{name: "invalid string", details: map[string]interface{}{"backup_id": "nope"}, key: "backup_id", wantExists: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotID, gotExists := projectLogDetailID(tc.details, tc.key)
			if gotID != tc.wantID || gotExists != tc.wantExists {
				t.Fatalf("projectLogDetailID() = (%d, %v), want (%d, %v)", gotID, gotExists, tc.wantID, tc.wantExists)
			}
		})
	}
}

func TestIsProjectAccessFilterError(t *testing.T) {
	if !isProjectAccessFilterError(shared.ErrForbidden("denied")) {
		t.Fatal("forbidden errors should be filtered from project-scoped logs")
	}
	if !isProjectAccessFilterError(shared.ErrNotFound("missing")) {
		t.Fatal("not-found errors should be filtered from project-scoped logs")
	}
	if isProjectAccessFilterError(shared.ErrInternal("broken")) {
		t.Fatal("internal errors must not be silently filtered")
	}
}
