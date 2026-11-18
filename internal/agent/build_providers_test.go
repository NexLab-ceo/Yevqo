package agent

import (
	"strings"
	"testing"
)

func TestResolveBuildProviderKeepsRemoteBuildsOffAndroid(t *testing.T) {
	android := AndroidBuildEnvironment{Android:true, Termux:true}
	for _, id := range []string{"dockerfile","nixpacks","railpack"} {
		provider, err := ResolveBuildProvider(id, android, true)
		if err == nil { t.Fatalf("%s unexpectedly resolved on Android", id) }
		if provider.Available { t.Fatalf("%s marked available on Android", id) }
		if !strings.Contains(err.Error(), "remote Linux executor") { t.Fatalf("unexpected error: %v", err) }
	}
}

func TestResolveBuildProviderRequiresOnlineExecutor(t *testing.T) {
	provider, err := ResolveBuildProvider("nixpacks", AndroidBuildEnvironment{}, false)
	if err == nil || provider.Available { t.Fatalf("expected unavailable provider, got %#v, %v", provider, err) }
	provider, err = ResolveBuildProvider("railpack", AndroidBuildEnvironment{}, true)
	if err != nil || !provider.Available { t.Fatalf("expected available remote provider, got %#v, %v", provider, err) }
}

func TestResolveBuildProviderAndroidLocal(t *testing.T) {
	provider, err := ResolveBuildProvider("auto", AndroidBuildEnvironment{Android:true, Termux:true}, false)
	if err != nil || provider.ID != "android-local" || !provider.Available { t.Fatalf("unexpected result: %#v, %v", provider, err) }
}
