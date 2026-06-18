package main

import (
	"errors"
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

func TestPruneUnavailableVersionsDropsOnlyMissingMeshServices(t *testing.T) {
	versions := []string{"stale", "live", "uncertain"}
	resolver := func(name string) (bool, error) {
		switch name {
		case "tea-shop-merchant-be-biz-kt-mesh-stale":
			return false, nil
		case "tea-shop-merchant-be-biz-kt-mesh-live":
			return true, nil
		case "tea-shop-merchant-be-biz-kt-mesh-uncertain":
			return false, errors.New("temporary dns failure")
		default:
			t.Fatalf("unexpected service lookup %q", name)
		}
		return false, nil
	}

	got := pruneUnavailableVersions("tea-shop-merchant-be-biz", versions, resolver)
	want := []string{"live", "uncertain"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pruneUnavailableVersions got %v, want %v", got, want)
	}
	if !reflect.DeepEqual(versions, []string{"stale", "live", "uncertain"}) {
		t.Fatalf("pruneUnavailableVersions mutated input: %v", versions)
	}
}
