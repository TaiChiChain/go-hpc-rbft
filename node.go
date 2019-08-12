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

	"github.com/ultramesh/flato-event/inner/protos"
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

	config Config
	logger Logger
}

// NewNode initializes a Node service.
func NewNode(conf Config) (Node, error) {
	cpChan := make(chan *pb.ServiceState)

	rbft, err := newRBFT(cpChan, conf)
	if err != nil {
		return nil, err
	}

	n := &node{
		rbft:   rbft,
		cpChan: cpChan,
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

// Start stops a Node instance.
func (n *node) Stop() {
	n.stateLock.Lock()
	defer n.stateLock.Unlock()
	n.currentState = nil
	n.cpChan = make(chan *pb.ServiceState)

	n.rbft.stop()
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
	switch cc.Type {
	case pb.ConfChangeType_ConfChangeAddNode:

	case pb.ConfChangeType_ConfChangeRemoveNode:
		n.rbft.removeNode(cc.NodeID)
	case pb.ConfChangeType_ConfChangeUpdateNode:

	default:
	}

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
	if state.Applied != 0 && state.Applied <= n.currentState.Applied {
		n.logger.Warningf("Receive invalid service state %+v, "+
			"current state %+v", state, n.currentState)
		return
	}
	n.logger.Debugf("Update service state: %+v", state)
	n.currentState = state

	if state.Applied%n.config.K == 0 {
		n.logger.Debugf("Report checkpoint: %d to RBFT core", state.Applied)
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
	if state.Applied != 0 && state.Applied <= n.currentState.Applied {
		n.logger.Criticalf("Receive invalid service state %+v, "+
			"current state %+v", state, n.currentState)
		return
	}
	n.currentState = state
	n.rbft.reportStateUpdated(state.Applied)
}

// Status returns the current node status of the RBFT state machine.
func (n *node) Status() NodeStatus {
	return n.rbft.postStatusRequest()
}

// getCurrentState retrieves the current application state.
func (n *node) getCurrentState() *pb.ServiceState {
	n.stateLock.Lock()
	defer n.stateLock.Unlock()
	return n.currentState
}
