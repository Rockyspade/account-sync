package accountsync

func sliceContains(sl []string, s string) bool {
	for _, candidate := range sl {
		if candidate == s {
			return true
		}
	}

	return false
}

func strPtrOrEmpty(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}
