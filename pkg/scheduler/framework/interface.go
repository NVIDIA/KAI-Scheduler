/*
Copyright 2017 The Kubernetes Authors.

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

// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package framework

type ActionType string

const (
	Reclaim           ActionType = "reclaim"
	Preempt           ActionType = "preempt"
	Allocate          ActionType = "allocate"
	Consolidation     ActionType = "consolidation"
	StaleGangEviction ActionType = "stalegangeviction"
)

// Action is the interface of scheduler action.
type Action interface {
	// The unique name of Action.
	Name() ActionType

	// Execute allocates the cluster's resources into each queue.
	Execute(ssn *Session)
}

type Plugin interface {
	// The unique name of Plugin.
	Name() string

	OnSessionOpen(ssn *Session)
	OnSessionClose(ssn *Session)
}
