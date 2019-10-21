package rbft

import (
	"sync"
	"testing"
	"time"

	mockexternal "github.com/ultramesh/flato-rbft/mock/mock_external"
	txpoolmock "github.com/ultramesh/flato-txpool/mock"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

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
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	tx := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:            1,
		IsNew:         false,
		Peers:         peerSet,
		Logger:        log,
		External:      external,
		RequestPool:   tx,
		K:             2,
		LogMultiplier: 2,
	}
	eventC := make(chan interface{})
	timeMgr := newTimerMgr(eventC, conf)

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
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	tx := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:            1,
		IsNew:         false,
		Peers:         peerSet,
		Logger:        log,
		External:      external,
		RequestPool:   tx,
		K:             2,
		LogMultiplier: 2,
	}
	eventC := make(chan interface{})
	timeMgr := newTimerMgr(eventC, conf)

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
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	tx := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:            1,
		IsNew:         false,
		Peers:         peerSet,
		Logger:        log,
		External:      external,
		RequestPool:   tx,
		K:             2,
		LogMultiplier: 2,
	}
	eventC := make(chan interface{})
	timeMgr := newTimerMgr(eventC, conf)

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
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	tx := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:            1,
		IsNew:         false,
		Peers:         peerSet,
		Logger:        log,
		External:      external,
		RequestPool:   tx,
		K:             2,
		LogMultiplier: 2,
	}
	eventC := make(chan interface{})
	timeMgr := newTimerMgr(eventC, conf)

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
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	tx := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:            1,
		IsNew:         false,
		Peers:         peerSet,
		Logger:        log,
		External:      external,
		RequestPool:   tx,
		K:             2,
		LogMultiplier: 2,
	}
	eventC := make(chan interface{})
	timeMgr := newTimerMgr(eventC, conf)

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
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	tx := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:            1,
		IsNew:         false,
		Peers:         peerSet,
		Logger:        log,
		External:      external,
		RequestPool:   tx,
		K:             2,
		LogMultiplier: 2,
	}
	eventC := make(chan interface{})
	timeMgr := newTimerMgr(eventC, conf)

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
	log := NewRawLogger()
	external := mockexternal.NewMockMinimalExternal(ctrl)
	tx := txpoolmock.NewMockMinimalTxPool(ctrl)

	conf := Config{
		ID:            1,
		IsNew:         false,
		Peers:         peerSet,
		Logger:        log,
		External:      external,
		RequestPool:   tx,
		K:             2,
		LogMultiplier: 2,
	}
	eventC := make(chan interface{})
	timeMgr := newTimerMgr(eventC, conf)

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
