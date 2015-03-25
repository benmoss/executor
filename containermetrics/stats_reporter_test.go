package containermetrics_test

import (
	"errors"
	"time"

	"github.com/cloudfoundry-incubator/executor"
	"github.com/cloudfoundry-incubator/executor/containermetrics"
	efakes "github.com/cloudfoundry-incubator/executor/fakes"
	msfake "github.com/cloudfoundry/dropsonde/metric_sender/fake"
	dmetrics "github.com/cloudfoundry/dropsonde/metrics"
	"github.com/pivotal-golang/clock/fakeclock"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type listContainerResults struct {
	containers []executor.Container
	err        error
}

type metricsResults struct {
	metrics executor.ContainerMetrics
	err     error
}

var _ = Describe("StatsReporter", func() {
	var (
		logger *lagertest.TestLogger

		interval           time.Duration
		fakeClock          *fakeclock.FakeClock
		fakeExecutorClient *efakes.FakeClient
		fakeMetricSender   *msfake.FakeMetricSender

		containerResults chan listContainerResults
		metricsResults   chan map[string]executor.ContainerMetrics
		process          ifrit.Process
	)

	sendContainerResults := func() {
		containerResults <- listContainerResults{
			containers: []executor.Container{
				{
					Guid: "guid-without-index",
					MetricsConfig: executor.MetricsConfig{
						Guid: "metrics-guid-without-index",
					},
				},
				{
					Guid: "guid-with-no-metrics-guid",
				},
				{
					Guid: "guid-with-index",
					MetricsConfig: executor.MetricsConfig{
						Guid:  "metrics-guid-with-index",
						Index: 1,
					},
				},
			},
			err: nil,
		}

		metricsResults <- map[string]executor.ContainerMetrics{
			"guid-without-index": executor.ContainerMetrics{
				MemoryUsageInBytes: 123,
				DiskUsageInBytes:   456,
				TimeSpentInCPU:     100 * time.Second,
			},
			"guid-with-no-metrics-guid": executor.ContainerMetrics{
				MemoryUsageInBytes: 1023,
				DiskUsageInBytes:   4056,
				TimeSpentInCPU:     1000 * time.Second,
			},
			"guid-with-index": executor.ContainerMetrics{
				MemoryUsageInBytes: 321,
				DiskUsageInBytes:   654,
				TimeSpentInCPU:     100 * time.Second,
			},
		}

		containerResults <- listContainerResults{
			containers: []executor.Container{
				{
					Guid: "guid-without-index",

					MetricsConfig: executor.MetricsConfig{
						Guid: "metrics-guid-without-index",
					},
				},
				{
					Guid: "guid-with-index",

					MetricsConfig: executor.MetricsConfig{
						Guid:  "metrics-guid-with-index",
						Index: 1,
					},
				},
			},
			err: nil,
		}

		metricsResults <- map[string]executor.ContainerMetrics{
			"guid-without-index": executor.ContainerMetrics{
				MemoryUsageInBytes: 1230,
				DiskUsageInBytes:   4560,
				TimeSpentInCPU:     105 * time.Second,
			},
			"guid-with-index": executor.ContainerMetrics{
				MemoryUsageInBytes: 3210,
				DiskUsageInBytes:   6540,
				TimeSpentInCPU:     110 * time.Second,
			},
		}

		containerResults <- listContainerResults{
			containers: []executor.Container{
				{
					Guid: "guid-without-index",

					MetricsConfig: executor.MetricsConfig{
						Guid: "metrics-guid-without-index",
					},
				},
				{
					Guid: "guid-with-index",

					MetricsConfig: executor.MetricsConfig{
						Guid:  "metrics-guid-with-index",
						Index: 1,
					},
				},
			},
			err: nil,
		}

		metricsResults <- map[string]executor.ContainerMetrics{
			"guid-without-index": executor.ContainerMetrics{
				MemoryUsageInBytes: 12300,
				DiskUsageInBytes:   45600,
				TimeSpentInCPU:     107 * time.Second,
			},
			"guid-with-index": executor.ContainerMetrics{
				MemoryUsageInBytes: 32100,
				DiskUsageInBytes:   65400,
				TimeSpentInCPU:     112 * time.Second,
			},
		}
	}

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		interval = 10 * time.Second
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))
		fakeExecutorClient = new(efakes.FakeClient)

		fakeMetricSender = msfake.NewFakeMetricSender()
		dmetrics.Initialize(fakeMetricSender)

		containerResults = make(chan listContainerResults, 10)
		metricsResults = make(chan map[string]executor.ContainerMetrics, 10)

		fakeExecutorClient.ListContainersStub = func(executor.Tags) ([]executor.Container, error) {
			result, closed := <-containerResults
			if closed {
				return []executor.Container{}, errors.New("closed")
			}
			return result.containers, result.err
		}

		fakeExecutorClient.GetMetricsStub = func(guid []string) (map[string]executor.ContainerMetrics, error) {
			result, closed := <-metricsResults
			if closed {
				return nil, errors.New("closed")
			}

			return result, nil
		}

		process = ifrit.Invoke(containermetrics.NewStatsReporter(logger, interval, fakeClock, fakeExecutorClient))
	})

	AfterEach(func() {
		close(containerResults)
		close(metricsResults)
		ginkgomon.Interrupt(process)
	})

	Context("when the interval elapses", func() {
		BeforeEach(func() {
			sendContainerResults()

			fakeClock.Increment(interval)
			Eventually(fakeExecutorClient.ListContainersCallCount).Should(Equal(1))
		})

		FIt("emits memory and disk usage for each container, but no CPU", func() {
			Eventually(func() msfake.ContainerMetric {
				return fakeMetricSender.GetContainerMetric("metrics-guid-without-index")
			}).Should(Equal(msfake.ContainerMetric{
				ApplicationId: "metrics-guid-without-index",
				InstanceIndex: 0,
				CpuPercentage: 0.0,
				MemoryBytes:   123,
				DiskBytes:     456,
			}))

			Eventually(func() msfake.ContainerMetric {
				return fakeMetricSender.GetContainerMetric("metrics-guid-with-index")
			}).Should(Equal(msfake.ContainerMetric{
				ApplicationId: "metrics-guid-with-index",
				InstanceIndex: 1,
				CpuPercentage: 0.0,
				MemoryBytes:   321,
				DiskBytes:     654,
			}))
		})

		It("does not emit anything for containers with no metrics guid", func() {
			Consistently(func() msfake.ContainerMetric {
				return fakeMetricSender.GetContainerMetric("")
			}).Should(BeZero())
		})

		Context("and the interval elapses again", func() {
			BeforeEach(func() {
				fakeClock.Increment(interval)
				Eventually(fakeExecutorClient.ListContainersCallCount).Should(Equal(2))
				Eventually(fakeExecutorClient.GetMetricsCallCount).Should(Equal(2))
			})

			It("emits the new memory and disk usage, and the computed CPU percent", func() {
				Eventually(func() msfake.ContainerMetric {
					return fakeMetricSender.GetContainerMetric("metrics-guid-without-index")
				}).Should(Equal(msfake.ContainerMetric{
					ApplicationId: "metrics-guid-without-index",
					InstanceIndex: 0,
					CpuPercentage: 50.0,
					MemoryBytes:   1230,
					DiskBytes:     4560,
				}))

				Eventually(func() msfake.ContainerMetric {
					return fakeMetricSender.GetContainerMetric("metrics-guid-with-index")
				}).Should(Equal(msfake.ContainerMetric{
					ApplicationId: "metrics-guid-with-index",
					InstanceIndex: 1,
					CpuPercentage: 100.0,
					MemoryBytes:   3210,
					DiskBytes:     6540,
				}))
			})

			Context("and the interval elapses again", func() {
				BeforeEach(func() {
					fakeClock.Increment(interval)
					Eventually(fakeExecutorClient.ListContainersCallCount).Should(Equal(3))
					Eventually(fakeExecutorClient.GetMetricsCallCount).Should(Equal(3))
				})

				It("emits the new memory and disk usage, and the computed CPU percent", func() {
					Eventually(func() msfake.ContainerMetric {
						return fakeMetricSender.GetContainerMetric("metrics-guid-without-index")
					}).Should(Equal(msfake.ContainerMetric{
						ApplicationId: "metrics-guid-without-index",
						InstanceIndex: 0,
						CpuPercentage: 20.0,
						MemoryBytes:   12300,
						DiskBytes:     45600,
					}))

					Eventually(func() msfake.ContainerMetric {
						return fakeMetricSender.GetContainerMetric("metrics-guid-with-index")
					}).Should(Equal(msfake.ContainerMetric{
						ApplicationId: "metrics-guid-with-index",
						InstanceIndex: 1,
						CpuPercentage: 20.0,
						MemoryBytes:   32100,
						DiskBytes:     65400,
					}))
				})
			})
		})
	})

	Context("when listing containers fails", func() {
		BeforeEach(func() {
			containerResults <- listContainerResults{containers: nil, err: errors.New("nope")}
			fakeClock.Increment(interval)
			Eventually(fakeExecutorClient.ListContainersCallCount).Should(Equal(1))
		})

		It("does not blow up", func() {
			Consistently(process.Wait()).ShouldNot(Receive())
		})

		Context("and the interval elapses again, and it works that time", func() {
			BeforeEach(func() {
				sendContainerResults()
				fakeClock.Increment(interval)
				Eventually(fakeExecutorClient.ListContainersCallCount).Should(Equal(2))
			})

			It("processes the containers happily", func() {
				Eventually(func() msfake.ContainerMetric {
					return fakeMetricSender.GetContainerMetric("metrics-guid-without-index")
				}).Should(Equal(msfake.ContainerMetric{
					ApplicationId: "metrics-guid-without-index",
					InstanceIndex: 0,
					CpuPercentage: 0.0,
					MemoryBytes:   123,
					DiskBytes:     456,
				}))

				Eventually(func() msfake.ContainerMetric {
					return fakeMetricSender.GetContainerMetric("metrics-guid-with-index")
				}).Should(Equal(msfake.ContainerMetric{
					ApplicationId: "metrics-guid-with-index",
					InstanceIndex: 1,
					CpuPercentage: 0.0,
					MemoryBytes:   321,
					DiskBytes:     654,
				}))
			})
		})
	})

	Context("when a container is no longer present", func() {
		var containers []executor.Container

		BeforeEach(func() {
			containers = []executor.Container{
				{
					Guid: "container-guid-1",
					MetricsConfig: executor.MetricsConfig{
						Guid: "metrics-guid-1",
					},
				},
				{
					Guid: "container-guid-2",
					MetricsConfig: executor.MetricsConfig{
						Guid: "metrics-guid-2",
					},
				},
			}
		})

		It("only remembers the previous metrics", func() {
			containerMetrics := map[string]executor.ContainerMetrics{
				"container-guid-1": executor.ContainerMetrics{TimeSpentInCPU: 1},
				"container-guid-2": executor.ContainerMetrics{TimeSpentInCPU: 2},
			}

			containerResults <- listContainerResults{
				containers: containers,
				err:        nil,
			}
			metricsResults <- containerMetrics

			fakeClock.Increment(interval)

			Eventually(func() msfake.ContainerMetric {
				return fakeMetricSender.GetContainerMetric("metrics-guid-with-index")
			}).Should(Equal(msfake.ContainerMetric{
				ApplicationId: "metrics-guid-with-index",
				InstanceIndex: 1,
				CpuPercentage: 0.0,
				MemoryBytes:   321,
				DiskBytes:     654,
			}))

			By("losing a container")
			containerResults <- listContainerResults{
				containers: containers,
				err:        nil,
			}
			metricsResults <- containerMetrics

			fakeClock.Increment(interval)

			Eventually(func() msfake.ContainerMetric {
				return fakeMetricSender.GetContainerMetric("metrics-guid-with-index")
			}).Should(Equal(msfake.ContainerMetric{
				ApplicationId: "metrics-guid-with-index",
				InstanceIndex: 1,
				CpuPercentage: 0.0,
				MemoryBytes:   321,
				DiskBytes:     654,
			}))

			By("finding the container again")
			containerResults <- listContainerResults{
				containers: containers,
				err:        nil,
			}
			metricsResults <- containerMetrics

			fakeClock.Increment(interval)

			Eventually(func() msfake.ContainerMetric {
				return fakeMetricSender.GetContainerMetric("metrics-guid-with-index")
			}).Should(Equal(msfake.ContainerMetric{
				ApplicationId: "metrics-guid-with-index",
				InstanceIndex: 1,
				CpuPercentage: 0.0,
				MemoryBytes:   321,
				DiskBytes:     654,
			}))

		})
	})

	Context("when getting metrics for multiple containers", func() {
		var containers []executor.Container

		BeforeEach(func() {
			containers = []executor.Container{
				{
					Guid: "guid-1",
					MetricsConfig: executor.MetricsConfig{
						Guid: "metrics-guid-1",
					},
				},
				{
					Guid: "guid-2",
					MetricsConfig: executor.MetricsConfig{
						Guid: "metrics-guid-2",
					},
				},
				{
					Guid: "guid-3",
					MetricsConfig: executor.MetricsConfig{
						Guid: "metrics-guid-3",
					},
				},
			}

			fakeExecutorClient.ListContainersReturns(containers, nil)

			fakeClock.Increment(interval)
		})

		It("retrieves the metrics for all", func() {
			Eventually(fakeExecutorClient.GetMetricsCallCount).Should(Equal(1))
			Ω(fakeExecutorClient.GetMetricsArgsForCall(0)).Should(ConsistOf("guid-1", "guid-2", "guid-3"))
		})
	})
})
