package main

import (
	"bytes"
	"encoding/hex"
	"errors"
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
		Name:  "quote",
		Value: "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		Usage: "hexencoded quote to verify",
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
		Name:   "dummy quote verification cli",
		Usage:  "Verify a quote",
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

	quoteHex := cCtx.String("quote")
	url := cCtx.String("url")

	log := common.SetupLogger(&common.LoggingOpts{
		Debug:   logDebug,
		JSON:    logJSON,
		Version: common.Version,
	})

	rawQuote, err := hex.DecodeString(quoteHex)
	if err != nil {
		log.Error("could not parse quote", "err", err)
		return err
	}

	resp, err := http.Post(url+"/verify", "application/octet-stream", bytes.NewReader(rawQuote))
	if err != nil {
		log.Error("could not request the dummy service", "err", err)
		return err
	}
	if resp == nil {
		log.Error("nil response")
		return errors.New("nil response")
	}

	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("could not get the quote from response", "err", err)
		return err
	}

	if resp.StatusCode == http.StatusOK {
		log.Info("Success", "quote", respData)
	} else {
		log.Info("Failure", "reason", respData)
	}
	return nil
}
