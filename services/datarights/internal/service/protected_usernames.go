package service

// ProtectedUsernames lists usernames that cannot be deleted.
// This check is owned by the datarights service (moved from auth).
var ProtectedUsernames = []string{"admin", "thompson"}

// isProtectedUsername checks if a username is in the protected list.
func isProtectedUsername(username string) bool {
	for _, protected := range ProtectedUsernames {
		if username == protected {
			return true
		}
	}
	return false
}
