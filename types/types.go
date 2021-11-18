// Copyright 2016-2017 Hyperchain Corp.
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

// Package types defines all structs used by external modules.
package types

import (
	"fmt"

	"github.com/ultramesh/flato-common/types/protos"
)

// ----------- ServiceState related structs-----------------

type ServiceState struct {
	MetaState *MetaState
	EpochInfo *EpochInfo
}

type MetaState struct {
	Height uint64
	Digest string
}

type EpochInfo struct {
	Epoch      uint64
	VSet       []*protos.NodeInfo
	LastConfig uint64
}

func (state *ServiceState) String() string {
	var s string
	if state.MetaState != nil {
		s += fmt.Sprintf("height: %d, digest: %s\n", state.MetaState.Height,
			state.MetaState.Digest)
	}
	if state.EpochInfo != nil {
		s += fmt.Sprintf("epoch: %d, last config: %d", state.EpochInfo.Epoch,
			state.EpochInfo.LastConfig)
	}
	if s == "" {
		return "NIL ServiceState"
	}
	return s
}

// ----------- Reload related structs-----------------

type ReloadType int32

const (
	ReloadType_FinishReloadRouter   ReloadType = 0
	ReloadType_FinishReloadCommitDB ReloadType = 1
)

type ReloadMessage struct {
	Type   ReloadType
	Height uint64
	Router *Router
}

type ReloadFinished struct {
	Height uint64
}

// ----------- Router related structs-----------------

type Router struct {
	Peers []*Peer
}

type Peer struct {
	Id       uint64
	Hostname string
	Hash     string
}

type ConfChange struct {
	Router *Router
}

type ConfState struct {
	QuorumRouter *Router
}

// ----------- Filter system related structs-----------------

type InformType int32

const (
	InformType_FilterFinishRecovery     InformType = 0
	InformType_FilterFinishViewChange   InformType = 1
	InformType_FilterFinishConfigChange InformType = 2
	InformType_FilterFinishStateUpdate  InformType = 3
	InformType_FilterPoolFull           InformType = 4
	InformType_FilterStableCheckpoint   InformType = 5
)
