package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/travis-ci/account-sync"
)

func main() {
	app := cli.NewApp()
	app.Usage = "Syncing accounts"
	app.Version = accountsync.VersionString
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "k, encryption-key",
			Value:  "",
			EnvVar: "TRAVIS_ACCOUNT_SYNC_ENCRYPTION_KEY",
		},
		cli.StringFlag{
			Name:   "d, database-url",
			Value:  "",
			EnvVar: "TRAVIS_ACCOUNT_SYNC_DATABASE_URL",
		},
		cli.StringSliceFlag{
			Name:   "u, github-usernames",
			Value:  &cli.StringSlice{},
			EnvVar: "TRAVIS_ACCOUNT_SYNC_GITHUB_USERNAMES",
		},
	}
	app.Action = func(c *cli.Context) {
		syncer := accountsync.NewSyncer(&accountsync.Config{
			DatabaseURL:     c.String("database-url"),
			EncryptionKey:   c.String("encryption-key"),
			GithubUsernames: c.StringSlice("github-usernames"),
		})
		syncer.Sync()
	}
	app.Run(os.Args)
}
