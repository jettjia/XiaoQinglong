package cmd

import (
	"context"
	"fmt"

	"github.com/jettjia/XiaoQinglong/runner/cli/client"
	"github.com/jettjia/XiaoQinglong/runner/cli/config"
)

// Stop 停止运行
func Stop(checkpointID string) error {
	if checkpointID == "" {
		return fmt.Errorf("checkpoint_id is required")
	}

	endpoint := config.GetEndpoint()
	runner := client.NewHTTPRunner(endpoint)

	ctx := context.Background()
	if err := runner.Stop(ctx, checkpointID); err != nil {
		return err
	}

	fmt.Printf("Stopped: %s\n", checkpointID)
	return nil
}
