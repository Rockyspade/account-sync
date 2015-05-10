package accountsync

import "github.com/codegangsta/cli"

var (
	EncryptionKeyFlag = &cli.StringFlag{
		Name:   "k, encryption-key",
		Value:  "",
		EnvVar: "TRAVIS_ACCOUNT_SYNC_ENCRYPTION_KEY",
	}
	DatabaseURLFlag = &cli.StringFlag{
		Name:   "d, database-url",
		Value:  "",
		EnvVar: "TRAVIS_ACCOUNT_SYNC_DATABASE_URL",
	}
	GithubUsernamesFlag = &cli.StringSliceFlag{
		Name:   "u, github-usernames",
		Value:  &cli.StringSlice{},
		EnvVar: "TRAVIS_ACCOUNT_SYNC_GITHUB_USERNAMES",
	}
	OrganizationsRepositoriesLimitFlag = &cli.IntFlag{
		Name:   "l, organizations-repositories-limit",
		Value:  1000,
		EnvVar: "TRAVIS_ACCOUNT_SYNC_ORGANIZATIONS_REPOSITORIES_LIMIT",
	}

	Flags = []cli.Flag{
		*EncryptionKeyFlag,
		*DatabaseURLFlag,
		*GithubUsernamesFlag,
		*OrganizationsRepositoriesLimitFlag,
	}
)

type Config struct {
	EncryptionKey                  string   `cfg:"encryption-key"`
	DatabaseURL                    string   `cfg:"database-url"`
	GithubUsernames                []string `cfg:"github-usernames"`
	OrganizationsRepositoriesLimit int      `cfg:"organizations-repositories-limit"`
}

func NewConfig(c *cli.Context) *Config {
	return &Config{
		DatabaseURL:                    c.String("database-url"),
		EncryptionKey:                  c.String("encryption-key"),
		GithubUsernames:                c.StringSlice("github-usernames"),
		OrganizationsRepositoriesLimit: c.Int("organizations-repositories-limit"),
	}
}
