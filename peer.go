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
	"github.com/ultramesh/flato-rbft/external"
	pb "github.com/ultramesh/flato-rbft/rbftpb"
	"github.com/ultramesh/flato-rbft/types"
)

// peerPool maintains local peer ID which is the unique peer through the consensus network.
// router is the consensus network routing table which will be compared in syncState to ensure
// every node has the latest peer list.
type peerPool struct {
	// track local node's ID
	ID uint64

	// track local node's Hash
	hash string

	// track the vp router
	routerMap routerMap

	// network helper to broadcast/unicast messages.
	network external.Network

	// logger
	logger Logger
}

// init peer pool in rbft core
func newPeerPool(c Config) *peerPool {
	pool := &peerPool{
		ID:      c.ID,
		hash:    c.Hash,
		network: c.External,
		logger:  c.Logger,
	}
	pool.initPeers(c.Peers)

	return pool
}

func (pool *peerPool) initPeers(peers []*types.Peer) {
	pool.logger.Infof("Local ID: %d, update routerMap:", pool.ID)
	length := len(peers)
	preID := pool.ID
	pool.routerMap.HashMap = make(map[string]uint64)
	for _, p := range peers {
		if p.ID > uint64(length) {
			pool.logger.Errorf("Something wrong with peer[id=%d], peer id cannot be larger than peers' amount %d", p.ID, length)
			return
		}
		if p.Hash == pool.hash {
			pool.ID = p.ID
		}
		pool.logger.Infof("ID: %d, Hash: %s", p.ID, p.Hash)
		pool.routerMap.HashMap[p.Hash] = p.ID
	}
	if preID != pool.ID {
		pool.logger.Infof("Update Local ID: %d ===> %d", preID, pool.ID)
	}
}

func (pool *peerPool) updateRouter(router *types.Router) {
	cc := &types.ConfChange{
		Router: router,
	}
	pool.network.UpdateTable(cc)
}

func (pool *peerPool) broadcast(msg *pb.ConsensusMessage) {
	err := pool.network.Broadcast(msg)
	if err != nil {
		pool.logger.Errorf("Broadcast failed: %v", err)
		return
	}
}

func (pool *peerPool) unicast(msg *pb.ConsensusMessage, to uint64) {
	err := pool.network.Unicast(msg, to)
	if err != nil {
		pool.logger.Errorf("Unicast to %d failed: %v", to, err)
		return
	}
}

func (pool *peerPool) unicastByHash(msg *pb.ConsensusMessage, to string) {
	err := pool.network.UnicastByHash(msg, to)
	if err != nil {
		pool.logger.Errorf("Unicast to %d failed: %v", to, err)
		return
	}
}
