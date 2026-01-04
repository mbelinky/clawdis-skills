package main

import "testing"

func TestScaleCloudValueTemps(t *testing.T) {
	val, raw := scaleCloudValue("va_temperature", 59)
	if raw == nil {
		t.Fatalf("expected raw value for scaled temp")
	}
	if v, ok := val.(float64); !ok || v != 5.9 {
		t.Fatalf("expected 5.9, got %#v", val)
	}

	val, raw = scaleCloudValue("temp_current", 232)
	if raw == nil {
		t.Fatalf("expected raw value for scaled temp_current")
	}
	if v, ok := val.(float64); !ok || v != 23.2 {
		t.Fatalf("expected 23.2, got %#v", val)
	}
}

func TestScaleCloudValueNoScale(t *testing.T) {
	val, raw := scaleCloudValue("temp_current_f", 11)
	if raw != nil {
		t.Fatalf("expected no raw for _f")
	}
	if val != 11 {
		t.Fatalf("expected 11, got %#v", val)
	}

	val, raw = scaleCloudValue("switch_1", true)
	if raw != nil {
		t.Fatalf("expected no raw for switch")
	}
	if val != true {
		t.Fatalf("expected true, got %#v", val)
	}
}
