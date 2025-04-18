/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package request

import (
	"os"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

const (
	minimumSeats                = 1
	defaultMaximumSeatsLimit    = 10
	defaultObjectsPerSeat       = 100.0
	watchesPerSeat              = 10.0
	enableMutatingWorkEstimator = true
	maxSeatsLimitEnvVar         = "APF_MAX_SEATS_LIMIT"
	objectsPerSeatEnvVar        = "APF_OBJECTS_PER_SEAT"
	failedToParseErr            = "failed to parse env var for key %q with value %q: %v"
)

var eventAdditionalDuration = 5 * time.Millisecond

// WorkEstimatorConfig holds work estimator parameters.
type WorkEstimatorConfig struct {
	*ListWorkEstimatorConfig     `json:"listWorkEstimatorConfig,omitempty"`
	*MutatingWorkEstimatorConfig `json:"mutatingWorkEstimatorConfig,omitempty"`

	// MinimumSeats is the minimum number of seats a request must occupy.
	MinimumSeats uint64 `json:"minimumSeats,omitempty"`

	// MaximumSeatsLimit is an upper limit on the max seats a request can occupy.
	//
	// NOTE: work_estimate_seats_samples metric uses the value of maximumSeats
	// as the upper bound, so when we change maximumSeats we should also
	// update the buckets of the metric.
	MaximumSeatsLimit uint64 `json:"maximumSeatsLimit,omitempty"`
}

// ListWorkEstimatorConfig holds work estimator parameters related to list requests.
type ListWorkEstimatorConfig struct {
	ObjectsPerSeat float64 `json:"objectsPerSeat,omitempty"`
}

// MutatingWorkEstimatorConfig holds work estimator
// parameters related to watches of mutating objects.
type MutatingWorkEstimatorConfig struct {
	// TODO(wojtekt): Remove it once we tune the algorithm to not fail
	// scalability tests.
	Enabled                 bool            `json:"enable,omitempty"`
	EventAdditionalDuration metav1.Duration `json:"eventAdditionalDurationMs,omitempty"`
	WatchesPerSeat          float64         `json:"watchesPerSeat,omitempty"`
}

func NewWorkEstimatorConfig() *WorkEstimatorConfig {
	maxSeatsLimit, envVal, err := getAndSetFromEnv(
		maxSeatsLimitEnvVar,
		defaultMaximumSeatsLimit,
		func(s string) (uint64, error) { return strconv.ParseUint(s, 10, 64) },
	)
	if err != nil {
		klog.Fatalf(failedToParseErr, maxSeatsLimitEnvVar, envVal, err)
	}

	objectsPerSeat, envVal, err := getAndSetFromEnv(
		objectsPerSeatEnvVar,
		defaultObjectsPerSeat,
		func(s string) (float64, error) { return strconv.ParseFloat(s, 64) },
	)
	if err != nil {
		klog.Fatalf(failedToParseErr, objectsPerSeatEnvVar, envVal, err)
	}

	return &WorkEstimatorConfig{
		MinimumSeats:                minimumSeats,
		MaximumSeatsLimit:           maxSeatsLimit,
		ListWorkEstimatorConfig:     newListWorkEstimatorConfig(objectsPerSeat),
		MutatingWorkEstimatorConfig: defaultMutatingWorkEstimatorConfig(),
	}
}

func getAndSetFromEnv[T any](key string, defaultValue T, converter func(string) (T, error)) (T, string, error) {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		cVal, err := converter(value)
		return cVal, value, err
	}
	return defaultValue, "", nil
}

// defaultListWorkEstimatorConfig creates a new ListWorkEstimatorConfig with default values.
func newListWorkEstimatorConfig(objectsPerSeat float64) *ListWorkEstimatorConfig {
	return &ListWorkEstimatorConfig{ObjectsPerSeat: objectsPerSeat}
}

// defaultMutatingWorkEstimatorConfig creates a new MutatingWorkEstimatorConfig with default values.
func defaultMutatingWorkEstimatorConfig() *MutatingWorkEstimatorConfig {
	return &MutatingWorkEstimatorConfig{
		Enabled:                 enableMutatingWorkEstimator,
		EventAdditionalDuration: metav1.Duration{Duration: eventAdditionalDuration},
		WatchesPerSeat:          watchesPerSeat,
	}
}

// eventAdditionalDuration converts eventAdditionalDurationMs to a time.Duration type.
func (c *MutatingWorkEstimatorConfig) eventAdditionalDuration() time.Duration {
	return c.EventAdditionalDuration.Duration
}
