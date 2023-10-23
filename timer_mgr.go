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
	"strconv"
	"sync"
	"time"

	"github.com/axiomesh/axiom-bft/common"
)

// titleTimer manages timer with the same timer name, which, we allow different timer with the same timer name, such as:
// we allow several request timers at the same time, each timer started after received a new request batch
type titleTimer struct {
	timerName string        // the unique timer name
	timeout   time.Duration // default timeout of this timer
	isActive  sync.Map      // track all the timers with this timerName if it is active now
}

func (tt *titleTimer) store(key, value any) {
	tt.isActive.Store(key, value)
}

func (tt *titleTimer) delete(key any) {
	afterTimer, ok := tt.isActive.LoadAndDelete(key)
	if ok {
		afterTimer.(*time.Timer).Stop()
	}
}

func (tt *titleTimer) has(key string) bool {
	_, ok := tt.isActive.Load(key)
	return ok
}

func (tt *titleTimer) count() int {
	length := 0
	tt.isActive.Range(func(_, _ any) bool {
		length++
		return true
	})
	return length
}

func (tt *titleTimer) clear() {
	tt.isActive.Range(func(key, afterTimer any) bool {
		tt.isActive.Delete(key)
		afterTimer.(*time.Timer).Stop()
		return true
	})
}

// timerManager manages consensus used timers.
type timerManager struct {
	tTimers   map[string]*titleTimer
	eventChan chan<- consensusEvent
	logger    common.Logger
}

// newTimerMgr news an instance of timerManager.
func newTimerMgr(eventC chan consensusEvent, c Config) *timerManager {
	tm := &timerManager{
		tTimers:   make(map[string]*titleTimer),
		eventChan: eventC,
		logger:    c.Logger,
	}

	return tm
}

// newTimer news a titleTimer with the given name and default timeout, then add this timer to timerManager
func (tm *timerManager) newTimer(name string, d time.Duration) {
	if d == 0 {
		switch name {
		case requestTimer:
			d = DefaultRequestTimeout
		case batchTimer:
			d = DefaultBatchTimeout
		case noTxBatchTimer:
			d = DefaultNoTxBatchTimeout
		case vcResendTimer:
			d = DefaultVcResendTimeout
		case newViewTimer:
			d = DefaultNewViewTimeout
		case nullRequestTimer:
			d = DefaultNullRequestTimeout
		case syncStateRspTimer:
			d = DefaultSyncStateRspTimeout
		case syncStateRestartTimer:
			d = DefaultSyncStateRestartTimeout
		case cleanViewChangeTimer:
			d = DefaultCleanViewChangeTimeout
		case checkPoolTimer:
			d = DefaultCheckPoolTimeout
		case checkPoolRemoveTimer:
			d = DefaultCheckPoolRemoveTimeout
		case fetchCheckpointTimer:
			d = DefaultFetchCheckpointTimeout
		case fetchViewTimer:
			d = DefaultFetchViewTimeout
		}
	}

	tm.tTimers[name] = &titleTimer{
		timerName: name,
		timeout:   d,
	}
}

// Stop stops all timers managed by timerManager
func (tm *timerManager) Stop() {
	for timerName := range tm.tTimers {
		tm.stopTimer(timerName)
	}
}

// startTimer starts the timer with the given name and default timeout, then sets the event which will be triggered
// after this timeout
func (tm *timerManager) startTimer(name string, event *LocalEvent) (key string) {
	return tm.startTimerWithNewTT(name, tm.tTimers[name].timeout, event)
}

// startTimerWithNewTT starts the timer with the given name and timeout, then sets the event which will be triggered
// after this timeout
func (tm *timerManager) startTimerWithNewTT(name string, timeout time.Duration, event *LocalEvent) (key string) {
	tm.stopTimer(name)

	return tm.createTimer(name, timeout, event)
}

