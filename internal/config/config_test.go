package config

import (
	"reflect"
	"testing"
)

func TestParseEnvStringSlice(t *testing.T) {
	t.Setenv("CRONICLE_WORKER_NODE_TAGS", `["default","docker"]`)

	got, ok := parseEnvStringSlice("CRONICLE_WORKER_NODE_TAGS")
	if !ok {
		t.Fatal("expected env override to be found")
	}

	want := []string{"default", "docker"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected tags: got %#v, want %#v", got, want)
	}
}

func TestParseEnvStringSliceCommaSeparated(t *testing.T) {
	t.Setenv("CRONICLE_WORKER_NODE_TAGS", "default,docker")

	got, ok := parseEnvStringSlice("CRONICLE_WORKER_NODE_TAGS")
	if !ok {
		t.Fatal("expected env override to be found")
	}

	want := []string{"default", "docker"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected tags: got %#v, want %#v", got, want)
	}
}
