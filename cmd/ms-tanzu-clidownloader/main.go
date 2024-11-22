package main

import (
	"github.com/NorskHelsenett/ror-ms-tanzu-clidownloader/internal/clidownloaderconfig"
	"github.com/NorskHelsenett/ror-ms-tanzu-clidownloader/pkg/clidownloader"
	"github.com/NorskHelsenett/ror/pkg/rlog"
)

func main() {

	clidownloaderconfig.Load()
	rlog.Info("Tanzu Cli Downloader is starting", rlog.String("version", clidownloaderconfig.GetRorVersion().GetVersionWithCommit()))
	cliversions, err := clidownloader.DownloadCli(clidownloaderconfig.GetDatacenterUrl())
	if err != nil {
		panic(err)
	}
	rlog.Info("kubectl downloaded", rlog.String("version", cliversions.KubectlVersion))
	rlog.Info("kubectl-vsphere downloaded", rlog.String("version", cliversions.KubectlVsphereVersion))
}