// softStartTimerWithNewTT first checks if there exists some running timer with
// the given name, if existed, not start, else start a new timer with the given
// timeout.
func (tm *timerManager) softStartTimerWithNewTT(name string, timeout time.Duration, event *LocalEvent) (existed bool, key string) {
	if tm.tTimers[name].count() != 0 {
		return true, ""
	}

	return false, tm.createTimer(name, timeout, event)
}

// createTimer creates a goroutine and waits for timeout. Then check if the timer is active. If so, send event.
func (tm *timerManager) createTimer(name string, timeout time.Duration, event *LocalEvent) (key string) {
	timestamp := time.Now().UnixNano()
	key = strconv.FormatInt(timestamp, 10)
	send := func() {
		if tm.tTimers[name].has(key) {
			tm.eventChan <- event
		}
	}
	afterTimer := time.AfterFunc(timeout, send)
	tm.tTimers[name].store(key, afterTimer)

	return key
}

// stopTimer stops all timers with the same timerName.
func (tm *timerManager) stopTimer(timerName string) {
	if !tm.containsTimer(timerName) {
		tm.logger.Errorf("Stop timer failed, timer %s not created yet!", timerName)
		return
	}

	tm.tTimers[timerName].clear()
}

// stopOneTimer stops one timer by the timerName and key.
func (tm *timerManager) stopOneTimer(timerName string, key string) {
	if !tm.containsTimer(timerName) {
		tm.logger.Errorf("Stop timer failed!, timer %s not created yet!", timerName)
		return
	}
	tm.tTimers[timerName].delete(key)
}

// containsTimer returns true if there exists a timer named timerName
func (tm *timerManager) containsTimer(timerName string) bool {
	_, ok := tm.tTimers[timerName]
	return ok
}

// getTimeoutValue gets the default timeout of the given timer
func (tm *timerManager) getTimeoutValue(timerName string) time.Duration {
	if !tm.containsTimer(timerName) {
		tm.logger.Warningf("Get timeout failed!, timer %s not created yet!", timerName)
		return 0 * time.Second
	}
	return tm.tTimers[timerName].timeout
}

// setTimeoutValue sets the default timeout of the given timer with a new timeout
func (tm *timerManager) setTimeoutValue(timerName string, timeout time.Duration) {
	if !tm.containsTimer(timerName) {
		tm.logger.Warningf("Set timeout failed!, timer %s not created yet!", timerName)
		return
	}
	tm.tTimers[timerName].timeout = timeout
}

// initTimers creates timers when start up
func (rbft *rbftImpl[T, Constraint]) initTimers() {
	rbft.timerMgr.newTimer(vcResendTimer, rbft.config.VcResendTimeout)
	rbft.timerMgr.newTimer(nullRequestTimer, rbft.config.NullRequestTimeout)
	rbft.timerMgr.newTimer(newViewTimer, rbft.config.NewViewTimeout)
	rbft.timerMgr.newTimer(syncStateRspTimer, rbft.config.SyncStateTimeout)
	rbft.timerMgr.newTimer(syncStateRestartTimer, rbft.config.SyncStateRestartTimeout)
	rbft.timerMgr.newTimer(batchTimer, rbft.config.BatchTimeout)
	rbft.timerMgr.newTimer(noTxBatchTimer, rbft.config.NoTxBatchTimeout)
	rbft.timerMgr.newTimer(requestTimer, rbft.config.RequestTimeout)
	rbft.timerMgr.newTimer(cleanViewChangeTimer, rbft.config.CleanVCTimeout)
	rbft.timerMgr.newTimer(checkPoolTimer, rbft.config.CheckPoolTimeout)
	rbft.timerMgr.newTimer(fetchCheckpointTimer, rbft.config.FetchCheckpointTimeout)
	rbft.timerMgr.newTimer(fetchViewTimer, rbft.config.FetchViewTimeout)

	rbft.timerMgr.newTimer(checkPoolRemoveTimer, rbft.config.CheckPoolRemoveTimeout)

	rbft.timerMgr.makeNullRequestTimeoutLegal()
	rbft.timerMgr.makeRequestTimeoutLegal()
	rbft.timerMgr.makeCleanVcTimeoutLegal()
	rbft.timerMgr.makeSyncStateTimeoutLegal()

	// we know that a quorum set of nodes have already executed a block of high-watermark, but some of them missed the
	// checkpoint message. now, we can wait for another nodes' execution or just trigger view-change. as the view-change
	// process may take too long time to harm the throughput, we would like to wait for others' execution at first.
	// here, they should increase their watermark for a K-interval. as one block's consensus timeout is equal to
	// requestTimer, the high-watermark timer should be at least K*requestTimer. but checkpoint message should be sent
	// after executed, we have to take the latency of executor into thought. so that we would like to set high-watermark
	// timer 2*k*requestTimer
	k := rbft.chainConfig.EpochInfo.ConsensusParams.CheckpointPeriod
	if k <= uint64(0) {
		k = DefaultK
	}
	// here, the timer value must be legal, so that k*requestTimer cannot be zero
	reqT := rbft.timerMgr.getTimeoutValue(requestTimer)
	hwT := time.Duration(2 * k * uint64(reqT))
	rbft.timerMgr.newTimer(highWatermarkTimer, hwT)
	rbft.logger.Infof("RBFT high watermark timeout = %v", rbft.timerMgr.getTimeoutValue(highWatermarkTimer))
}

