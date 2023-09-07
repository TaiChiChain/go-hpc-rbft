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
	"errors"
	"time"

	"github.com/axiomesh/axiom-bft/common/consensus"
)

// peerManager maintains local peer ID which is the unique peer through the consensus network.
// router is the consensus network routing table which will be compared in syncState to ensure
// every node has the latest peer list.
type peerManager struct {
	chainConfig *ChainConfig

	// network helper to broadcast/unicast messages.
	network Network

	// logger
	logger Logger

	selfAccountAddress string
	selfID             uint64
	nodes              map[uint64]*NodeInfo
	msgNonce           int64
	isTest             bool
}

// init peer pool in rbft core
func newPeerManager(network Network, config Config, isTest bool) *peerManager {
	return &peerManager{
		selfAccountAddress: config.SelfAccountAddress,
		network:            network,
		logger:             config.Logger,
		msgNonce:           time.Now().UnixNano(),
		isTest:             isTest,
	}
}

func (m *peerManager) updateRoutingTable(c *ChainConfig) error {
	m.chainConfig = c
	if len(m.chainConfig.EpochInfo.ValidatorSet) == 0 {
		return errors.New("nil peers")
	}

	nodes := make(map[uint64]*NodeInfo, len(m.chainConfig.EpochInfo.ValidatorSet))
	var selfID uint64
	hasFound := false
	for _, p := range m.chainConfig.EpochInfo.ValidatorSet {
		if p.AccountAddress == m.selfAccountAddress {
			selfID = p.ID
			hasFound = true
		}
		nodes[p.ID] = p
	}
	for _, p := range m.chainConfig.EpochInfo.CandidateSet {
		if p.AccountAddress == m.selfAccountAddress {
			selfID = p.ID
			hasFound = true
		}
		nodes[p.ID] = p
	}
	if !hasFound {
		return errors.New("self not allowed in network")
	}
	m.selfID = selfID
	m.nodes = nodes
	m.logger.Infof("Local ID: %d, update routerMap:", m.selfID)
	return nil
}

func (m *peerManager) broadcast(ctx context.Context, msg *consensus.ConsensusMessage) {
	msg.From = m.selfID
	msg.Epoch = m.chainConfig.EpochInfo.Epoch
	msg.View = m.chainConfig.View
	if !m.isTest {
		m.msgNonce++
		msg.Nonce = m.msgNonce
	}

	err := m.network.Broadcast(ctx, msg)
	if err != nil {
		m.logger.Errorf("Broadcast failed: %v", err)
		return
	}
}

func (m *peerManager) unicast(ctx context.Context, msg *consensus.ConsensusMessage, to uint64) {
	msg.From = m.selfID
	msg.Epoch = m.chainConfig.EpochInfo.Epoch
	msg.View = m.chainConfig.View
	if !m.isTest {
		m.msgNonce++
		msg.Nonce = m.msgNonce
	}

	node, ok := m.nodes[to]
	if !ok {
		m.logger.Errorf("Unicast to %d failed: not found node", to)
		return
	}
	err := m.network.Unicast(ctx, msg, node.P2PNodeID)
	if err != nil {
		m.logger.Errorf("Unicast to %d failed: %v", to, err)
		return
	}
}
