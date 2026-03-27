package aidecisionsvc

import (
	"reflect"
	"testing"
)

func TestNormalizeActionList(t *testing.T) {
	got := normalizeActionList([]string{" A ", "a", "b", "b"})
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
