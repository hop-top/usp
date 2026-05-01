package main

import (
	"encoding/json"
	"os"
	"testing"

	"hop.top/kit/uxp"
)

func TestDoctorRowsRender(t *testing.T) {
	checks := []uxp.Check{
		{Name: "a", Status: uxp.StatusOK, Message: "fine", Detail: "/tmp"},
		{Name: "b", Status: uxp.StatusFail, Message: "bad"},
		{Name: "c", Status: uxp.StatusSkip, Message: "n/a"},
	}
	rows := toDoctorRows(checks)
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	if rows[0].Status != "✓" {
		t.Errorf("rows[0].Status = %q, want ✓", rows[0].Status)
	}
	if rows[1].Status != "✗" {
		t.Errorf("rows[1].Status = %q, want ✗", rows[1].Status)
	}
	if rows[2].Status != "—" {
		t.Errorf("rows[2].Status = %q, want —", rows[2].Status)
	}
}

func TestDoctorRowsJSONRoundTrip(t *testing.T) {
	checks := []uxp.Check{
		{Name: "a", Status: uxp.StatusOK, Message: "ok", Detail: "/x"},
	}
	b, err := json.Marshal(toDoctorRows(checks))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got []map[string]string
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 1 || got[0]["name"] != "a" || got[0]["status"] != "✓" {
		t.Errorf("unexpected JSON: %s", b)
	}
}

func TestFormatFromViperFallback(t *testing.T) {
	rootViper.Set("format", "")
	if got := formatFromViper(); got != "table" {
		t.Errorf("formatFromViper empty = %q, want table", got)
	}
	rootViper.Set("format", "json")
	if got := formatFromViper(); got != "json" {
		t.Errorf("formatFromViper json = %q, want json", got)
	}
	rootViper.Set("format", "")
	_ = os.Stdout // touch unused import guard
}
