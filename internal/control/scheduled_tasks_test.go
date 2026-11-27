package control

import (
	"net/http"
	"testing"
)

func TestScheduledTaskPermission(t *testing.T) {
	cases := []struct {
		method string
		want   string
		ok     bool
	}{
		{http.MethodGet, "device.read", true},
		{http.MethodPost, "site.configure", true},
		{http.MethodPatch, "site.configure", true},
		{http.MethodDelete, "site.configure", true},
		{http.MethodPut, "", false},
		{http.MethodOptions, "", false},
	}
	for _, tc := range cases {
		got, ok := scheduledTaskPermission(tc.method)
		if got != tc.want || ok != tc.ok {
			t.Errorf("scheduledTaskPermission(%q) = (%q, %v), want (%q, %v)", tc.method, got, ok, tc.want, tc.ok)
		}
	}
}
