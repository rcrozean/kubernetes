/*
Copyright 2021 The Kubernetes Authors.

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
	"errors"
	"net/http"
	"testing"
	"time"

	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/features"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
)

type workEstimateTestValues struct {
	maxSeats                  uint64
	initialSeatsExpected      uint64
	finalSeatsExpected        uint64
	additionalLatencyExpected time.Duration
}

func TestWorkEstimator(t *testing.T) {
	featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.WatchList, true)
	defaultMaximumSeatsLimit := uint64(10)
	defaultObjectsPerSeat := 100.0
	customMaximumSeatsLimit := uint64(50)
	customObjectsPerSeat := 500.0

	defaultCfg := NewWorkEstimatorConfig(defaultMaximumSeatsLimit, defaultObjectsPerSeat)
	customCfg := NewWorkEstimatorConfig(customMaximumSeatsLimit, customObjectsPerSeat)

	tests := []struct {
		name                   string
		requestURI             string
		requestInfo            *apirequest.RequestInfo
		counts                 map[string]int64
		countErr               error
		watchCount             int
		defaultEsitmatorValues *workEstimateTestValues
		customEsitmatorValues  *workEstimateTestValues
	}{
		{
			name:        "request has no RequestInfo",
			requestURI:  "http://server/apis/",
			requestInfo: nil,
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:             defaultMaximumSeatsLimit,
				initialSeatsExpected: 10,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:             customMaximumSeatsLimit,
				initialSeatsExpected: 50,
			},
		},
		{
			name:       "request verb is not list",
			requestURI: "http://server/apis/",
			requestInfo: &apirequest.RequestInfo{
				Verb: "get",
			},
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:             defaultMaximumSeatsLimit,
				initialSeatsExpected: 1,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:             customMaximumSeatsLimit,
				initialSeatsExpected: 1,
			},
		},
		{
			name:       "request verb is list, conversion to ListOptions returns error",
			requestURI: "http://server/apis/foo.bar/v1/events?limit=invalid",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "list",
				APIGroup: "foo.bar",
				Resource: "events",
			},
			counts: map[string]int64{
				"events.foo.bar": 799,
			},
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:             defaultMaximumSeatsLimit,
				initialSeatsExpected: 10,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:             customMaximumSeatsLimit,
				initialSeatsExpected: 50,
			},
		},
		{
			name:       "request verb is list, has limit and resource version is 1",
			requestURI: "http://server/apis/foo.bar/v1/events?limit=399&resourceVersion=1",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "list",
				APIGroup: "foo.bar",
				Resource: "events",
			},
			counts: map[string]int64{
				"events.foo.bar": 699,
			},
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:             defaultMaximumSeatsLimit,
				initialSeatsExpected: 8,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:             customMaximumSeatsLimit,
				initialSeatsExpected: 2,
			},
		},
		{
			name:       "request verb is list, limit not set",
			requestURI: "http://server/apis/foo.bar/v1/events?resourceVersion=1",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "list",
				APIGroup: "foo.bar",
				Resource: "events",
			},
			counts: map[string]int64{
				"events.foo.bar": 699,
			},
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:             defaultMaximumSeatsLimit,
				initialSeatsExpected: 7,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:             customMaximumSeatsLimit,
				initialSeatsExpected: 2,
			},
		},
		{
			name:       "request verb is list, resource version not set",
			requestURI: "http://server/apis/foo.bar/v1/events?limit=399",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "list",
				APIGroup: "foo.bar",
				Resource: "events",
			},
			counts: map[string]int64{
				"events.foo.bar": 699,
			},
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:             defaultMaximumSeatsLimit,
				initialSeatsExpected: 8,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:             customMaximumSeatsLimit,
				initialSeatsExpected: 2,
			},
		},
		{
			name:       "request verb is list, no query parameters, count known",
			requestURI: "http://server/apis/foo.bar/v1/events",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "list",
				APIGroup: "foo.bar",
				Resource: "events",
			},
			counts: map[string]int64{
				"events.foo.bar": 399,
			},
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:             defaultMaximumSeatsLimit,
				initialSeatsExpected: 8,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:             customMaximumSeatsLimit,
				initialSeatsExpected: 2,
			},
		},
		{
			name:       "request verb is list, no query parameters, count not known",
			requestURI: "http://server/apis/foo.bar/v1/events",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "list",
				APIGroup: "foo.bar",
				Resource: "events",
			},
			countErr: ObjectCountNotFoundErr,
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:             defaultMaximumSeatsLimit,
				initialSeatsExpected: 1,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:             customMaximumSeatsLimit,
				initialSeatsExpected: 1,
			},
		},
		{
			name:       "request verb is list, continuation is set",
			requestURI: "http://server/apis/foo.bar/v1/events?continue=token&limit=399",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "list",
				APIGroup: "foo.bar",
				Resource: "events",
			},
			counts: map[string]int64{
				"events.foo.bar": 699,
			},
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:             defaultMaximumSeatsLimit,
				initialSeatsExpected: 8,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:             customMaximumSeatsLimit,
				initialSeatsExpected: 2,
			},
		},
		{
			name:       "request verb is list, resource version is zero",
			requestURI: "http://server/apis/foo.bar/v1/events?limit=299&resourceVersion=0",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "list",
				APIGroup: "foo.bar",
				Resource: "events",
			},
			counts: map[string]int64{
				"events.foo.bar": 399,
			},
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:             defaultMaximumSeatsLimit,
				initialSeatsExpected: 4,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:             customMaximumSeatsLimit,
				initialSeatsExpected: 1,
			},
		},
		{
			name:       "request verb is list, resource version is zero, no limit",
			requestURI: "http://server/apis/foo.bar/v1/events?resourceVersion=0",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "list",
				APIGroup: "foo.bar",
				Resource: "events",
			},
			counts: map[string]int64{
				"events.foo.bar": 799,
			},
			defaultEsitmatorValues: &workEstimateTestValues{
				initialSeatsExpected: 8,
			},
			customEsitmatorValues: &workEstimateTestValues{
				initialSeatsExpected: 2,
			},
		},
		{
			name:       "request verb is list, resource version match is Exact",
			requestURI: "http://server/apis/foo.bar/v1/events?resourceVersion=foo&resourceVersionMatch=Exact&limit=399",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "list",
				APIGroup: "foo.bar",
				Resource: "events",
			},
			counts: map[string]int64{
				"events.foo.bar": 699,
			},
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:             defaultMaximumSeatsLimit,
				initialSeatsExpected: 8,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:             customMaximumSeatsLimit,
				initialSeatsExpected: 2,
			},
		},
		{
			name:       "request verb is list, resource version match is NotOlderThan, limit not specified",
			requestURI: "http://server/apis/foo.bar/v1/events?resourceVersion=foo&resourceVersionMatch=NotOlderThan",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "list",
				APIGroup: "foo.bar",
				Resource: "events",
			},
			counts: map[string]int64{
				"events.foo.bar": 799,
			},
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:             defaultMaximumSeatsLimit,
				initialSeatsExpected: 8,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:             customMaximumSeatsLimit,
				initialSeatsExpected: 2,
			},
		},
		{
			name:       "request verb is list, maximum is capped",
			requestURI: "http://server/apis/foo.bar/v1/events?resourceVersion=foo",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "list",
				APIGroup: "foo.bar",
				Resource: "events",
			},
			counts: map[string]int64{
				"events.foo.bar": 1999,
			},
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:             defaultMaximumSeatsLimit,
				initialSeatsExpected: 10,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:             customMaximumSeatsLimit,
				initialSeatsExpected: 4,
			},
		},
		{
			name:       "request verb is list, maximum is capped, lower max seats",
			requestURI: "http://server/apis/foo.bar/v1/events?resourceVersion=foo",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "list",
				APIGroup: "foo.bar",
				Resource: "events",
			},
			counts: map[string]int64{
				"events.foo.bar": 1999,
			},
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:             5,
				initialSeatsExpected: 5,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:             5,
				initialSeatsExpected: 4,
			},
		},
		{
			name:       "request verb is list, list from cache, count not known",
			requestURI: "http://server/apis/foo.bar/v1/events?resourceVersion=0&limit=799",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "list",
				APIGroup: "foo.bar",
				Resource: "events",
			},
			countErr: ObjectCountNotFoundErr,
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:             defaultMaximumSeatsLimit,
				initialSeatsExpected: 1,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:             customMaximumSeatsLimit,
				initialSeatsExpected: 1,
			},
		},
		{
			name:       "request verb is list, object count is stale",
			requestURI: "http://server/apis/foo.bar/v1/events?limit=499",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "list",
				APIGroup: "foo.bar",
				Resource: "events",
			},
			counts: map[string]int64{
				"events.foo.bar": 799,
			},
			countErr: ObjectCountStaleErr,
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:             defaultMaximumSeatsLimit,
				initialSeatsExpected: 10,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:             customMaximumSeatsLimit,
				initialSeatsExpected: 50,
			},
		},
		{
			name:       "request verb is list, object count is not found",
			requestURI: "http://server/apis/foo.bar/v1/events?limit=499",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "list",
				APIGroup: "foo.bar",
				Resource: "events",
			},
			countErr: ObjectCountNotFoundErr,
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:             defaultMaximumSeatsLimit,
				initialSeatsExpected: 1,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:             customMaximumSeatsLimit,
				initialSeatsExpected: 1,
			},
		},
		{
			name:       "request verb is list, count getter throws unknown error",
			requestURI: "http://server/apis/foo.bar/v1/events?limit=499",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "list",
				APIGroup: "foo.bar",
				Resource: "events",
			},
			countErr: errors.New("unknown error"),
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:             defaultMaximumSeatsLimit,
				initialSeatsExpected: 10,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:             customMaximumSeatsLimit,
				initialSeatsExpected: 50,
			},
		},
		{
			name:       "request verb is list, metadata.name specified",
			requestURI: "http://server/apis/foo.bar/v1/events?fieldSelector=metadata.name%3Dtest",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "list",
				Name:     "test",
				APIGroup: "foo.bar",
				Resource: "events",
			},
			counts: map[string]int64{
				"events.foo.bar": 799,
			},
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:             defaultMaximumSeatsLimit,
				initialSeatsExpected: 1,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:             customMaximumSeatsLimit,
				initialSeatsExpected: 1,
			},
		},
		{
			name:       "request verb is list, metadata.name, resourceVersion and limit specified",
			requestURI: "http://server/apis/foo.bar/v1/events?fieldSelector=metadata.name%3Dtest&limit=500&resourceVersion=0",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "list",
				Name:     "test",
				APIGroup: "foo.bar",
				Resource: "events",
			},
			counts: map[string]int64{
				"events.foo.bar": 799,
			},
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:             defaultMaximumSeatsLimit,
				initialSeatsExpected: 1,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:             customMaximumSeatsLimit,
				initialSeatsExpected: 1,
			},
		},
		{
			name:       "request verb is watch, sendInitialEvents is nil",
			requestURI: "http://server/apis/foo.bar/v1/events?watch=true",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "watch",
				APIGroup: "foo.bar",
				Resource: "events",
			},
			counts: map[string]int64{
				"events.foo.bar": 799,
			},
			defaultEsitmatorValues: &workEstimateTestValues{
				initialSeatsExpected: minimumSeats,
			},
			customEsitmatorValues: &workEstimateTestValues{
				initialSeatsExpected: minimumSeats,
			},
		},
		{
			name:       "request verb is watch, sendInitialEvents is false",
			requestURI: "http://server/apis/foo.bar/v1/events?watch=true&sendInitialEvents=false",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "watch",
				APIGroup: "foo.bar",
				Resource: "events",
			},
			counts: map[string]int64{
				"events.foo.bar": 799,
			},
			defaultEsitmatorValues: &workEstimateTestValues{
				initialSeatsExpected: minimumSeats,
			},
			customEsitmatorValues: &workEstimateTestValues{
				initialSeatsExpected: minimumSeats,
			},
		},
		{
			name:       "request verb is watch, sendInitialEvents is true",
			requestURI: "http://server/apis/foo.bar/v1/events?watch=true&sendInitialEvents=true",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "watch",
				APIGroup: "foo.bar",
				Resource: "events",
			},
			counts: map[string]int64{
				"events.foo.bar": 799,
			},
			defaultEsitmatorValues: &workEstimateTestValues{
				initialSeatsExpected: 8,
			},
			customEsitmatorValues: &workEstimateTestValues{
				initialSeatsExpected: 2,
			},
		},
		{
			name:       "request verb is create, no watches",
			requestURI: "http://server/apis/foo.bar/v1/foos",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "create",
				APIGroup: "foo.bar",
				Resource: "foos",
			},
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  defaultMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        0,
				additionalLatencyExpected: 0,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  customMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        0,
				additionalLatencyExpected: 0,
			},
		},
		{
			name:       "request verb is create, watches registered",
			requestURI: "http://server/apis/foo.bar/v1/foos",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "create",
				APIGroup: "foo.bar",
				Resource: "foos",
			},
			watchCount: 29,
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  defaultMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        3,
				additionalLatencyExpected: 5 * time.Millisecond,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  customMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        3,
				additionalLatencyExpected: 5 * time.Millisecond,
			},
		},
		{
			name:       "request verb is create, watches registered, no additional latency",
			requestURI: "http://server/apis/foo.bar/v1/foos",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "create",
				APIGroup: "foo.bar",
				Resource: "foos",
			},
			watchCount: 5,
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  defaultMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        0,
				additionalLatencyExpected: 0,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  customMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        0,
				additionalLatencyExpected: 0,
			},
		},
		{
			name:       "request verb is create, watches registered, maximum is capped",
			requestURI: "http://server/apis/foo.bar/v1/foos",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "create",
				APIGroup: "foo.bar",
				Resource: "foos",
			},
			watchCount: 199,
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  defaultMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        10,
				additionalLatencyExpected: 10 * time.Millisecond,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  customMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        20,
				additionalLatencyExpected: 5 * time.Millisecond,
			},
		},
		{
			name:       "request verb is update, no watches",
			requestURI: "http://server/apis/foo.bar/v1/foos/myfoo",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "update",
				APIGroup: "foo.bar",
				Resource: "foos",
			},
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  defaultMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        0,
				additionalLatencyExpected: 0,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  customMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        0,
				additionalLatencyExpected: 0,
			},
		},
		{
			name:       "request verb is update, watches registered",
			requestURI: "http://server/apis/foor.bar/v1/foos/myfoo",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "update",
				APIGroup: "foo.bar",
				Resource: "foos",
			},
			watchCount: 29,
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  defaultMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        3,
				additionalLatencyExpected: 5 * time.Millisecond,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  customMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        3,
				additionalLatencyExpected: 5 * time.Millisecond,
			},
		},
		{
			name:       "request verb is patch, no watches",
			requestURI: "http://server/apis/foo.bar/v1/foos/myfoo",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "patch",
				APIGroup: "foo.bar",
				Resource: "foos",
			},
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  defaultMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        0,
				additionalLatencyExpected: 0,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  customMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        0,
				additionalLatencyExpected: 0,
			},
		},
		{
			name:       "request verb is patch, watches registered",
			requestURI: "http://server/apis/foo.bar/v1/foos/myfoo",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "patch",
				APIGroup: "foo.bar",
				Resource: "foos",
			},
			watchCount: 29,
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  defaultMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        3,
				additionalLatencyExpected: 5 * time.Millisecond,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  customMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        3,
				additionalLatencyExpected: 5 * time.Millisecond,
			},
		},
		{
			name:       "request verb is patch, watches registered, lower max seats",
			requestURI: "http://server/apis/foo.bar/v1/foos/myfoo",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "patch",
				APIGroup: "foo.bar",
				Resource: "foos",
			},
			watchCount: 100,
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  5,
				initialSeatsExpected:      1,
				finalSeatsExpected:        5,
				additionalLatencyExpected: 10 * time.Millisecond,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  5,
				initialSeatsExpected:      1,
				finalSeatsExpected:        5,
				additionalLatencyExpected: 10 * time.Millisecond,
			},
		},
		{
			name:       "request verb is delete, no watches",
			requestURI: "http://server/apis/foo.bar/v1/foos/myfoo",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "delete",
				APIGroup: "foo.bar",
				Resource: "foos",
			},
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  defaultMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        0,
				additionalLatencyExpected: 0,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  customMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        0,
				additionalLatencyExpected: 0,
			},
		},
		{
			name:       "request verb is delete, watches registered",
			requestURI: "http://server/apis/foo.bar/v1/foos/myfoo",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "delete",
				APIGroup: "foo.bar",
				Resource: "foos",
			},
			watchCount: 29,
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  defaultMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        3,
				additionalLatencyExpected: 5 * time.Millisecond,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  customMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        3,
				additionalLatencyExpected: 5 * time.Millisecond,
			},
		},
		{
			name:       "creating token for service account",
			requestURI: "http://server/api/v1/namespaces/foo/serviceaccounts/default/token",
			requestInfo: &apirequest.RequestInfo{
				Verb:        "create",
				APIGroup:    "v1",
				Resource:    "serviceaccounts",
				Subresource: "token",
			},
			watchCount: 5777,
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  defaultMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        0,
				additionalLatencyExpected: 0,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  customMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        0,
				additionalLatencyExpected: 0,
			},
		},
		{
			name:       "creating service account",
			requestURI: "http://server/api/v1/namespaces/foo/serviceaccounts",
			requestInfo: &apirequest.RequestInfo{
				Verb:     "create",
				APIGroup: "v1",
				Resource: "serviceaccounts",
			},
			watchCount: 1000,
			defaultEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  defaultMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        10,
				additionalLatencyExpected: 50 * time.Millisecond,
			},
			customEsitmatorValues: &workEstimateTestValues{
				maxSeats:                  customMaximumSeatsLimit,
				initialSeatsExpected:      1,
				finalSeatsExpected:        50,
				additionalLatencyExpected: 10 * time.Millisecond,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			counts := test.counts
			if len(counts) == 0 {
				counts = map[string]int64{}
			}
			countsFn := func(key string) (int64, error) {
				return counts[key], test.countErr
			}
			watchCountsFn := func(_ *apirequest.RequestInfo) int {
				return test.watchCount
			}
			defaultMaxSeatsFn := func(_ string) uint64 {
				return test.defaultEsitmatorValues.maxSeats
			}
			customMaxSeatsFn := func(_ string) uint64 {
				return test.customEsitmatorValues.maxSeats
			}

			defaultEstimator := NewWorkEstimator(countsFn, watchCountsFn, defaultCfg, defaultMaxSeatsFn)
			customEstimator := NewWorkEstimator(countsFn, watchCountsFn, customCfg, customMaxSeatsFn)

			req, err := http.NewRequest("GET", test.requestURI, nil)
			if err != nil {
				t.Fatalf("Failed to create new HTTP request - %v", err)
			}

			if test.requestInfo != nil {
				req = req.WithContext(apirequest.WithRequestInfo(req.Context(), test.requestInfo))
			}

			workestimateGot := defaultEstimator.EstimateWork(req, "testFS", "testPL")
			if test.defaultEsitmatorValues.initialSeatsExpected != workestimateGot.InitialSeats {
				t.Errorf("[DEFAULT] Expected work estimate to match: %d initial seats, but got: %d", test.defaultEsitmatorValues.initialSeatsExpected, workestimateGot.InitialSeats)
			}
			if test.defaultEsitmatorValues.finalSeatsExpected != workestimateGot.FinalSeats {
				t.Errorf("[DEFAULT] Expected work estimate to match: %d final seats, but got: %d", test.defaultEsitmatorValues.finalSeatsExpected, workestimateGot.FinalSeats)
			}
			if test.defaultEsitmatorValues.additionalLatencyExpected != workestimateGot.AdditionalLatency {
				t.Errorf("[DEFAULT] Expected work estimate to match additional latency: %v, but got: %v", test.defaultEsitmatorValues.additionalLatencyExpected, workestimateGot.AdditionalLatency)
			}

			workestimateGot = customEstimator.EstimateWork(req, "testFS", "testPL")
			if test.customEsitmatorValues.initialSeatsExpected != workestimateGot.InitialSeats {
				t.Errorf("[CUSTOM] Expected work estimate to match: %d initial seats, but got: %d", test.customEsitmatorValues.initialSeatsExpected, workestimateGot.InitialSeats)
			}
			if test.customEsitmatorValues.finalSeatsExpected != workestimateGot.FinalSeats {
				t.Errorf("[CUSTOM] Expected work estimate to match: %d final seats, but got: %d", test.customEsitmatorValues.finalSeatsExpected, workestimateGot.FinalSeats)
			}
			if test.customEsitmatorValues.additionalLatencyExpected != workestimateGot.AdditionalLatency {
				t.Errorf("[CUSTOM] Expected work estimate to match additional latency: %v, but got: %v", test.customEsitmatorValues.additionalLatencyExpected, workestimateGot.AdditionalLatency)
			}
		})
	}
}
