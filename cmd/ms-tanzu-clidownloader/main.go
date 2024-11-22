package main

import (
	"github.com/NorskHelsenett/ror-ms-tanzu-clidownloader/pkg/clidownloader"
	"github.com/NorskHelsenett/ror/pkg/config/configconsts"
	"github.com/spf13/viper"
)

func main() {

	viper.AutomaticEnv()
	datacenterUrl := viper.GetString(configconsts.TANZU_AGENT_DATACENTER_URL)

	if datacenterUrl == "" {
		panic("Datacenter URL is not set")
	}

	err := clidownloader.DownloadCli(datacenterUrl)
	if err != nil {
		panic(err)
	}
}
