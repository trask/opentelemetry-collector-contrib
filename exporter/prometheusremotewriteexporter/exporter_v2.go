// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"sync"

	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"
)

func (prwe *prwExporter) pushMetricsV2(ctx context.Context, md pmetric.Metrics) error {
	tsMap, symbolsTable, err := prometheusremotewrite.FromMetricsV2(md, prwe.exporterSettings)

	prwe.telemetry.recordTranslatedTimeSeries(ctx, len(tsMap))

	if err != nil {
		prwe.telemetry.recordTranslationFailure(ctx)
		prwe.settings.Logger.Debug("failed to translate metrics, exporting remaining metrics", zap.Error(err), zap.Int("translated", len(tsMap)))
	}
	// Call export even if a conversion error, since there may be points that were successfully converted.
	return prwe.handleExportV2(ctx, symbolsTable, tsMap)
}

// exportV2 sends a Snappy-compressed writev2.Request containing writev2.TimeSeries to a remote write endpoint.
func (prwe *prwExporter) exportV2(ctx context.Context, requests []*writev2.Request) error {
	input := make(chan *writev2.Request, len(requests))
	for _, request := range requests {
		input <- request
	}
	close(input)

	var wg sync.WaitGroup

	concurrencyLimit := int(math.Min(float64(prwe.concurrency), float64(len(requests))))
	wg.Add(concurrencyLimit) // used to wait for workers to be finished

	var mu sync.Mutex
	var errs error
	// Run concurrencyLimit of workers until there
	// is no more requests to execute in the input channel.
	for i := 0; i < concurrencyLimit; i++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done(): // Check firstly to ensure that the context wasn't cancelled.
					return

				case request, ok := <-input:
					if !ok {
						return
					}

					buf := bufferPool.Get().(*buffer)
					buf.protobuf.Reset()

					errMarshal := buf.protobuf.Marshal(request)
					if errMarshal != nil {
						mu.Lock()
						errs = multierr.Append(errs, consumererror.NewPermanent(errMarshal))
						mu.Unlock()
						bufferPool.Put(buf)
						return
					}

					if errExecute := prwe.execute(ctx, buf); errExecute != nil {
						mu.Lock()
						errs = multierr.Append(errs, consumererror.NewPermanent(errExecute))
						mu.Unlock()
					}
					bufferPool.Put(buf)
				}
			}
		}()
	}
	wg.Wait()

	return errs
}

func (prwe *prwExporter) handleExportV2(ctx context.Context, symbolsTable writev2.SymbolsTable, tsMap map[string]*writev2.TimeSeries) error {
	// There are no metrics to export, so return.
	if len(tsMap) == 0 {
		return nil
	}

	state := prwe.batchStatePool.Get().(*batchTimeSeriesState)
	defer prwe.batchStatePool.Put(state)
	requests, err := batchTimeSeriesV2(tsMap, symbolsTable, prwe.maxBatchSizeBytes, state)
	if err != nil {
		return err
	}

	// TODO implement WAl support, can be done after #15277 is fixed

	return prwe.exportV2(ctx, requests)
}

func (prwe *prwExporter) handleHeader(ctx context.Context, resp *http.Response, headerName string, metricType string, recordFunc func(context.Context, int64)) {
	headerValue := resp.Header.Get(headerName)
	if headerValue == "" {
		prwe.settings.Logger.Warn(
			headerName+" header is missing from the response, suggesting that the endpoint doesn't support RW2 and might be silently dropping data.",
			zap.String("url", resp.Request.URL.String()),
		)
		return
	}

	value, err := strconv.ParseInt(headerValue, 10, 64)
	if err != nil {
		prwe.settings.Logger.Warn(
			"Failed to convert "+headerName+" header to int64, not counting "+metricType+" written",
			zap.String("url", resp.Request.URL.String()),
		)
		return
	}
	recordFunc(ctx, value)
}

func (prwe *prwExporter) handleWrittenHeaders(ctx context.Context, resp *http.Response) {
	prwe.handleHeader(ctx, resp,
		"X-Prometheus-Remote-Write-Samples-Written",
		"samples",
		prwe.telemetry.recordWrittenSamples)

	prwe.handleHeader(ctx, resp,
		"X-Prometheus-Remote-Write-Histograms-Written",
		"histograms",
		prwe.telemetry.recordWrittenHistograms)

	prwe.handleHeader(ctx, resp,
		"X-Prometheus-Remote-Write-Exemplars-Written",
		"exemplars",
		prwe.telemetry.recordWrittenExemplars)
}
