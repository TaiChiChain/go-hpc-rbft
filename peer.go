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
	"sync"

	"github.com/gogo/protobuf/proto"
)

// peerPool maintains local peer ID which is the unique peer through the consensus network.
// router is the consensus network routing table which will be compared in syncState to ensure
// every node has the latest peer list.
type peerPool struct {
	localID uint64 // track local node's ID
	self    *pb.Peer
	router  *pb.Router        // track the vp replicas' routers
	noMap   map[uint64]uint64 // map node's actual id to rbft.no
	lock    sync.RWMutex
	network external.Network // network helper to broadcast/unicast messages.
	logger  Logger
}

func newPeerPool(c Config) *peerPool {
	pool := &peerPool{
		localID: c.ID,
		noMap:   make(map[uint64]uint64),
		network: c.External,
		logger:  c.Logger,
	}
	pool.initPeers(c.Peers)

	return pool
}

func (pool *peerPool) initPeers(peers []*pb.Peer) {
	pool.logger.Infof("Local ID: %d, update routers:", pool.localID)
	pool.self = nil
	pool.lock.Lock()
	defer pool.lock.Unlock()
	pool.noMap = make(map[uint64]uint64)
	pool.router = &pb.Router{
		Peers: make([]*pb.Peer, 0),
	}
	for i, p := range peers {
		if p.Id == pool.localID {
			pool.self = p
		}
		pool.noMap[p.Id] = uint64(i + 1)
		pool.logger.Infof("ID: %d ===> no: %d, context: %s", p.Id, i+1, p.Context)
		pool.router.Peers = append(pool.router.Peers, p)
	}
}

func (pool *peerPool) serializeRouterInfo() []byte {
	info, _ := proto.Marshal(pool.router)
	return info
}

func (pool *peerPool) unSerializeRouterInfo(info []byte) *pb.Router {
	router := &pb.Router{}
	_ = proto.Unmarshal(info, router)
	return router
}

func (pool *peerPool) getSelfInfo() []byte {
	info, _ := proto.Marshal(pool.self)
	return info
}

func (pool *peerPool) isInRoutingTable(id uint64) bool {
	for _, peer := range pool.router.Peers {
		if peer.Id == id {
			return true
		}
	}
	return false
}

func (pool *peerPool) getPeerByID(id uint64) *pb.Peer {
	for _, peer := range pool.router.Peers {
		if peer.Id == id {
			return peer
		}
	}
	return nil
}

func (pool *peerPool) updateAddNode(newNodeID uint64, newNodeInfo []byte) {
	cc := &pb.ConfChange{
		NodeID:  newNodeID,
		Type:    pb.ConfChangeType_ConfChangeAddNode,
		Context: newNodeInfo,
	}
	pool.network.UpdateTable(cc)
}

func (pool *peerPool) updateDelNode(delNodeID uint64, delNodeInfo []byte) {
	cc := &pb.ConfChange{
		NodeID:  delNodeID,
		Type:    pb.ConfChangeType_ConfChangeRemoveNode,
		Context: delNodeInfo,
	}
	pool.network.UpdateTable(cc)
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
