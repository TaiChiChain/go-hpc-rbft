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

	"github.com/ultramesh/flato-common/types/protos"
	pb "github.com/ultramesh/flato-rbft/rbftpb"
)

// Node represents a node in a RBFT cluster.
type Node interface {
	// Start starts a RBFT node instance.
	Start() error
	// Propose proposes requests to RBFT core, requests are ensured to be eventually
	// submitted to all non-fault nodes unless current node crash down.
	Propose(requests []*protos.Transaction) error
	// ProposeConfChange proposes config change.
	// Application needs to call ApplyConfChange when applying EntryConfChange type entry.
	ProposeConfChange(cc *pb.ConfChange) error
	// Step advances the state machine using the given message.
	Step(msg *pb.ConsensusMessage)
	// ApplyConfChange applies config change to the local node.
	ApplyConfChange(cc *pb.ConfState)
	// Status returns the current node status of the RBFT state machine.
	Status() NodeStatus

	// ServiceInbound receives and records modifications from application service.
	ServiceInbound

	// Stop performs any necessary termination of the Node.
	Stop()
}

// ServiceInbound receives and records modifications from application service which includes two events:
// 1. ReportExecuted is invoked after application service has actually applied a batch.
// 2. ReportStateUpdated is invoked after application service finished stateUpdate which must be
//    triggered by RBFT core before.
// ServiceInbound is corresponding to External.ServiceOutbound.
type ServiceInbound interface {
	// TODO(DH): set when start node.
	// ReportExecuted reports to RBFT core that application service has finished applied one batch with
	// current applied batch seqNo and state digest.
	// Users can report any necessary extra field optionally.
	// NOTE. Users should ReportExecuted directly after start node to help track the initial state.
	ReportExecuted(state *pb.ServiceState)

	// ReportStateUpdated reports to RBFT core that application service has finished one desired StateUpdate
	// request which was triggered by RBFT core before.
	// Users must ReportStateUpdated after RBFT core invoked StateUpdate request no matter this request was
	// finished successfully or not, otherwise, RBFT core will enter abnormal status infinitely.
	ReportStateUpdated(state *pb.ServiceState)

	// ReportRouterUpdated report router updated:
	// If validator set was changed after reload, service reports the latest router info to RBFT by ReportRouterUpdated
	ReportReloadFinished(reload *pb.ReloadMessage)
}

// node implements the Node interface and track application service synchronously to help RBFT core
// retrieve service state.
type node struct {
	// rbft is the actually RBFT service.
	rbft *rbftImpl

	// currentState maintains the current application service state.
	currentState *pb.ServiceState
	// cpChan is used between node and RBFT service to deliver checkpoint state.
	cpChan chan *pb.ServiceState
	// stateLock is used to ensure mutually exclusive access of currentState.
	stateLock sync.Mutex

	// reloadRouter is used to store the validator set from reload temporarily
	// for that consensus will use it to update router after the execution of config transaction
	reloadRouter *pb.Router
	// mutex to set value of reloadRouter
	reloadRouterLock sync.RWMutex
	// confChan is used to track if config transaction execution was finished by CommitDB
	confChan chan *pb.ReloadFinished

	config Config
	logger Logger
}

// NewNode initializes a Node service.
func NewNode(conf Config) (Node, error) {
	return newNode(conf)
}

