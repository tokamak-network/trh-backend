package thanos_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	thanosHandler "github.com/tokamak-network/trh-backend/pkg/api/handlers/thanos"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestHandler() *thanosHandler.ThanosDeploymentHandler {
	// ListPresets and GetPresetByID do not use ThanosDeploymentService,
	// so a zero-value handler is sufficient.
	return &thanosHandler.ThanosDeploymentHandler{}
}

func TestListPresets_Returns200WithFourPresets(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/presets", nil)

	h := newTestHandler()
	h.ListPresets(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := resp["data"].([]any)
	if !ok {
		t.Fatalf("expected data to be an array, got %T", resp["data"])
	}
	if len(data) != 4 {
		t.Errorf("expected 4 presets in response, got %d", len(data))
	}
}

func TestListPresets_ResponseContainsRequiredFields(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/presets", nil)

	h := newTestHandler()
	h.ListPresets(c)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]any)

	for _, item := range data {
		preset := item.(map[string]any)
		for _, field := range []string{"ID", "Name", "Description", "Modules", "ChainDefaults"} {
			if _, ok := preset[field]; !ok {
				t.Errorf("preset missing field %q: %v", field, preset)
			}
		}
	}
}

func TestGetPresetByID_KnownID_Returns200(t *testing.T) {
	ids := []string{"general", "defi", "gaming", "full"}

	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, "/presets/"+id, nil)
			c.Params = gin.Params{{Key: "presetId", Value: id}}

			h := newTestHandler()
			h.GetPresetByID(c)

			if w.Code != http.StatusOK {
				t.Errorf("expected 200 for preset %q, got %d", id, w.Code)
			}

			var resp map[string]any
			json.Unmarshal(w.Body.Bytes(), &resp)

			data, ok := resp["data"].(map[string]any)
			if !ok {
				t.Fatalf("expected data to be an object for preset %q", id)
			}
			if data["ID"] != id {
				t.Errorf("expected ID %q in response, got %v", id, data["ID"])
			}
		})
	}
}

func TestGetPresetByID_UnknownID_Returns404(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/presets/unknown", nil)
	c.Params = gin.Params{{Key: "presetId", Value: "unknown"}}

	h := newTestHandler()
	h.GetPresetByID(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown preset, got %d", w.Code)
	}
}

func TestGetPresetByID_EmptyID_Returns404(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/presets/", nil)
	c.Params = gin.Params{{Key: "presetId", Value: ""}}

	h := newTestHandler()
	h.GetPresetByID(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for empty preset ID, got %d", w.Code)
	}
}

func TestGetFundingStatus_MissingID_Returns400(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/stacks//funding-status", nil)
	// id param intentionally empty

	h := newTestHandler()
	h.GetFundingStatus(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when id is missing, got %d", w.Code)
	}
}
