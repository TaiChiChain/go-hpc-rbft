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

package rbft

import (
	"encoding/json"

	"github.com/ultramesh/flato-rbft/external"
	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/gogo/protobuf/proto"
)

// unique peer identity
type Peer struct {
	ID   uint64 `json:"id"`
	Hash string `json:"hash"`
}

// peer list which we call consensus Router
type Router struct {
	Peers []Peer `json:"peers"`
}

// peerPool maintains local peer ID which is the unique peer through the consensus network.
// router is the consensus network routing table which will be compared in syncState to ensure
// every node has the latest peer list.
type peerPool struct {
	localID         uint64            // track local node's ID
	localHash       string            // track local node's hash
	tempNewNodeHash string            // track the temp new node hash
	router          Router            // track the vp replicas' routers
	noMap           map[uint64]uint64 // map node's actual id to rbft.no
	network         external.Network  // network helper to broadcast/unicast messages.
	logger          Logger
}

func newPeerPool(c Config) *peerPool {
	pool := &peerPool{
		localID:   c.ID,
		localHash: c.Hash,
		noMap:     make(map[uint64]uint64),
		network:   c.External,
		logger:    c.Logger,
	}
	pool.router.Peers = c.Peers

	return pool
}

func (pool *peerPool) setLocalID(id uint64) {
	pool.localID = id
}

func (pool *peerPool) setLocalHash(hash string) {
	pool.localHash = hash
}

func (pool *peerPool) getRouterInfo() []byte {
	info, _ := json.Marshal(pool.router)
	return info
}

func (pool *peerPool) isInRoutingTable(hash string) bool {
	for _, peer := range pool.router.Peers {
		if peer.Hash == hash {
			return true
		}
	}
	return false
}

// TODO(DH): is it necessary to keep node hash here?
func (pool *peerPool) findRouterIndexByHash(hash string) uint64 {
	for i, router := range pool.router.Peers {
		if router.Hash == hash {
			return uint64(i) + 1
		}
	}
	pool.logger.Warningf("Can not find replica with hash:%s", hash)
	return 0
}

func (pool *peerPool) updateAddNode(newNodeHash string) {
	pool.network.UpdateTable([]byte(newNodeHash), pb.ConfChangeType_ConfChangeAddNode)
}

func (pool *peerPool) updateDelNode(delNodeHash string) {
	pool.network.UpdateTable([]byte(delNodeHash), pb.ConfChangeType_ConfChangeRemoveNode)
}

func (pool *peerPool) updateRouter(quorumRouter []byte) {
	pool.network.UpdateTable(quorumRouter, pb.ConfChangeType_ConfChangeUpdateNode)
}

func (pool *peerPool) broadcast(msg *pb.ConsensusMessage) {
	m, err := proto.Marshal(msg)
	if err != nil {
		pool.logger.Errorf("Marshal consensus message failed: %v", err)
		return
	}
	err = pool.network.Broadcast(m)
	if err != nil {
		pool.logger.Errorf("Broadcast failed: %v", err)
		return
	}
}

func (pool *peerPool) unicast(msg *pb.ConsensusMessage, to uint64) {
	m, err := proto.Marshal(msg)
	if err != nil {
		pool.logger.Errorf("Marshal consensus message failed: %v", err)
		return
	}
	err = pool.network.Unicast(m, to)
	if err != nil {
		pool.logger.Errorf("Unicast to %d failed: %v", to, err)
		return
	}
}
