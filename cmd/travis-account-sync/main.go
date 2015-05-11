package main

import (
	"log"
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
		cfg := accountsync.NewConfig(c)
		err := cfg.Validate()
		if err != nil {
			log.Fatalf("err=%q", err.Error())
		}
		accountsync.NewSyncer().Sync()
	}
	app.Run(os.Args)
}
