package service

// protectedUsernames lists usernames that cannot be deleted.
// This check is owned by the datarights service (moved from auth).
var protectedUsernames = []string{"admin", "thompson"}

// isProtectedUsername checks if a username is in the protected list.
func isProtectedUsername(username string) bool {
	for _, protected := range protectedUsernames {
		if username == protected {
			return true
		}
	}
	return false
}
