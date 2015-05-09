package accountsync

func sliceContains(sl []string, s string) bool {
	for _, candidate := range sl {
		if candidate == s {
			return true
		}
	}

	return false
}
