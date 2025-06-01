package main

import "testing"

func TestBuildTelemetry(t *testing.T) {
	td := buildTelemetry(3)
	if td.Profile != 3 {
		t.Fatal("profile not set")
	}
}
