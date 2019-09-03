package library

import "github.com/BurntSushi/toml"

type TomlConfig struct {
	Title string
	Server serverInfo
	Fabric fabricInfo
	Static staticInfo
	Views viewsInfo
	Logs logsInfo
}

type serverInfo struct {
	ServerName string
	Host string
	Port int
}

type fabricInfo struct {
	FabFile string
}

type staticInfo struct {
	StaticPath string
}

type viewsInfo struct {
	ViewPath string
}

type logsInfo struct {
	LogPath string
}

func ReadConf (config_file string) (*TomlConfig,error){
	var (
		conf  *TomlConfig
		err error
	)
	_, err = toml.DecodeFile(config_file, &conf)
	return conf, err
}