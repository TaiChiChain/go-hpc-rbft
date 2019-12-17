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

import "sync/atomic"

// StatusType defines the RBFT internal status.
type StatusType int

// consensus status type.
const (
	Normal = iota
	Pending
	InRecovery
	InViewChange
	InUpdatingN
	SkipInProgress
	StateTransferring
	NeedSyncState
	InSyncState
	PoolFull

	isNewNode
	byzantine
)

// NodeStatus reflects the internal consensus status.
type NodeStatus struct {
	ID     uint64
	View   uint64
	Status StatusType
}

type statusManager struct {
	status uint32 // consensus status
}

func newStatusMgr() *statusManager {
	return &statusManager{}
}

// reset only resets consensus status to 0.
func (st *statusManager) reset() {
	atomic.StoreUint32(&st.status, 0)
}

// setBit sets the bit at position in integer n.
func (st *statusManager) setBit(position uint64) {
	// try CompareAndSwapUint64 until success
	for {
		oldStatus := atomic.LoadUint32(&st.status)
		if atomic.CompareAndSwapUint32(&st.status, oldStatus, oldStatus|(1<<position)) {
			break
		}
	}
}

// clearBit clears the bit at position in integer n.
func (st *statusManager) clearBit(position uint64) {
	// try CompareAndSwapUint64 until success
	for {
		oldStatus := atomic.LoadUint32(&st.status)
		if atomic.CompareAndSwapUint32(&st.status, oldStatus, oldStatus&^(1<<position)) {
			break
		}
	}
}

// hasBit checks whether a bit position is set.
func (st *statusManager) hasBit(position uint64) bool {
	val := atomic.LoadUint32(&st.status) & (1 << position)
	return val > 0
}

// on sets the status of specified positions.
func (rbft *rbftImpl) on(statusPos ...uint64) {
	for _, pos := range statusPos {
		rbft.status.setBit(pos)
	}
}

// off resets the status of specified positions.
func (rbft *rbftImpl) off(statusPos ...uint64) {
	for _, pos := range statusPos {
		rbft.status.clearBit(pos)
	}
}

// in returns the status of specified position.
func (rbft *rbftImpl) in(pos uint64) bool {
	return rbft.status.hasBit(pos)
}

// inOne checks the result of several status computed with each other using '||'
func (rbft *rbftImpl) inOne(poss ...uint64) bool {
	var rs = false
	for _, pos := range poss {
		rs = rs || rbft.in(pos)
	}
	return rs
}

// setNormal sets system to normal.
func (rbft *rbftImpl) setNormal() {
	rbft.on(Normal)
}

// maybeSetNormal checks if system is in normal or not, if in normal, set status to normal.
func (rbft *rbftImpl) maybeSetNormal() {
	if !rbft.inOne(InRecovery, InUpdatingN, InViewChange, StateTransferring, Pending) {
		rbft.setNormal()
		rbft.startCheckPoolTimer()
	} else {
		rbft.logger.Debugf("Replica %d not set normal as it's still in abnormal now.", rbft.no)
	}
}

// setAbNormal sets system to abnormal which means system may be in viewChange,
// recovery, state update...
// we can't do sync state when we are in abnormal.
func (rbft *rbftImpl) setAbNormal() {
	rbft.exitSyncState()
	rbft.stopCheckPoolTimer()
	if rbft.isPrimary(rbft.peerPool.localID) {
		rbft.logger.Debug("Old primary stop batch timer before enter abnormal status")
		rbft.stopBatchTimer()
	}
	rbft.off(Normal)
}

// setFull means tx pool has reached the pool size.
func (rbft *rbftImpl) setFull() {
	rbft.on(PoolFull)
}

// setNotFull means tx pool hasn't reached the pool size.
func (rbft *rbftImpl) setNotFull() {
	rbft.off(PoolFull)
}

// isNormal checks setNormal and returns if system is normal or not.
func (rbft *rbftImpl) isNormal() bool {
	return rbft.in(Normal)
}

// isPoolFull checks and returns if tx pool is full or not.
func (rbft *rbftImpl) isPoolFull() bool {
	return rbft.in(PoolFull)
}

// initStatus init basic status when starts up
func (rbft *rbftImpl) initStatus() {
	rbft.status.reset()
	// set consensus status to pending to avoid process consensus messages
	// until RBFT starts recovery
	rbft.on(Pending)
	rbft.setNotFull()
}
