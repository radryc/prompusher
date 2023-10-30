package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"

	"github.com/radryc/prompusher/logger"
	"github.com/radryc/prompusher/metrics"
	"github.com/radryc/prompusher/webserver"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	httpPort int
)

func init() {
	flag.IntVar(&httpPort, "port", 8080, "port number to listen on")
	flag.Parse()
}

func main() {
	metricStore := metrics.NewMetricStore()
	fx.New(
		fx.Provide(http.NewServeMux, func() *metrics.MetricStore { return metricStore }),
		fx.Invoke(webserver.New),
		fx.Invoke(registerHooks),
		logger.Module,
	).Run()
}

func registerHooks(
	lifecycle fx.Lifecycle, mux *http.ServeMux, logger *zap.SugaredLogger, metricStore *metrics.MetricStore,
) {
	lifecycle.Append(
		fx.Hook{
			OnStart: func(context.Context) error {
				go http.ListenAndServe(fmt.Sprintf(":%d", httpPort), mux)
				metricStore.StartCron()
				return nil
			},
			OnStop: func(context.Context) error {
				metricStore.StopCron()
				return logger.Sync()
			},
		},
	)
}
