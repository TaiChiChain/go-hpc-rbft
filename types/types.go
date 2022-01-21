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

// ServiceState indicates the returned info from chain db.
type ServiceState struct {
	MetaState *MetaState
	Epoch     uint64
}

// MetaState is the basic info for block.
type MetaState struct {
	Height uint64
	Digest string
}

// EpochInfo is the information for epoch, including the epoch number, last configuration block height, and validator set.
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
	s += fmt.Sprintf("epoch: %d", state.Epoch)
	if s == "" {
		return "NIL ServiceState"
	}
	return s
}

// ----------- Reload related structs-----------------

// ReloadType indicates different reload event types.
type ReloadType int32

const (
	// ReloadTypeFinishReloadCommitDB indicates the reload event of commit-db finished.
	ReloadTypeFinishReloadCommitDB ReloadType = 1
)

// ReloadMessage is the event message after reload process finished.
type ReloadMessage struct {
	Type   ReloadType
	Height uint64
}

// ReloadFinished is the reload finished event.
type ReloadFinished struct {
	Height uint64
}

// ----------- Router related structs-----------------

// Router is the router info.
type Router struct {
	Peers []*Peer
}

// Peer is the peer info.
type Peer struct {
	ID       uint64
	Hostname string
	Hash     string
}

// ConfChange is the config change event.
type ConfChange struct {
	Router *Router
}

// ConfState is the config state structure.
type ConfState struct {
	QuorumRouter *Router
}

// ----------- Filter system related structs-----------------

// InformType indicates different event types.
type InformType int32

const (
	// InformTypeFilterFinishRecovery indicates recovery finished event.
	InformTypeFilterFinishRecovery InformType = 0

	// InformTypeFilterFinishViewChange indicates vc finished event.
	InformTypeFilterFinishViewChange InformType = 1

	// InformTypeFilterFinishConfigChange indicates config change finished event.
	InformTypeFilterFinishConfigChange InformType = 2

	// InformTypeFilterFinishStateUpdate indicates state update finished event.
	InformTypeFilterFinishStateUpdate InformType = 3

	// InformTypeFilterPoolFull indicates pool full event.
	InformTypeFilterPoolFull InformType = 4

	// InformTypeFilterStableCheckpoint indicates stable checkpoint event.
	InformTypeFilterStableCheckpoint InformType = 5
)
