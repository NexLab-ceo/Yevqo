package control

import "testing"

func TestMetricAlertTransitionUsesHysteresis(t *testing.T) {
	cases := []struct { active bool; value, threshold, recovery float64; wantActive bool; wantEvent string }{
		{false,91,90,85,true,"triggered"},
		{false,87,90,85,false,""},
		{true,88,90,85,true,""},
		{true,85,90,85,false,"recovered"},
		{false,1,1,0,true,"triggered"},
		{true,0,1,0,false,"recovered"},
	}
	for _, tc := range cases {
		gotActive, gotEvent := metricAlertTransition(tc.active,tc.value,tc.threshold,tc.recovery)
		if gotActive != tc.wantActive || gotEvent != tc.wantEvent { t.Errorf("transition(%v,%.1f)=(%v,%q), want (%v,%q)",tc.active,tc.value,gotActive,gotEvent,tc.wantActive,tc.wantEvent) }
	}
}
