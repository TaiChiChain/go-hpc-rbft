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
	"time"

	"github.com/axiomesh/axiom-bft/common"
	"github.com/axiomesh/axiom-bft/common/consensus"
)

// peerManager maintains local peer ID which is the unique peer through the consensus network.
// router is the consensus network routing table which will be compared in syncState to ensure
// every node has the latest peer list.
type peerManager struct {
	chainConfig *ChainConfig
	metrics     *rbftMetrics // collect all metrics in rbft

	// network helper to broadcast/unicast messages.
	network Network

	// logger
	logger common.Logger

	msgNonce int64
	isTest   bool
}

// init peer pool in rbft core
func newPeerManager(chainConfig *ChainConfig, metrics *rbftMetrics, network Network, config Config, isTest bool) *peerManager {
	return &peerManager{
		chainConfig: chainConfig,
		metrics:     metrics,
		network:     network,
		logger:      config.Logger,
		msgNonce:    time.Now().UnixNano(),
		isTest:      isTest,
	}
}

func (m *peerManager) broadcast(ctx context.Context, msg *consensus.ConsensusMessage) {
	msg.From = m.chainConfig.SelfID
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

func (m *peerManager) unicastByP2PID(ctx context.Context, msg *consensus.ConsensusMessage, p2pID string) {
	msg.From = m.chainConfig.SelfID
	msg.Epoch = m.chainConfig.EpochInfo.Epoch
	msg.View = m.chainConfig.View
	if !m.isTest {
		m.msgNonce++
		msg.Nonce = m.msgNonce
	}

	start := time.Now()
	err := m.network.Unicast(ctx, msg, p2pID)
	m.metrics.processEventDuration.With("event", "p2p_unicast").Observe(time.Since(start).Seconds())
	if err != nil {
		m.logger.Errorf("Unicast to %s failed: %v", p2pID, err)
		return
	}
}

func (m *peerManager) unicast(ctx context.Context, msg *consensus.ConsensusMessage, to uint64) {
	n, ok := m.chainConfig.NodeInfoMap[to]
	if !ok {
		m.logger.Errorf("Unicast to %d failed: not found node", to)
		return
	}

	m.unicastByP2PID(ctx, msg, n.P2PNodeID)
}

func (m *peerManager) unicastByAccountAddr(ctx context.Context, msg *consensus.ConsensusMessage, to string) {
	n, ok := m.chainConfig.AccountAddr2NodeInfoMap[to]
	if !ok {
		m.logger.Errorf("Unicast to %s failed: not found node", to)
		return
	}

	m.unicastByP2PID(ctx, msg, n.P2PNodeID)
}