// makeNullRequestTimeoutLegal checks if nullRequestTimeout is legal or not, if not, make it
// legal, which, nullRequest timeout must be larger than requestTimeout
func (tm *timerManager) makeNullRequestTimeoutLegal() {
	nullRequestTimeout := tm.getTimeoutValue(nullRequestTimer)
	requestTimeout := tm.getTimeoutValue(requestTimer)

	if requestTimeout >= nullRequestTimeout && nullRequestTimeout != 0 {
		tm.setTimeoutValue(nullRequestTimer, 3*requestTimeout/2)
		tm.logger.Infof("Configured null request timeout must be greater "+
			"than request timeout, set to %v", tm.getTimeoutValue(nullRequestTimer))
	}

	if tm.getTimeoutValue(nullRequestTimer) > 0 {
		tm.logger.Infof("RBFT null request timeout = %v", tm.getTimeoutValue(nullRequestTimer))
	} else {
		tm.logger.Infof("RBFT null request disabled")
	}
}

// makeCleanVcTimeoutLegal checks if requestTimeout is legal or not, if not, make it
// legal, which, request timeout must be larger than batch timeout
func (tm *timerManager) makeRequestTimeoutLegal() {
	requestTimeout := tm.getTimeoutValue(requestTimer)
	batchTimeout := tm.getTimeoutValue(batchTimer)
	tm.logger.Infof("RBFT Batch timeout = %v", batchTimeout)

	if batchTimeout >= requestTimeout {
		tm.setTimeoutValue(requestTimer, 3*batchTimeout/2)
		tm.logger.Infof("Configured request timeout must be greater than batch timeout, set to %v", tm.getTimeoutValue(requestTimer))
	}
	tm.logger.Infof("RBFT request timeout = %v", tm.getTimeoutValue(requestTimer))
}

// makeCleanVcTimeoutLegal checks if clean vc timeout is legal or not, if not, make it
// legal, which, cleanVcTimeout should more than 6* viewChange time
func (tm *timerManager) makeCleanVcTimeoutLegal() {
	cleanVcTimeout := tm.getTimeoutValue(cleanViewChangeTimer)
	nvTimeout := tm.getTimeoutValue(newViewTimer)

	if cleanVcTimeout < 6*nvTimeout {
		cleanVcTimeout = 6 * nvTimeout
		tm.setTimeoutValue(cleanViewChangeTimer, cleanVcTimeout)
		tm.logger.Infof("Configured clean viewChange timeout is too short, set to %v", cleanVcTimeout)
	}

	tm.logger.Infof("RBFT null clean vc timeout = %v", tm.getTimeoutValue(cleanViewChangeTimer))
	tm.logger.Infof("RBFT new view timeout = %v", tm.getTimeoutValue(newViewTimer))
}

