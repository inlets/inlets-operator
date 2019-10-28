package uuid

import "regexp"

var isUUIDRegexp = regexp.MustCompile(`[0-9a-fA-F]{8}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{12}`)

// IsUUID returns true if the given string has a UUID format.
func IsUUID(s string) bool {
	return isUUIDRegexp.MatchString(s)
}
