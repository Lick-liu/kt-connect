package main

import (
	"reflect"
	"testing"
)

func TestAddVersionDedupes(t *testing.T) {
	got := addVersion([]string{"jz2", "chrisr3"}, "jz2")
	want := []string{"jz2", "chrisr3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("addVersion duplicated existing version: got %v, want %v", got, want)
	}
}

func TestRemoveVersionIsIdempotent(t *testing.T) {
	got := removeVersion([]string{"jz2", "chrisr3"}, "missing")
	want := []string{"jz2", "chrisr3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("removeVersion changed versions for missing item: got %v, want %v", got, want)
	}
}

func TestRemoveVersionRemovesAllDuplicates(t *testing.T) {
	got := removeVersion([]string{"jz2", "chrisr3", "jz2", "lick", "jz2"}, "jz2")
	want := []string{"chrisr3", "lick"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("removeVersion left duplicate versions: got %v, want %v", got, want)
	}
}