// makeSyncStateTimeoutLegal checks if syncStateRspTimeout is legal or not, if not, make it
// legal, which, syncStateRestartTimeout should more than 10 * syncStateRspTimeout
func (tm *timerManager) makeSyncStateTimeoutLegal() {
	rspTimeout := tm.getTimeoutValue(syncStateRspTimer)
	restartTimeout := tm.getTimeoutValue(syncStateRestartTimer)

	if restartTimeout < 10*rspTimeout {
		restartTimeout = 10 * rspTimeout
		tm.setTimeoutValue(syncStateRestartTimer, restartTimeout)
		tm.logger.Infof("Configured sync state restart timeout is too short, set to %v", restartTimeout)
	}

	tm.logger.Infof("RBFT sync state response timeout = %v", tm.getTimeoutValue(syncStateRspTimer))
	tm.logger.Infof("RBFT sync state restart timeout = %v", tm.getTimeoutValue(syncStateRestartTimer))
}

// softStartHighWatermarkTimer starts a high-watermark timer to get the latest stable checkpoint, in following conditions:
// 1) primary is trying to send pre-prepare out of range
// 2) replica received pre-prepare out of range
// 3) replica is trying to send checkpoint equal to high-watermark
// 4) replica received f+1 checkpoints out of range, but it has already executed blocks larger than these checkpoints
func (rbft *rbftImpl[T, Constraint]) softStartHighWatermarkTimer(reason string) {
	rbft.logger.Debugf("Replica %d soft start high-watermark timer, current low-watermark is %d, reason: %s", rbft.chainConfig.SelfID, rbft.chainConfig.H, reason)

	event := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreHighWatermarkEvent,
		Event:     rbft.chainConfig.H,
	}

	hasStarted, _ := rbft.timerMgr.softStartTimerWithNewTT(highWatermarkTimer, rbft.timerMgr.getTimeoutValue(highWatermarkTimer), event)
	if hasStarted {
		rbft.logger.Debugf("Replica %d has started new view timer before", rbft.chainConfig.SelfID)
	} else {
		rbft.highWatermarkTimerReason = reason
	}
}

// stopHighWatermarkTimer stops a high-watermark timer
func (rbft *rbftImpl[T, Constraint]) stopHighWatermarkTimer() {
	rbft.logger.Debugf("Replica %d stop high-watermark timer", rbft.chainConfig.SelfID)
	rbft.timerMgr.stopTimer(highWatermarkTimer)
}

func (rbft *rbftImpl[T, Constraint]) startFetchCheckpointTimer() {
	event := &LocalEvent{
		Service:   EpochMgrService,
		EventType: FetchCheckpointEvent,
	}
	// use fetchCheckpointTimer to fetch the missing checkpoint
	rbft.timerMgr.startTimer(fetchCheckpointTimer, event)
}

func (rbft *rbftImpl[T, Constraint]) stopFetchCheckpointTimer() {
	rbft.timerMgr.stopTimer(fetchCheckpointTimer)
}

func (rbft *rbftImpl[T, Constraint]) startFetchViewTimer() {
	rbft.logger.Debugf("Replica %d start a fetchView timer", rbft.chainConfig.SelfID)
	event := &LocalEvent{
		Service:   ViewChangeService,
		EventType: FetchViewEvent,
	}
	// use fetchViewTimer to fetch the higher view
	rbft.timerMgr.startTimer(fetchViewTimer, event)
}

func (rbft *rbftImpl[T, Constraint]) stopFetchViewTimer() {
	rbft.logger.Debugf("Replica %d stop a running fetchView timer", rbft.chainConfig.SelfID)
	rbft.timerMgr.stopTimer(fetchViewTimer)
}
