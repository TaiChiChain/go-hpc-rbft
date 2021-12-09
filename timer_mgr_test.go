package rbft

import (
	"sync"
	"testing"
	"time"

	"github.com/ultramesh/flato-common/metrics/disabled"
	mockexternal "github.com/ultramesh/flato-rbft/mock/mock_external"
	txpoolmock "github.com/ultramesh/flato-txpool/mock"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func newTestTimerMgr(ctrl *gomock.Controller) *timerManager {
	log := FrameworkNewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	tx := txpoolmock.NewMockMinimalTxPool(ctrl)
	conf := Config{
		ID:            1,
		Peers:         peerSet,
		Logger:        log,
		External:      external,
		RequestPool:   tx,
		K:             2,
		LogMultiplier: 2,
		MetricsProv:   &disabled.Provider{},
		DelFlag:       make(chan bool),
	}
	eventC := make(chan interface{})

	return newTimerMgr(eventC, conf)
}

func TestTimerMgr_store(t *testing.T) {
	tt := titleTimer{
		timerName: "test timer",
		timeout:   5,
		isActive:  sync.Map{},
	}
	tt.store("key1", "value1")
	v, _ := tt.isActive.Load("key1")
	assert.Equal(t, "value1", v)
}

func TestTimerMgr_delete(t *testing.T) {
	tt := titleTimer{
		timerName: "test timer",
		timeout:   5,
		isActive:  sync.Map{},
	}
	tt.store("key1", "value1")
	tt.delete("key1")
	v, _ := tt.isActive.Load("key1")
	assert.Equal(t, nil, v)
}

func TestTimerMgr_count(t *testing.T) {
	tt := titleTimer{
		timerName: "test timer",
		timeout:   5,
		isActive:  sync.Map{},
	}
	tt.store("key1", "value1")
	assert.Equal(t, 1, tt.count())
}

func TestTimerMgr_stopTimer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	timeMgr := newTestTimerMgr(ctrl)

	tt := titleTimer{
		timerName: "test timer",
		timeout:   5,
		isActive:  sync.Map{},
	}
	tt.store("key1", "value1")
	timeMgr.tTimers["test1"] = &tt

	timeMgr.stopTimer("null timerMgr")
	assert.Equal(t, 1, timeMgr.tTimers["test1"].count())

	timeMgr.stopTimer("test1")
	assert.Equal(t, 0, timeMgr.tTimers["test1"].count())
}

func TestTimerMgr_getTimeoutValue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	timeMgr := newTestTimerMgr(ctrl)

	tt := titleTimer{
		timerName: "test timer",
		timeout:   5,
		isActive:  sync.Map{},
	}
	tt.store("key1", "value1")
	tt.store("key2", "value2")
	timeMgr.tTimers["test1"] = &tt

	timeMgr.stopOneTimer("null timerMgr", "key1")
	assert.Equal(t, true, timeMgr.tTimers["test1"].has("key1"))

	timeMgr.stopOneTimer("test1", "key1")
	assert.Equal(t, false, timeMgr.tTimers["test1"].has("key1"))
}

func TestTimerMgr_stopOneTimer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	timeMgr := newTestTimerMgr(ctrl)

	tt := titleTimer{
		timerName: "test timer",
		timeout:   5,
		isActive:  sync.Map{},
	}
	tt.store("key1", "value1")
	tt.store("key2", "value2")
	timeMgr.tTimers["test1"] = &tt

	assert.Equal(t, time.Duration(0), timeMgr.getTimeoutValue("null timerMgr"))
	assert.Equal(t, time.Duration(5), timeMgr.getTimeoutValue("test1"))
}

func TestTimerMgr_setTimeoutValue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	timeMgr := newTestTimerMgr(ctrl)

	tt := titleTimer{
		timerName: "test timer",
		timeout:   5,
		isActive:  sync.Map{},
	}
	tt.store("key1", "value1")
	tt.store("key2", "value2")
	timeMgr.tTimers["test1"] = &tt

	timeMgr.setTimeoutValue("null", time.Duration(10))
	assert.Equal(t, time.Duration(5), timeMgr.getTimeoutValue("test1"))
	timeMgr.setTimeoutValue("test1", time.Duration(10))
	assert.Equal(t, time.Duration(10), timeMgr.getTimeoutValue("test1"))
}

