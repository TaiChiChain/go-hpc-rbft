// Copyright 2016-2019 Flato Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import "time"

// Enabled is checked by the constructor functions for all of the
// standard metrics. If it is true, the metric returned is a stub.
// This global kill-switch helps quantify the observer effect and makes
// for less cluttered pprof profiles.
var Enabled = false

// EnabledExpensive is a soft-flag meant for external packages to check if costly
// metrics gathering is allowed or not. The goal is to separate standard metrics
// for health monitoring and debug metrics that might impact runtime performance.
var EnabledExpensive = false

// EnableExpensive returns whether enable expensive metrics or not.
func EnableExpensive() bool {
	return EnabledExpensive
}

// A Provider is an abstraction for a metrics provider. It is a factory for
// Counter, Gauge, and Histogram meters.
type Provider interface {
	// SubProvider creates a new instance of Provider with given suffix.
	SubProvider(suffix string) Provider

	// SetNamespace set given Namespace to a provider.
	SetNamespace(namespace string)

	// NewCounter creates a new instance of a Counter.
	NewCounter(CounterOpts) (Counter, error)

	// NewGauge creates a new instance of a Gauge.
	NewGauge(GaugeOpts) (Gauge, error)

	// NewHistogram creates a new instance of a Histogram.
	NewHistogram(HistogramOpts) (Histogram, error)

	// NewSummary creates a new instance of a Summary.
	NewSummary(SummaryOpts) (Summary, error)
}

// A Counter represents a monotonically increasing value.
type Counter interface {
	// With is used to provide label values when updating a Counter. This must be
	// used to provide values for all LabelNames provided to CounterOpts.
	With(labelValues ...string) Counter

	// Add increments a counter value.
	Add(delta float64)

	// Unregister un-register current counter.
	Unregister()
}

// CounterOpts is used to provide basic information about a counter to the
// metrics subsystem.
type CounterOpts struct {
	// Namespace, Subsystem, and Name are components of the fully-qualified name
	// of the Metric. The fully-qualified name is created by joining these
	// components with an appropriate separator. Only Name is mandatory, the
	// others merely help structuring the name.
	Namespace string

	Subsystem string
	Name      string

	// Help provides information about this metric.
	Help string

	// LabelNames provides the names of the labels that can be attached to this
	// metric. When a metric is recorded, label values must be provided for each
	// of these label names.
	LabelNames []string
}

// A Gauge is a meter that expresses the current value of some metric.
type Gauge interface {
	// With is used to provide label values when recording a Gauge value. This
	// must be used to provide values for all LabelNames provided to GaugeOpts.
	With(labelValues ...string) Gauge

	// Add increments a Gauge value.
	Add(delta float64) // TODO: consider removing

	// Set is used to update the current value associated with a Gauge.
	Set(value float64)

	// Unregister un-register current gauge.
	Unregister()
}

// GaugeOpts is used to provide basic information about a gauge to the
// metrics subsystem.
type GaugeOpts struct {
	// Namespace, Subsystem, and Name are components of the fully-qualified name
	// of the Metric. The fully-qualified name is created by joining these
	// components with an appropriate separator. Only Name is mandatory, the
	// others merely help structuring the name.
	Namespace string

	Subsystem string
	Name      string

	// Help provides information about this metric.
	Help string

	// LabelNames provides the names of the labels that can be attached to this
	// metric. When a metric is recorded, label values must be provided for each
	// of these label names.
	LabelNames []string
}

// A Histogram is a meter that records an observed value into quantized
// buckets.
type Histogram interface {
	// With is used to provide label values when recording a Histogram
	// observation. This must be used to provide values for all LabelNames
	// provided to HistogramOpts.
	With(labelValues ...string) Histogram

	// Observe adds a single observation to the histogram.
	Observe(value float64)

	// Unregister un-register current histogram.
	Unregister()
}

// HistogramOpts is used to provide basic information about a histogram to the
// metrics subsystem.
type HistogramOpts struct {
	// Namespace, Subsystem, and Name are components of the fully-qualified name
	// of the Metric. The fully-qualified name is created by joining these
	// components with an appropriate separator. Only Name is mandatory, the
	// others merely help structuring the name.
	Namespace string

	Subsystem string
	Name      string

	// Help provides information about this metric.
	Help string

	// Buckets can be used to provide the bucket boundaries for Prometheus. When
	// omitted, the default Prometheus bucket values are used.
	Buckets []float64

	// LabelNames provides the names of the labels that can be attached to this
	// metric. When a metric is recorded, label values must be provided for each
	// of these label names.
	LabelNames []string
}

// A Summary is a meter that records an observed value into quantized
// buckets.
type Summary interface {
	// With is used to provide label values when recording a Histogram
	// observation. This must be used to provide values for all LabelNames
	// provided to Histogram.
	With(labelValues ...string) Summary

	// Observe adds a single observation to the summary.
	Observe(value float64)

	// Unregister un-register current summary.
	Unregister()
}

// SummaryOpts is used to provide basic information about a Summary to the
// metrics subsystem.
type SummaryOpts struct {
	// Namespace, Subsystem, and Name are components of the fully-qualified name
	// of the Metric. The fully-qualified name is created by joining these
	// components with an appropriate separator. Only Name is mandatory, the
	// others merely help structuring the name.
	Namespace string

	Subsystem string
	Name      string

	// Help provides information about this metric.
	Help string

	// Objectives defines the quantile rank estimates with their respective
	// absolute error. If Objectives[q] = e, then the value reported for q
	// will be the φ-quantile value for some φ between q-e and q+e.  The
	// default value is an empty map, resulting in a summary without
	// quantiles.
	Objectives map[float64]float64

	// MaxAge defines the duration for which an observation stays relevant
	// for the summary. Must be positive. The default value is DefMaxAge.
	MaxAge time.Duration

	// AgeBuckets is the number of buckets used to exclude observations that
	// are older than MaxAge from the summary. A higher number has a
	// resource penalty, so only increase it if the higher resolution is
	// really required. For very high observation rates, you might want to
	// reduce the number of age buckets. With only one age bucket, you will
	// effectively see a complete reset of the summary each time MaxAge has
	// passed. The default value is DefAgeBuckets.
	AgeBuckets uint32

	// BufCap defines the default sample stream buffer size.  The default
	// value of DefBufCap should suffice for most uses. If there is a need
	// to increase the value, a multiple of 500 is recommended (because that
	// is the internal buffer size of the underlying package
	// "github.com/bmizerany/perks/quantile").
	BufCap uint32

	// LabelNames provides the names of the labels that can be attached to this
	// metric. When a metric is recorded, label values must be provided for each
	// of these label names.
	LabelNames []string
}
