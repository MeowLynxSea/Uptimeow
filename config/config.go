package config

import (
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"sync"
)

type ConfigData struct {
	Web struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"web"`
	Rcon struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Password string `yaml:"password"`
	} `yaml:"rcon"`
	ServerInfo struct {
		Name        string `yaml:"name"`
		Address     string `yaml:"address"`
		Website     string `yaml:"website"`
		Description string `yaml:"description"`
	} `yaml:"server_info"`
	Warn struct {
		Enabled     bool `yaml:"enabled"`
		DingTalkBot struct {
			Enabled     bool   `yaml:"enabled"`
			AccessToken string `yaml:"accessToken"`
			Secret      string `yaml:"secret"`
			AtMobile    string `yaml:"atMobile"`
		} `yaml:"dingtalkBot"`
		EnabledType struct {
			LowTps struct {
				Enabled bool    `yaml:"enabled"`
				Threold float64 `yaml:"threshold"`
			} `yaml:"lowTps"`
			Offline bool `yaml:"offline"`
		} `yaml:"enabledType"`
	}
}

var config ConfigData
var once sync.Once

func Load() ConfigData {
	once.Do(func() {
		// 读取YAML文件
		data, err := os.ReadFile("config.yml")
		if err != nil {
			log.Fatalln("Error reading YAML file:", err)
			return
		}

		// 解析YAML数据到config结构体
		err = yaml.Unmarshal(data, &config)
		if err != nil {
			log.Fatalln("Error parsing YAML data:", err)
			return
		}

		// 设置缺省值
		if config.Web.Port == 0 {
			log.Println("Port not defined in config, using 80 as default...")
			config.Web.Port = 80 // 默认端口
		}
		if config.Web.Host == "" {
			log.Println("Host not defined in config, using 0.0.0.0 as default...")
			config.Web.Host = "0.0.0.0" // 默认主机
		}
	})

	return config
}