func TestTimerMgr_makeNullRequestTimeoutLegal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	timeMgr := newTestTimerMgr(ctrl)

	tt := titleTimer{
		timerName: "test timer",
		timeout:   5,
		isActive:  sync.Map{},
	}
	timeMgr.tTimers[nullRequestTimer] = &tt
	timeMgr.tTimers[requestTimer] = &tt

	timeMgr.setTimeoutValue(nullRequestTimer, time.Duration(5))
	timeMgr.setTimeoutValue(requestTimer, time.Duration(10))
	timeMgr.makeNullRequestTimeoutLegal()
	assert.Equal(t, time.Duration(15), timeMgr.getTimeoutValue(nullRequestTimer))
}

func TestTimerMgr_makeCleanVcTimeoutLegal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	timeMgr := newTestTimerMgr(ctrl)

	tt := titleTimer{
		timerName: "test timer",
		timeout:   5,
		isActive:  sync.Map{},
	}
	timeMgr.tTimers[cleanViewChangeTimer] = &tt
	timeMgr.tTimers[newViewTimer] = &tt

	timeMgr.setTimeoutValue(cleanViewChangeTimer, time.Duration(5))
	timeMgr.setTimeoutValue(newViewTimer, time.Duration(1))
	timeMgr.makeCleanVcTimeoutLegal()
	assert.Equal(t, time.Duration(6), timeMgr.getTimeoutValue(cleanViewChangeTimer))
}

func TestTimerMgr_makeSyncStateTimeoutLegal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	timeMgr := newTestTimerMgr(ctrl)

	tt := titleTimer{
		timerName: "test timer",
		timeout:   5,
		isActive:  sync.Map{},
	}
	timeMgr.tTimers[syncStateRspTimer] = &tt
	timeMgr.tTimers[syncStateRestartTimer] = &tt

	timeMgr.setTimeoutValue(syncStateRspTimer, time.Duration(5))
	timeMgr.setTimeoutValue(syncStateRestartTimer, time.Duration(1))
	timeMgr.makeSyncStateTimeoutLegal()
	assert.Equal(t, time.Duration(10), timeMgr.getTimeoutValue(syncStateRspTimer))
}

func TestTimerMgr_newTimer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	timeMgr := newTestTimerMgr(ctrl)

	timeMgr.newTimer(requestTimer, 0)
	timeMgr.newTimer(batchTimer, 0)
	timeMgr.newTimer(vcResendTimer, 0)
	timeMgr.newTimer(newViewTimer, 0)
	timeMgr.newTimer(nullRequestTimer, 0)
	timeMgr.newTimer(firstRequestTimer, 0)
	timeMgr.newTimer(syncStateRspTimer, 0)
	timeMgr.newTimer(syncStateRestartTimer, 0)
	timeMgr.newTimer(recoveryRestartTimer, 0)
	timeMgr.newTimer(cleanViewChangeTimer, 0)
	timeMgr.newTimer(checkPoolTimer, 0)
	timeMgr.newTimer(fetchCheckpointTimer, 0)
	assert.Equal(t, DefaultRequestTimeout, timeMgr.tTimers[requestTimer].timeout)
	assert.Equal(t, DefaultBatchTimeout, timeMgr.tTimers[batchTimer].timeout)
	assert.Equal(t, DefaultVcResendTimeout, timeMgr.tTimers[vcResendTimer].timeout)
	assert.Equal(t, DefaultNewViewTimeout, timeMgr.tTimers[newViewTimer].timeout)
	assert.Equal(t, DefaultNullRequestTimeout, timeMgr.tTimers[nullRequestTimer].timeout)
	assert.Equal(t, DefaultFirstRequestTimeout, timeMgr.tTimers[firstRequestTimer].timeout)
	assert.Equal(t, DefaultSyncStateRspTimeout, timeMgr.tTimers[syncStateRspTimer].timeout)
	assert.Equal(t, DefaultSyncStateRestartTimeout, timeMgr.tTimers[syncStateRestartTimer].timeout)
	assert.Equal(t, DefaultRecoveryRestartTimeout, timeMgr.tTimers[recoveryRestartTimer].timeout)
	assert.Equal(t, DefaultCleanViewChangeTimeout, timeMgr.tTimers[cleanViewChangeTimer].timeout)
	assert.Equal(t, DefaultCheckPoolTimeout, timeMgr.tTimers[checkPoolTimer].timeout)
	assert.Equal(t, DefaultFetchCheckpointTimeout, timeMgr.tTimers[fetchCheckpointTimer].timeout)
}
