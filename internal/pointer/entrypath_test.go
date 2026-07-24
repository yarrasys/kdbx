package pointer

import (
	"reflect"
	"testing"
)

func TestParseEntryPathDefaults(t *testing.T) {
	g, title, field, err := ParseEntryPath("api/openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(g, []string{"api"}) {
		t.Fatalf("group %v, want [api]", g)
	}
	if title != "openai" || field != "password" {
		t.Fatalf("title=%q field=%q, want openai/password", title, field)
	}
}

func TestParseEntryPathNestedGroupsAndField(t *testing.T) {
	g, title, field, err := ParseEntryPath("a/b/c/Title:CUSTOM")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(g, []string{"a", "b", "c"}) {
		t.Fatalf("group %v", g)
	}
	if title != "Title" || field != "CUSTOM" {
		t.Fatalf("title=%q field=%q", title, field)
	}
}

func TestParseEntryPathBareTitleHasEmptyGroup(t *testing.T) {
	g, title, _, err := ParseEntryPath("Solo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(g) != 0 || title != "Solo" {
		t.Fatalf("group=%v title=%q, want empty group and Solo", g, title)
	}
}

func TestParseEntryPathRejectsAmbiguous(t *testing.T) {
	for _, bad := range []string{"a:b:c", "api/openai:", "api//openai", "/api/openai", "api/openai/"} {
		if _, _, _, err := ParseEntryPath(bad); err == nil {
			t.Errorf("ParseEntryPath(%q) should have failed", bad)
		}
	}
}

func TestValidVarName(t *testing.T) {
	ok := []string{"A", "_A", "OPENAI_API_KEY", "K9"}
	bad := []string{"", "9K", "lower", "WITH-DASH", "WITH SPACE", "WITH.DOT"}
	for _, s := range ok {
		if !ValidVarName(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range bad {
		if ValidVarName(s) {
			t.Errorf("%q should be invalid", s)
		}
	}
}
