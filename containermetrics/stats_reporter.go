package containermetrics

import (
	"os"
	"time"

	"github.com/cloudfoundry-incubator/executor"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

type StatsReporter struct {
	logger lager.Logger

	interval       time.Duration
	clock          clock.Clock
	executorClient executor.Client

	cpuInfos map[string]cpuInfo
}

type cpuInfo struct {
	timeSpentInCPU time.Duration
	timeOfSample   time.Time
}

func NewStatsReporter(logger lager.Logger, interval time.Duration, clock clock.Clock, executorClient executor.Client) *StatsReporter {
	return &StatsReporter{
		logger: logger,

		interval:       interval,
		clock:          clock,
		executorClient: executorClient,
	}
}

func (reporter *StatsReporter) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	ticker := reporter.clock.NewTicker(reporter.interval)
	defer ticker.Stop()

	cpuInfos := make(map[string]*cpuInfo)
	for {
		select {
		case <-signals:
			return nil

		case <-ticker.C():
			cpuInfos = reporter.emitContainerMetrics(reporter.logger.Session("tick"), cpuInfos)
		}
	}

	return nil
}

func (reporter *StatsReporter) emitContainerMetrics(logger lager.Logger, previousCpuInfos map[string]*cpuInfo) map[string]*cpuInfo {
	startTime := reporter.clock.Now()

	logger.Info("started")
	defer func() {
		logger.Info("done", lager.Data{
			"took": reporter.clock.Now().Sub(startTime).String(),
		})
	}()

	containers, err := reporter.executorClient.ListContainers(nil)
	if err != nil {
		logger.Error("failed-to-list-containers", err)
		return previousCpuInfos
	}

	logger.Info("emitting", lager.Data{
		"total-containers":        len(containers),
		"listing-containers-took": reporter.clock.Now().Sub(startTime).String(),
	})

	containerGuids := make([]string, 0, len(containers))
	for _, container := range containers {
		if container.MetricsConfig.Guid != "" {
			containerGuids = append(containerGuids, container.Guid)
		}
	}

	metrics, err := reporter.executorClient.GetMetrics(containerGuids)
	if err != nil {
		logger.Error("failed-to-get-metrics", err)
		return previousCpuInfos
	}

	newCpuInfos := make(map[string]*cpuInfo)
	now := reporter.clock.Now()
	for _, container := range containers {
		if containerMetrics, found := metrics[container.Guid]; found {
			previousCpuInfo := previousCpuInfos[container.Guid]
			cpu := reporter.calculateAndSendMetrics(logger, &container.MetricsConfig, &containerMetrics, previousCpuInfo, now)
			if cpu != nil {
				newCpuInfos[container.Guid] = cpu
			}
		}
	}

	return newCpuInfos
}

func (reporter *StatsReporter) calculateAndSendMetrics(
	logger lager.Logger,
	metricsConfig *executor.MetricsConfig,
	containerMetrics *executor.ContainerMetrics,
	previousInfo *cpuInfo,
	now time.Time,
) *cpuInfo {
	if metricsConfig.Guid == "" {
		return nil
	}

	currentInfo := cpuInfo{
		timeSpentInCPU: containerMetrics.TimeSpentInCPU,
		timeOfSample:   now,
	}

	var cpuPercent float64
	if previousInfo == nil {
		cpuPercent = 0.0
	} else {
		cpuPercent = computeCPUPercent(
			previousInfo.timeSpentInCPU,
			currentInfo.timeSpentInCPU,
			previousInfo.timeOfSample,
			currentInfo.timeOfSample,
		)
	}

	err := metrics.SendContainerMetric(metricsConfig.Guid, int32(metricsConfig.Index), cpuPercent, containerMetrics.MemoryUsageInBytes, containerMetrics.DiskUsageInBytes)
	if err != nil {
		logger.Error("failed-to-send-container-metrics", err, lager.Data{
			"metrics_guid":  metricsConfig.Guid,
			"metrics_index": metricsConfig.Index,
		})
	}

	return &currentInfo
}

// scale from 0 - 100
func computeCPUPercent(timeSpentA, timeSpentB time.Duration, sampleTimeA, sampleTimeB time.Time) float64 {
	// divide change in time spent in CPU over time between samples.
	// result is out of 100.
	//
	// don't worry about overflowing int64. it's like, 30 years.
	return float64((timeSpentB-timeSpentA)*100) / float64(sampleTimeB.UnixNano()-sampleTimeA.UnixNano())
}
