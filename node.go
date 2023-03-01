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
	"sync"

	"github.com/hyperchain/go-hpc-common/types/protos"
	pb "github.com/hyperchain/go-hpc-rbft/v2/rbftpb"
	"github.com/hyperchain/go-hpc-rbft/v2/types"
)

// Node represents a node in a RBFT cluster.
type Node interface {
	// Start starts a RBFT node instance.
	Start() error
	// Stop performs any necessary termination of the Node.
	Stop() []*protos.Transaction
	// Propose proposes requests to RBFT core, requests are ensured to be eventually
	// submitted to all non-fault nodes unless current node crash down.
	Propose(requests *pb.RequestSet) error
	// Step advances the state machine using the given message.
	Step(msg *pb.ConsensusMessage)
	// ApplyConfChange applies config change to the local node.
	ApplyConfChange(cc *types.ConfState)
	// Status returns the current node status of the RBFT state machine.
	Status() NodeStatus
	// GetUncommittedTransactions returns uncommitted txs
	GetUncommittedTransactions(maxsize uint64) []*protos.Transaction
	// ServiceInbound receives and records modifications from application service.
	ServiceInbound
}

// ServiceInbound receives and records modifications from application service which includes two events:
// 1. ReportExecuted is invoked after application service has actually height a batch.
// 2. ReportStateUpdated is invoked after application service finished stateUpdate which must be
//    triggered by RBFT core before.
// ServiceInbound is corresponding to External.ServiceOutbound.
type ServiceInbound interface {
	// ReportExecuted reports to RBFT core that application service has finished height one batch with
	// current height batch seqNo and state digest.
	// Users can report any necessary extra field optionally.
	// NOTE. Users should ReportExecuted directly after start node to help track the initial state.
	ReportExecuted(state *types.ServiceState)

	// ReportStateUpdated reports to RBFT core that application service has finished one desired StateUpdate
	// request which was triggered by RBFT core before.
	// Users must ReportStateUpdated after RBFT core invoked StateUpdate request no matter this request was
	// finished successfully or not, otherwise, RBFT core will enter abnormal status infinitely.
	ReportStateUpdated(state *types.ServiceState)

	// ReportStableCheckpointFinished reports stable checkpoint event has been processed successfully.
	ReportStableCheckpointFinished(height uint64)
}

// node implements the Node interface and track application service synchronously to help RBFT core
// retrieve service state.
type node struct {
	// rbft is the actually RBFT service.
	rbft *rbftImpl

	// stateLock is used to ensure mutually exclusive access of currentState.
	stateLock sync.RWMutex
	// currentState maintains the current application service state.
	currentState *types.ServiceState

	config Config
	logger Logger
}

// NewNode initializes a Node service.
func NewNode(conf Config) (Node, error) {
	return newNode(conf)
}

// newNode help to initialize a Node service.
func newNode(conf Config) (*node, error) {
	rbft, err := newRBFT(conf)
	if err != nil {
		return nil, err
	}

	n := &node{
		rbft:   rbft,
		config: conf,
		logger: conf.Logger,
	}

	rbft.node = n

	return n, nil
}

// Start starts a Node instance.
func (n *node) Start() error {
	err := n.rbft.start()
	if err != nil {
		return err
	}

	return nil
}

// Stop stops a Node instance.
func (n *node) Stop() []*protos.Transaction {
	// stop RBFT core.
	remainTxs := n.rbft.stop()

	n.stateLock.Lock()
	n.currentState = nil
	n.stateLock.Unlock()

	return remainTxs
}

// Propose proposes requests to RBFT core, requests are ensured to be eventually
// submitted to all non-fault nodes unless current node crash down.
func (n *node) Propose(requests *pb.RequestSet) error {
	n.rbft.postRequests(requests)

	return nil
}

// Step advances the state machine using the given message.
func (n *node) Step(msg *pb.ConsensusMessage) {
	n.rbft.step(msg)
}

// ApplyConfChange applies config change to the local node.
func (n *node) ApplyConfChange(cc *types.ConfState) {
	n.rbft.postConfState(cc)
}

// ReportExecuted reports to RBFT core that application service has finished height one batch with
// current height batch seqNo and state digest.
// Users can report any necessary extra field optionally.
func (n *node) ReportExecuted(state *types.ServiceState) {
	n.stateLock.Lock()
	if n.currentState == nil {
		n.logger.Noticef("Init service state with: %s", state)
		n.currentState = state
		n.stateLock.Unlock()
		return
	}
	if state.MetaState.Height != 0 && state.MetaState.Height <= n.currentState.MetaState.Height {
		n.logger.Warningf("Receive invalid service state %+v, "+
			"current state %+v", state, n.currentState)
		n.stateLock.Unlock()
		return
	}
	n.logger.Debugf("Update service state: %s", state)
	n.currentState = state
	n.stateLock.Unlock()

	// a config transaction executed or checkpoint, send state to checkpoint channel
	n.rbft.reportCheckpoint(state)
}

// ReportStateUpdated reports to RBFT core that application service has finished one desired StateUpdate
// request which was triggered by RBFT core before.
// Users must ReportStateUpdated after RBFT core invoked StateUpdate request no matter this request was
// finished successfully or not, otherwise, RBFT core will enter abnormal status infinitely.
func (n *node) ReportStateUpdated(state *types.ServiceState) {
	n.stateLock.Lock()
	if state.MetaState.Height != 0 && state.MetaState.Height <= n.currentState.MetaState.Height {
		n.logger.Infof("Receive a service state with height ID which is not "+
			"larger than current state, received: %+v, current state: %+v", state, n.currentState)
	}
	n.currentState = state
	n.stateLock.Unlock()

	n.rbft.reportStateUpdated(state)
}

// ReportStableCheckpointFinished reports stable checkpoint event has been processed successfully.
func (n *node) ReportStableCheckpointFinished(height uint64) {
	n.logger.Noticef("process stable checkpoint finished, height: %d", height)
	n.rbft.reportStableCheckpointFinished(height)
}

// Status returns the current node status of the RBFT state machine.
func (n *node) Status() NodeStatus {
	return n.rbft.getStatus()
}

func (n *node) GetUncommittedTransactions(maxsize uint64) []*protos.Transaction {
	// get hash of transactions that had committed
	var digestList []string
	committedHeight := n.rbft.h
	for digest, batch := range n.rbft.storeMgr.batchStore {
		if batch.SeqNo <= committedHeight {
			digestList = append(digestList, digest)
		}
	}
	// remove committed transactions
	n.rbft.batchMgr.requestPool.RemoveBatches(digestList)

	return n.rbft.batchMgr.requestPool.GetUncommittedTransactions(maxsize)
}

// getCurrentState retrieves the current application state.
func (n *node) getCurrentState() *types.ServiceState {
	n.stateLock.RLock()
	defer n.stateLock.RUnlock()
	return n.currentState
}
