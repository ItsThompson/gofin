package service

// isProtectedUsername reports whether username is in the protected list. The
// protected list is owned by the datarights service and is
// injected from config via WithProtectedUsernames.
func isProtectedUsername(protected []string, username string) bool {
	for _, name := range protected {
		if username == name {
			return true
		}
	}
	return false
}
