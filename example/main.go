package main

import (
	"context"
	"time"

	"alpineworks.io/ootel"
)

func main() {
	ctx := context.Background()

	ootelClient := ootel.NewOotelClient(
		ootel.WithMetricConfig(ootel.NewMetricConfig(true, 8081)),
		ootel.WithTraceConfig(ootel.NewTraceConfig(true, 1.0, "example-service", "1.0.0")),
	)

	shutdown, err := ootelClient.Init(ctx)
	if err != nil {
		panic(err)
	}

	defer func() {
		_ = shutdown(ctx)
	}()

	<-time.After(time.Minute)
}
