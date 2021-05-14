package pkg

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
)

type triggerWorkflow struct {
	triggerStruct
	log logr.Logger
	git gitCommand
}

func (w *triggerWorkflow) start(ctx context.Context, action <-chan string, version chan<- string) error {
	for {
		select {
		case action := <-action:
			msg := fmt.Sprintf("trigger %s started...", action)
			w.log.Info(msg)
			version <- triggeredMessage
		case <-ctx.Done():
			return nil
		}
	}
}
