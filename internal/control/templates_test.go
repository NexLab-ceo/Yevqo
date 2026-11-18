package control

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"veltrix/internal/shared"
)

func TestAppTemplatesEndpointReturnsRunnableCatalog(t *testing.T) {
	server := &Server{}
	request := httptest.NewRequest(http.MethodGet, "/api/templates", nil)
	response := httptest.NewRecorder()
	server.handleAppTemplates(response, request)
	if response.Code != http.StatusOK { t.Fatalf("status = %d, want %d", response.Code, http.StatusOK) }
	var body struct {
		OK bool `json:"ok"`
		Templates []shared.AppTemplate `json:"templates"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil { t.Fatal(err) }
	if !body.OK || len(body.Templates) != 6 { t.Fatalf("unexpected template catalog: %#v", body) }
	want := map[string]string{"php-basic":"php", "php-api":"php", "nodejs-http":"nodejs", "nodejs-api":"nodejs", "static-site":"static", "go-http":"go"}
	for _, template := range body.Templates {
		if want[template.ID] != template.Runtime { t.Errorf("unexpected template: %#v", template) }
	}
}

func TestAppTemplatesEndpointRejectsNonGet(t *testing.T) {
	server := &Server{}
	request := httptest.NewRequest(http.MethodPost, "/api/templates", nil)
	response := httptest.NewRecorder()
	server.handleAppTemplates(response, request)
	if response.Code != http.StatusMethodNotAllowed { t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed) }
}
