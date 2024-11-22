package clidownloaderconfig

import (
	"github.com/NorskHelsenett/ror-ms-tanzu-clidownloader/pkg/clidownloader"
	"github.com/NorskHelsenett/ror/pkg/config/configconsts"
	"github.com/NorskHelsenett/ror/pkg/config/rorversion"

	"github.com/spf13/viper"
)

var (
	Version            string = "1.1.0"
	Commit             string = "dev"
	TanzuDatacenterUrl string
	TanzuAppPath       string
	TanzuUrlPath       string
	DownloaderConfig   clidownloader.TanzuCliDownloaderConfig
)

func Load() {
	viper.AutomaticEnv()
	viper.SetDefault(configconsts.VERSION, Version)
	viper.SetDefault(configconsts.COMMIT, Commit)
	viper.SetDefault(configconsts.TANZU_AGENT_DATACENTER_URL, "")
	viper.SetDefault("TANZU_URL_PATH", "wcp/plugin/linux-amd64/vsphere-plugin.zip")
	viper.SetDefault("TANZU_APP_PATH", "/app")

	DownloaderConfig = clidownloader.TanzuCliDownloaderConfig{
		DatacenterUrl: viper.GetString(configconsts.TANZU_AGENT_DATACENTER_URL),
		UrlPath:       viper.GetString("TANZU_URL_PATH"),
		AppPath:       viper.GetString("TANZU_APP_PATH"),
	}

}

func GetRorVersion() rorversion.RorVersion {
	return rorversion.NewRorVersion(viper.GetString(configconsts.VERSION), viper.GetString(configconsts.COMMIT))
}

func GetDownloaderConfig() *clidownloader.TanzuCliDownloaderConfig {
	return &DownloaderConfig
}
