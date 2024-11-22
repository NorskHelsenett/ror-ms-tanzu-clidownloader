package main

import (
	"github.com/NorskHelsenett/ror-ms-tanzu-clidownloader/internal/clidownloaderconfig"
	"github.com/NorskHelsenett/ror-ms-tanzu-clidownloader/pkg/clidownloader"
	"github.com/NorskHelsenett/ror/pkg/rlog"
)

func main() {

	clidownloaderconfig.Load()
	rlog.Info("Tanzu Cli Downloader is starting", rlog.String("version", clidownloaderconfig.GetRorVersion().GetVersionWithCommit()))
	err := clidownloader.DownloadCli(clidownloaderconfig.DatacenterUrl)
	if err != nil {
		panic(err)
	}
}
