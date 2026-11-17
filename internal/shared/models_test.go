package shared

import "testing"

func TestMaxDevicesTotalFreePlanIsAlwaysOnePhone(t *testing.T) {
	user := &User{Status: UserFree, MaxDevices: 1, BonusDevices: 0}
	if got := user.MaxDevicesTotal(); got != 1 { t.Fatalf("Free limit = %d, want 1", got) }

	// Referral bonuses and stale database values must never raise the Free cap.
	user.MaxDevices = 5
	user.BonusDevices = 10
	if got := user.MaxDevicesTotal(); got != 1 { t.Fatalf("Free limit with bonuses = %d, want 1", got) }
}

func TestMaxDevicesTotalCommercialPlanUsesAllowance(t *testing.T) {
	user := &User{Status: UserCommercial, MaxDevices: 3, BonusDevices: 2}
	if got := user.MaxDevicesTotal(); got != 5 { t.Fatalf("Commercial limit = %d, want 5", got) }
}

func TestMaxDevicesTotalHasSafeMinimum(t *testing.T) {
	user := &User{Status: UserCommercial}
	if got := user.MaxDevicesTotal(); got != 1 { t.Fatalf("minimum limit = %d, want 1", got) }
}
