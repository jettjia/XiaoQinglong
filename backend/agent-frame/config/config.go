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

// InitConfig read config
func initConfig() *conf.Config {
	// 默认配置路径
	defaultFile := "./manifest/config/config.yaml"

	// 用户配置目录（优先使用）
	userConfigDir := getConfigDir()
	userConfigFile := filepath.Join(userConfigDir, "config.yaml")

	// product: load k8s config path
	if os.Getenv("env") == "release" {
		defaultFile = "/sysvol/conf/public-center.yaml"
	}

	// product: load docker config path
	if os.Getenv("env") == "docker" {
		defaultFile = "./manifest/config/config-docker.yaml"
	}

	// 确定使用的配置文件
	file := defaultFile
	if _, err := os.Stat(userConfigFile); err == nil {
		// 用户配置存在，优先使用
		file = userConfigFile
	}

	// load config.yaml
	cfg = &conf.Config{}
	if err := flag.Set("conf", file); err != nil {
		panic(err)
	}
	if err := conf.ParseYaml(cfg); err != nil {
		panic(err)
	}

	if err := conf.ParseYaml(cfg); err != nil {
		panic(err)
	}

	return cfg
}
