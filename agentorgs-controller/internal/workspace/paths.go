package workspace

import (
	"strings"
)

// Render replaces simple {{DISPLAY_NAME}} placeholders.
func Render(content, displayName string) string {
	if displayName == "" {
		displayName = "Agent Member"
	}
	return strings.ReplaceAll(content, "{{DISPLAY_NAME}}", displayName)
}

// MemberPrefix is the object-storage key prefix for one Member workspace.
func MemberPrefix(namespace, memberName string) string {
	return namespace + "/members/" + memberName + "/"
}
