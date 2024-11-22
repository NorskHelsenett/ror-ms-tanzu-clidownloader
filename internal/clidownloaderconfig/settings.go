package clidownloaderconfig

import (
	"github.com/NorskHelsenett/ror/pkg/config/configconsts"
	"github.com/NorskHelsenett/ror/pkg/config/rorversion"

	"github.com/spf13/viper"
)

var (
	Version       string = "1.1.0"
	Commit        string = "dev"
	DatacenterUrl string
)

func Load() {
	viper.AutomaticEnv()
	viper.SetDefault(configconsts.VERSION, Version)
	viper.SetDefault(configconsts.COMMIT, Commit)
	viper.SetDefault(configconsts.TANZU_AGENT_DATACENTER_URL, "")

	DatacenterUrl = viper.GetString(configconsts.TANZU_AGENT_DATACENTER_URL)

	if DatacenterUrl == "" {
		panic("Datacenter URL is not set")
	}

}

func GetRorVersion() rorversion.RorVersion {
	return rorversion.NewRorVersion(viper.GetString(configconsts.VERSION), viper.GetString(configconsts.COMMIT))
}

func GetDatacenterUrl() string {
	return DatacenterUrl
}
