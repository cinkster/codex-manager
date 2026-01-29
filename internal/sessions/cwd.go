package sessions

import "strings"

// UnknownCwd is used when a session has no working directory metadata.
const UnknownCwd = "(unknown)"

// NormalizeCwd maps empty/whitespace values to UnknownCwd.
func NormalizeCwd(value string) string {
	if strings.TrimSpace(value) == "" {
		return UnknownCwd
	}
	return value
}

// CwdForFile returns the normalized working directory for a session file.
func CwdForFile(file SessionFile) string {
	if file.Meta != nil {
		return NormalizeCwd(file.Meta.Cwd)
	}
	return UnknownCwd
}
