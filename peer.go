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

	"github.com/gogo/protobuf/proto"
)

// peerPool maintains local peer ID which is the unique peer through the consensus network.
// router is the consensus network routing table which will be compared in syncState to ensure
// every node has the latest peer list.
type peerPool struct {
	localID   uint64           // track local node's ID
	localHash string           // track local node's Hash
	router    *pb.Router       // track the vp replicas' routers
	network   external.Network // network helper to broadcast/unicast messages.
	logger    Logger
}

func newPeerPool(c Config) *peerPool {
	pool := &peerPool{
		localID:   c.ID,
		localHash: c.Hash,
		network:   c.External,
		logger:    c.Logger,
	}
	pool.initPeers(c.Peers)

	return pool
}

func (pool *peerPool) initPeers(peers []*pb.Peer) {
	pool.logger.Infof("Local ID: %d, update routers:", pool.localID)
	length := len(peers)
	pool.router = &pb.Router{
		Peers: make([]*pb.Peer, length),
	}
	preID := pool.localID
	for index, p := range peers {
		if index >= length {
			pool.logger.Errorf("Wrong length of peers, out of range: len=%d", length)
			return
		}
		if p.Hash == pool.localHash {
			pool.localID = p.Id
		}
		pool.logger.Infof("ID: %d, Hash: %s, Hostname: %s", p.Id, p.Hash, p.Hostname)
		pool.router.Peers[index] = p
	}
	if preID != pool.localID {
		pool.logger.Infof("Update Local ID: %d ===> %d", preID, pool.localID)
	}
}

func (pool *peerPool) serializeRouterInfo() []byte {
	info, _ := proto.Marshal(pool.router)
	return info
}

func (pool *peerPool) updateRouter(quorumRouter []byte) {
	cc := &pb.ConfChange{
		Type:    pb.ConfChangeType_ConfChangeUpdateNode,
		Context: quorumRouter,
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