// newNode help to initializes a Node service.
func newNode(conf Config) (*node, error) {
	cpChan := make(chan *pb.ServiceState)
	confC := make(chan *pb.ReloadFinished)

	rbft, err := newRBFT(cpChan, confC, conf)
	if err != nil {
		return nil, err
	}

	n := &node{
		rbft:     rbft,
		cpChan:   cpChan,
		confChan: confC,
		config:   conf,
		logger:   conf.Logger,

		reloadRouter: nil,
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

// Start stops a Node instance.
func (n *node) Stop() {
	n.stateLock.Lock()
	defer n.stateLock.Unlock()

	select {
	case <-n.cpChan:
	default:
		n.logger.Notice("close channel: checkpoint")
		close(n.cpChan)
	}

	select {
	case <-n.confChan:
	default:
		n.logger.Notice("close channel: config")
		close(n.confChan)
	}

	n.rbft.stop()
	n.currentState = nil
}

// Propose proposes requests to RBFT core, requests are ensured to be eventually
// submitted to all non-fault nodes unless current node crash down.
func (n *node) Propose(requests []*protos.Transaction) error {
	n.rbft.postRequests(requests)

	return nil
}

// ProposeConfChange proposes config change.
// Application needs to call ApplyConfChange when applying EntryConfChange type entry.
func (n *node) ProposeConfChange(cc *pb.ConfChange) error {
	return nil
}

// Step advances the state machine using the given message.
func (n *node) Step(msg *pb.ConsensusMessage) {
	n.rbft.step(msg)
}

// ApplyConfChange applies config change to the local node.
func (n *node) ApplyConfChange(cc *pb.ConfState) {
	n.rbft.postConfState(cc)
	return
}

// ReportExecuted reports to RBFT core that application service has finished applied one batch with
// current applied batch seqNo and state digest.
// Users can report any necessary extra field optionally.
func (n *node) ReportExecuted(state *pb.ServiceState) {
	n.stateLock.Lock()
	defer n.stateLock.Unlock()
	if n.currentState == nil {
		n.logger.Noticef("Init service state with: %+v", state)
		n.currentState = state
		return
	}
	if state.MetaState.Applied != 0 && state.MetaState.Applied <= n.currentState.MetaState.Applied {
		n.logger.Warningf("Receive invalid service state %+v, "+
			"current state %+v", state, n.currentState)
		return
	}
	n.logger.Debugf("Update service state: %+v", state)
	n.currentState = state

	// a config transaction executed or checkpoint, send state to checkpoint channel
	if !n.rbft.atomicIn(Pending) && n.rbft.readConfigBatchToExecute() == state.MetaState.Applied || state.MetaState.Applied%n.config.K == 0 && n.cpChan != nil {
		n.logger.Debugf("Report checkpoint: {%d, %s} to RBFT core", state.MetaState.Applied, state.MetaState.Digest)
		n.cpChan <- state
	}
}

// ReportStateUpdated reports to RBFT core that application service has finished one desired StateUpdate
// request which was triggered by RBFT core before.
// Users must ReportStateUpdated after RBFT core invoked StateUpdate request no matter this request was
// finished successfully or not, otherwise, RBFT core will enter abnormal status infinitely.
func (n *node) ReportStateUpdated(state *pb.ServiceState) {
	n.stateLock.Lock()
	defer n.stateLock.Unlock()
	if state.MetaState.Applied != 0 && state.MetaState.Applied <= n.currentState.MetaState.Applied {
		n.logger.Infof("Receive a service state with applied ID which is not "+
			"larger than current state, received: %+v, current state: %+v", state, n.currentState)
	}
	n.currentState = state
	n.rbft.reportStateUpdated(state)
}

// ReportRouterUpdated report router updated
func (n *node) ReportReloadFinished(reload *pb.ReloadMessage) {
	switch reload.Type {
	case pb.ReloadType_FinishReloadRouter:
		n.logger.Noticef("Consensus-Reload finished, recv router: %+v", reload.Router)
		n.setReloadRouter(reload.Router)
	case pb.ReloadType_FinishReloadCommitDB:
		n.logger.Noticef("Commit-DB finished, recv height: %d", reload.Height)
		if n.rbft.atomicIn(InConfChange) && n.confChan != nil {
			rf := &pb.ReloadFinished{Height: reload.Height}
			n.confChan <- rf
		} else {
			n.logger.Info("Current node isn't in config-change")
		}
	}

}

// Status returns the current node status of the RBFT state machine.
func (n *node) Status() NodeStatus {
	return n.rbft.getStatus()
}

// getCurrentState retrieves the current application state.
func (n *node) getCurrentState() *pb.ServiceState {
	n.stateLock.Lock()
	defer n.stateLock.Unlock()
	return n.currentState
}

func (n *node) setReloadRouter(router *pb.Router) {
	n.reloadRouterLock.Lock()
	defer n.reloadRouterLock.Unlock()
	n.reloadRouter = router
}

func (n *node) readReloadRouter() *pb.Router {
	n.reloadRouterLock.RLock()
	defer n.reloadRouterLock.RUnlock()
	return n.reloadRouter
}
