package queuecontroller

import (
	"context"
	"flag"
	"fmt"

	"k8s.io/client-go/rest"

	"github.com/NVIDIA/KAI-scheduler/cmd/queuecontroller/app"
)

func RunQueueController(cfg *rest.Config, ctx context.Context) error {
	opts := app.InitOptions()

	opts.EnableLeaderElection = false
	opts.MetricsAddress = ":8084"

	flag.Parse()

	go func() {
		err := app.Run(opts, cfg, ctx)
		if err != nil {
			panic(fmt.Errorf("failed to run queuecontroller app: %w", err))
		}
	}()

	return nil
}
