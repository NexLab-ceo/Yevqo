package control

import "testing"

func TestHostnameBelongsToZone(t *testing.T) {
	for _,host:=range []string{"example.com","app.example.com","a.b.example.com"}{if !hostnameBelongsToZone(host,"example.com"){t.Errorf("%s should belong to zone",host)}}
	for _,host:=range []string{"evil-example.com","example.com.evil.org","other.net",""}{if hostnameBelongsToZone(host,"example.com"){t.Errorf("%s must not belong to zone",host)}}
}

func TestCloudflareTunnelUUIDValidation(t *testing.T) {
	if !validCloudflareTunnelID("12345678-1234-abcd-9876-1234567890ab"){t.Fatal("valid tunnel UUID rejected")}
	for _,id:=range []string{"","1234","12345678-1234-1234-1234-1234567890ag","12345678_1234_abcd_9876_1234567890ab"}{if validCloudflareTunnelID(id){t.Fatalf("invalid tunnel ID %q accepted",id)}}
}

func TestCloudflareZoneIDValidation(t *testing.T) {
	if !validCloudflareZoneID("0123456789abcdef0123456789abcdef") { t.Fatal("valid zone ID rejected") }
	for _,id:=range []string{"", "not-a-zone-id", "0123456789abcdef0123456789abcdef/path", "0123456789abcdef0123456789abcde"} {
		if validCloudflareZoneID(id) { t.Fatalf("invalid zone ID %q accepted", id) }
	}
}
