package workspace

import "testing"

func TestResolveSkills(t *testing.T) {
	got, err := ResolveSkills("coordination", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != CommonSkill || got[1] != "team-coordination" {
		t.Fatalf("coordination pack: %v", got)
	}

	got, err = ResolveSkills("qa", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1] != "qa-test" {
		t.Fatalf("qa pack: %v", got)
	}

	got, err = ResolveSkills("backend", []string{"team-coordination", "backend-dev"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("backend+extra dedupe want 3, got %v", got)
	}

	if _, err := ResolveSkills("unknown", nil); err == nil {
		t.Fatal("expected error for unknown pack")
	}

	got, err = ResolveSkills("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != CommonSkill {
		t.Fatalf("empty pack: %v", got)
	}
}
