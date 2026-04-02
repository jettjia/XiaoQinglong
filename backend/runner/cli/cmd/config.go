package cmd

import (
	"fmt"

	"github.com/jettjia/XiaoQinglong/runner/cli/config"
)

// ConfigShow 显示当前配置
func ConfigShow() error {
	req, err := config.LoadConfig()
	if err != nil {
		return err
	}

	fmt.Print(config.ShowConfig(req))
	return nil
}
