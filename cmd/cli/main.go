package main

import (
	"io"
	"log"
	"net/http"
	"os"

	"github.com/ruteri/dummy-tdx-dcap/common"
	"github.com/urfave/cli/v2" // imports as package "cli"
)

var flags []cli.Flag = []cli.Flag{
	&cli.StringFlag{
		Name:  "url",
		Value: "http://127.0.0.1:8080",
		Usage: "dummy attestation service url to request",
	},
	&cli.StringFlag{
		Name:  "appdata",
		Value: "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		Usage: "appdata (user data) to submit",
	},
	&cli.BoolFlag{
		Name:  "log-json",
		Value: false,
		Usage: "log in JSON format",
	},
	&cli.BoolFlag{
		Name:  "log-debug",
		Value: false,
		Usage: "log debug messages",
	},
}

func main() {
	app := &cli.App{
		Name:   "dummy attestation cli",
		Usage:  "Request a dummy attestation",
		Flags:  flags,
		Action: runCli,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func runCli(cCtx *cli.Context) error {
	logJSON := cCtx.Bool("log-json")
	logDebug := cCtx.Bool("log-debug")

	data := cCtx.String("appdata")
	url := cCtx.String("url")

	log := common.SetupLogger(&common.LoggingOpts{
		Debug:   logDebug,
		JSON:    logJSON,
		Version: common.Version,
	})

	resp, err := http.Get(url + "/attestation/" + data)
	if err != nil {
		log.Error("could not request the dummy quote", "err", err)
		return err
	}

	quote, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("could not get the dummy quote from response", "err", err)
		return err
	}

	log.Info("Success", "quote", quote)
	return nil
}
