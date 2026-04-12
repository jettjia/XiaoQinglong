package config

import (
	"flag"
	"os"
	"path/filepath"
	"sync"

	"github.com/jettjia/igo-pkg/pkg/conf"
)

var (
	configOnce sync.Once
	cfg        *conf.Config
)

// getHomeDir 获取用户 home 目录
func getHomeDir() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/tmp"
	}
	return home
}

// getConfigDir 获取配置目录
func getConfigDir() string {
	return filepath.Join(getHomeDir(), ".xiaoqinglong", "config")
}

func NewConfig() (conf *conf.Config) {
	configOnce.Do(func() {
		cfg = initConfig()
	})

	return cfg
}

// initConfig read config
func initConfig() *conf.Config {
	// 开发环境使用 dev-config.yaml
	if os.Getenv("env") == "debug" {
		devConfig := "agent-frame/manifest/config/dev-config.yaml"
		if _, err := os.Stat(devConfig); err == nil {
			cfg = &conf.Config{}
			if err := flag.Set("conf", devConfig); err != nil {
				panic(err)
			}
			if err := conf.ParseYaml(cfg); err != nil {
				panic(err)
			}
			return cfg
		}
	}

	// 生产环境：优先使用 XQL_CONFIG_PATH 环境变量
	if xqlConfigPath := os.Getenv("XQL_CONFIG_PATH"); xqlConfigPath != "" {
		if _, err := os.Stat(xqlConfigPath); err == nil {
			cfg = &conf.Config{}
			if err := flag.Set("conf", xqlConfigPath); err != nil {
				panic(err)
			}
			if err := conf.ParseYaml(cfg); err != nil {
				panic(err)
			}
			return cfg
		}
	}

	// 生产环境：使用 ~/.xiaoqinglong/config/config.yaml
	userConfigFile := filepath.Join(getConfigDir(), "config.yaml")
	if _, err := os.Stat(userConfigFile); err == nil {
		cfg = &conf.Config{}
		if err := flag.Set("conf", userConfigFile); err != nil {
			panic(err)
		}
		if err := conf.ParseYaml(cfg); err != nil {
			panic(err)
		}
		return cfg
	}

	// 异常：配置不存在
	panic("config file not found: " + userConfigFile)
}
