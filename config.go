package accountsync

type Config struct {
	EncryptionKey   string
	DatabaseURL     string
	GithubUsernames []string
}
