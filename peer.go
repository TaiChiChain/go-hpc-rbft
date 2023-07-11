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
	"context"

	consensus "github.com/hyperchain/go-hpc-rbft/v2/common/consensus"
	"github.com/hyperchain/go-hpc-rbft/v2/external"
	"github.com/hyperchain/go-hpc-rbft/v2/types"
)

// peerPool maintains local peer ID which is the unique peer through the consensus network.
// router is the consensus network routing table which will be compared in syncState to ensure
// every node has the latest peer list.
type peerPool struct {
	// track local node's ID
	ID uint64

	// track local node's Host
	hostname string

	// track node's epoch number
	epoch uint64

	// track node's view number
	view uint64

	// track the vp router
	router map[uint64]string

	// network helper to broadcast/unicast messages.
	network external.Network

	// logger
	logger Logger
}

// init peer pool in rbft core
func newPeerPool[T any, Constraint consensus.TXConstraint[T]](c Config[T, Constraint]) *peerPool {
	pool := &peerPool{
		ID:       c.ID,
		hostname: c.Hostname,
		epoch:    c.EpochInit,
		network:  c.External,
		logger:   c.Logger,
	}
	pool.initPeers(c.Peers)

	return pool
}

func (pool *peerPool) initPeers(peers []*types.Peer) {
	pool.logger.Infof("Local ID: %d, update routerMap:", pool.ID)
	length := len(peers)
	preID := pool.ID
	pool.router = make(map[uint64]string)
	for _, p := range peers {
		if p.ID > uint64(length) {
			pool.logger.Errorf("Something wrong with peer[id=%d], peer id cannot be larger than peers' amount %d", p.ID, length)
			return
		}
		if p.Hostname == pool.hostname {
			pool.ID = p.ID
		}
		pool.logger.Infof("ID: %d, Hostname: %s", p.ID, p.Hostname)
		pool.router[p.ID] = p.Hostname
	}
	if preID != pool.ID {
		pool.logger.Infof("Update Local ID: %d ===> %d", preID, pool.ID)
	}
}

func (pool *peerPool) broadcast(ctx context.Context, msg *consensus.ConsensusMessage) {
	msg.From = pool.ID
	msg.Author = pool.hostname
	msg.Epoch = pool.epoch
	msg.View = pool.view
	err := pool.network.Broadcast(ctx, msg)
	if err != nil {
		pool.logger.Errorf("Broadcast failed: %v", err)
		return
	}
}

func (pool *peerPool) unicast(ctx context.Context, msg *consensus.ConsensusMessage, to uint64) {
	msg.From = pool.ID
	msg.Author = pool.hostname
	msg.Epoch = pool.epoch
	msg.View = pool.view
	err := pool.network.Unicast(ctx, msg, to)
	if err != nil {
		pool.logger.Errorf("Unicast to %d failed: %v", to, err)
		return
	}
}

func (pool *peerPool) unicastByHostname(ctx context.Context, msg *consensus.ConsensusMessage, to string) {
	msg.From = pool.ID
	msg.Author = pool.hostname
	msg.Epoch = pool.epoch
	msg.View = pool.view
	err := pool.network.UnicastByHostname(ctx, msg, to)
	if err != nil {
		pool.logger.Errorf("Unicast to %s failed: %v", to, err)
		return
	}
}
