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
	app.Flags = accountsync.Flags
	app.Action = func(c *cli.Context) {
		accountsync.NewSyncer(accountsync.NewConfig(c)).Sync()
	}
	app.Run(os.Args)
}
