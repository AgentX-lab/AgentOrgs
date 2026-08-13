package workspace

import "testing"

func TestMemberPrefix(t *testing.T) {
	got := MemberPrefix("agentorgs", "developer")
	want := "agentorgs/members/developer/"
	if got != want {
		t.Fatalf("MemberPrefix = %q, want %q", got, want)
	}
}

func TestRender(t *testing.T) {
	got := Render("You are {{DISPLAY_NAME}}", "Developer")
	if got != "You are Developer" {
		t.Fatalf("Render = %q", got)
	}
	got = Render("You are {{DISPLAY_NAME}}", "")
	if got != "You are Agent Member" {
		t.Fatalf("Render empty = %q", got)
	}
}
