/*
 * Tencent is pleased to support the open source community by making TKEStack
 * available.
 *
 * Copyright (C) 2012-2022 Tencent. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use
 * this file except in compliance with the License. You may obtain a copy of the
 * License at
 *
 * https://opensource.org/licenses/Apache-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
 * WARRANTIES OF ANY KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations under the License.
 */

package metrics

import (
	"sync"
	"time"

	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
)

const (
	// Namespace is the etcd controller's metric namespace
	Namespace = ""
	// EtcdControllerSubsystem - subsystem name used for this controller.
	EtcdControllerSubsystem = "etcd_controller"
)

var (
	// EtcdObserveLatency observe the latency from etcd object created to the etcd controller watched
	EtcdObserveLatency = metrics.NewSummary(
		&metrics.SummaryOpts{
			Namespace:      Namespace,
			Subsystem:      EtcdControllerSubsystem,
			Name:           "etcd_observe_latency",
			Help:           "Latency from etcd object created to the etcd controller watched",
			StabilityLevel: metrics.ALPHA,
		})

	// EtcdStartLatency observe the latency from etcd object created to etcd Cluster controller starting
	EtcdStartLatency = metrics.NewSummary(
		&metrics.SummaryOpts{
			Namespace: Namespace,
			Subsystem: EtcdControllerSubsystem,
			Name:      "etcd_start_latency",
			Help:      "Latency from etcd object created to etcd Cluster controller starting",
		})

	// EtcdRunningLatency observe the latency from etcd object created to etcd cluster running
	EtcdRunningLatency = metrics.NewSummary(
		&metrics.SummaryOpts{
			Namespace: Namespace,
			Subsystem: EtcdControllerSubsystem,
			Name:      "etcd_running_latency",
			Help:      "Latency from etcd object created to etcd cluster running",
		})

	// EtcdSyncTotal observes the total number of etcd synchronizations since the start of Controller
	EtcdSyncTotal = metrics.NewCounter(
		&metrics.CounterOpts{
			Namespace: Namespace,
			Subsystem: EtcdControllerSubsystem,
			Name:      "etcd_sync_total",
			Help:      "Total number of etcd synchronizations since the start of Controller",
		})

	// EtcdSyncSucceedTotal observes the total number of successful etcd synchronizations since the start of Controller
	EtcdSyncSucceedTotal = metrics.NewCounter(
		&metrics.CounterOpts{
			Namespace: Namespace,
			Subsystem: EtcdControllerSubsystem,
			Name:      "etcd_sync_succeed_total",
			Help:      "Total number of successful etcd synchronizations since the start of Controller",
		})

	// EtcdSyncFailedTotal observes the total number of failed etcd synchronizations since the start of Controller
	EtcdSyncFailedTotal = metrics.NewCounterVec(
		&metrics.CounterOpts{
			Namespace: Namespace,
			Subsystem: EtcdControllerSubsystem,
			Name:      "etcd_sync_failed_total",
			Help:      "The total number of failed etcd synchronizations since the start of Controller",
		},
		[]string{
			"reason",
		},
	)

	// ClustersTotal observes the total number of etcd managed by Controller
	ClustersTotal = metrics.NewGauge(&metrics.GaugeOpts{
		Namespace: Namespace,
		Subsystem: EtcdControllerSubsystem,
		Name:      "etcds_total",
		Help:      "Total number of etcd managed by the controller",
	})

	// EtcdSyncTimeCost observe the time cost for etcd cluster sync once
	EtcdSyncTimeCost = metrics.NewSummary(
		&metrics.SummaryOpts{
			Namespace: Namespace,
			Subsystem: EtcdControllerSubsystem,
			Name:      "etcd_sync_time_cost",
			Help:      "The time cost for etcd cluster sync once",
		},
	)

	// EtcdSyncInterval observe the time interval between two synchronizations of etcd
	EtcdSyncInterval = metrics.NewSummary(
		&metrics.SummaryOpts{
			Namespace: Namespace,
			Subsystem: EtcdControllerSubsystem,
			Name:      "etcd_sync_time_interval",
			Help:      "The time interval between two synchronizations of etcd",
		},
	)

	// EtcdStatusSyncInterval observe the time interval between two synchronizations of etcd status
	EtcdStatusSyncInterval = metrics.NewSummary(
		&metrics.SummaryOpts{
			Namespace: Namespace,
			Subsystem: EtcdControllerSubsystem,
			Name:      "etcd_status_sync_time_interval",
			Help:      "The time interval between two synchronizations of etcd status",
		},
	)
)

// RecordEtcdSyncTimeCost records the time cost for etcd cluster sync once
func RecordEtcdSyncTimeCost(cost time.Duration) {
	EtcdSyncTimeCost.Observe(cost.Seconds())
}

// RecordEtcdSyncTimeInterval record the time interval between two synchronizations of etcd
func RecordEtcdSyncTimeInterval(cost time.Duration) {
	EtcdSyncInterval.Observe(cost.Seconds())
}

// RecordEtcdStatusSyncTimeInterval record the time interval between two synchronizations of etcd status
func RecordEtcdStatusSyncTimeInterval(cost time.Duration) {
	EtcdStatusSyncInterval.Observe(cost.Seconds())
}

// RecordEtcdRunningLatency record the latency from etcd creating to running
func RecordEtcdRunningLatency(latency time.Duration) {
	EtcdRunningLatency.Observe(latency.Seconds())
}

var registerMetrics sync.Once

// Register registers etcd controller metrics.
func Register() {
	registerMetrics.Do(func() {
		legacyregistry.MustRegister(EtcdObserveLatency)
		legacyregistry.MustRegister(EtcdStartLatency)
		legacyregistry.MustRegister(EtcdRunningLatency)
		legacyregistry.MustRegister(EtcdSyncTotal)
		legacyregistry.MustRegister(EtcdSyncSucceedTotal)
		legacyregistry.MustRegister(EtcdSyncFailedTotal)
		legacyregistry.MustRegister(ClustersTotal)
		legacyregistry.MustRegister(EtcdSyncTimeCost)
		legacyregistry.MustRegister(EtcdSyncInterval)
		legacyregistry.MustRegister(EtcdStatusSyncInterval)
	})
}
