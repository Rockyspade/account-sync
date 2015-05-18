package accountsync

import (
	"fmt"

	"github.com/codegangsta/cli"
)

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
		Name:   "organizations-repositories-limit",
		Value:  1000,
		EnvVar: "TRAVIS_ACCOUNT_SYNC_ORGANIZATIONS_REPOSITORIES_LIMIT",
	}
	RepositoriesStartPageFlag = &cli.IntFlag{
		Name:   "repositories-start-page",
		Value:  1,
		EnvVar: "TRAVIS_ACCOUNT_SYNC_REPOSITORIES_START_PAGE",
	}
	SyncTypesFlag = &cli.StringSliceFlag{
		Name:   "T, sync-types",
		Value:  &cli.StringSlice{"public"},
		EnvVar: "TRAVIS_ACCOUNT_SYNC_TYPES",
	}
	SyncCacheSizeFlag = &cli.IntFlag{
		Name:   "sync-cache-size",
		Value:  64,
		EnvVar: "TRAVIS_ACCOUNT_SYNC_CACHE_SIZE",
	}

	Flags = []cli.Flag{
		*EncryptionKeyFlag,
		*DatabaseURLFlag,
		*GithubUsernamesFlag,
		*OrganizationsRepositoriesLimitFlag,
		*RepositoriesStartPageFlag,
		*SyncTypesFlag,
		*SyncCacheSizeFlag,
	}

	errPrivateSyncNotSupported = fmt.Errorf("private sync is not supported (yet)!")
)

type Config struct {
	EncryptionKey                  string   `cfg:"encryption-key"` // TODO: do something with these tags
	DatabaseURL                    string   `cfg:"database-url"`
	GithubUsernames                []string `cfg:"github-usernames"`
	OrganizationsRepositoriesLimit int      `cfg:"organizations-repositories-limit"`
	RepositoriesStartPage          int      `cfg:"repositories-start-page"`
	SyncTypes                      []string `cfg:"sync-types"`
	SyncCacheSize                  int      `cfg:"sync-cache-size"`
}

func NewConfig(c *cli.Context) *Config {
	return &Config{
		DatabaseURL:                    c.String("database-url"),
		EncryptionKey:                  c.String("encryption-key"),
		GithubUsernames:                c.StringSlice("github-usernames"),
		OrganizationsRepositoriesLimit: c.Int("organizations-repositories-limit"),
		RepositoriesStartPage:          c.Int("repositories-start-page"),
		SyncTypes:                      c.StringSlice("sync-types"),
		SyncCacheSize:                  c.Int("sync-cache-size"),
	}
}

func (cfg *Config) Validate() error {
	if sliceContains(cfg.SyncTypes, "private") {
		return errPrivateSyncNotSupported
	}
	return nil
}
