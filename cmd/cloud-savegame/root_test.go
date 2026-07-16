package main

import (
	"reflect"
	"testing"
)

func TestAppendUnique(t *testing.T) {
	t.Parallel()

	var list []string
	list = appendUnique(list, "farming-simulator-19")
	list = appendUnique(list, "euro-truck-simulator-2")
	// Same app appears on many $documents rules; must not multiply work.
	list = appendUnique(list, "farming-simulator-19")
	list = appendUnique(list, "farming-simulator-19")
	list = appendUnique(list, "american-truck-simulator")

	want := []string{
		"farming-simulator-19",
		"euro-truck-simulator-2",
		"american-truck-simulator",
	}
	if !reflect.DeepEqual(list, want) {
		t.Fatalf("appendUnique = %#v, want %#v", list, want)
	}
}
