package main

// CREDIT: from https://github.com/filecoin-project/lassie/blob/main/cmd/lassie/main.go
// The MIT License (MIT)

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
import (
	"context"
	"os"

	"github.com/filecoin-project/lassie/pkg/aggregateeventrecorder"
	"github.com/filecoin-project/lassie/pkg/lassie"
	"github.com/google/uuid"
	"github.com/ipfs/go-log"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func before(cctx *cli.Context) error {
	// Determine logging level
	subsystems := []string{
		"lassie",
		"lassie/httpserver",
		"indexerlookup",
		"lassie/bitswap",
	}

	level := "WARN"
	if IsVerbose {
		level = "INFO"
	}
	if IsVeryVerbose {
		level = "DEBUG"
	}

	// don't over-ride logging if set in the environment.
	if os.Getenv("GOLOG_LOG_LEVEL") == "" {
		for _, name := range subsystems {
			_ = log.SetLogLevel(name, level)
		}
	}

	return nil
}

// setupLassieEventRecorder creates and subscribes an EventRecorder if an event recorder URL is given
func setupLassieEventRecorder(
	ctx context.Context,
	eventRecorderURL string,
	authToken string,
	instanceID string,
	lassie *lassie.Lassie,
) {
	if eventRecorderURL != "" {
		if instanceID == "" {
			uuid, err := uuid.NewRandom()
			if err != nil {
				klog.Warning("failed to generate default event recorder instance ID UUID, no instance ID will be provided", "err", err)
			}
			instanceID = uuid.String() // returns "" if uuid is invalid
		}

		eventRecorder := aggregateeventrecorder.NewAggregateEventRecorder(ctx, aggregateeventrecorder.EventRecorderConfig{
			InstanceID:            instanceID,
			EndpointURL:           eventRecorderURL,
			EndpointAuthorization: authToken,
		})
		lassie.RegisterSubscriber(eventRecorder.RetrievalEventSubscriber())
		klog.Warningln("Reporting retrieval events to event recorder API", "url", eventRecorderURL, "instance_id", instanceID)
	}
}
