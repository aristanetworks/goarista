// Copyright (c) 2022 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package monitor

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/aristanetworks/glog"
)

type logsetSrv struct {
	mu      sync.Mutex
	resetTo map[string]*resetState // ongoing resets
	timer   newTimerFunc           // dependency injencted timer to avoid time.Sleep in tests
	wg      sync.WaitGroup         // used during testing to ensure we're not waiting
}

func newLogsetSrv() *logsetSrv {
	return &logsetSrv{timer: realTimer, resetTo: map[string]*resetState{}}
}

func (ls *logsetSrv) err(w http.ResponseWriter, err string, code int) {
	err = fmt.Sprintf("loglevel error: %v (code %v)", err, code)
	glog.Error(err)
	http.Error(w, err, code)
}

// ServeHTTP handles a /debug/loglevel request.
//
// It parses options from a HTTP form or from URL params.
//
// The following verbositys can be set:
// - glog: set "github.com/aristanetworks/glog" verbosity.
//
// The following options control log resetting:
//
//   - timeout: A duration (e.g. "1m") for which the log should remain set at the verbosity
//     passed in. it's safe to send multiple: if you send another request with a timeout,
//     the ongoing timeout will be cancelled but the value will be reset to the original
//     value detected by this endpoint.
//
// Here's a detailed example of timeout behavior with overlapping timeouts:
// - User wants to increase verbosity to find bug. Lets assume it starts at 0.
// - They call /debug/loglevel?glog=1&timeout=10m
// - User decides this glog verbosity is not enough, so decides to increase to 10.
// - They call /debug/loglevel?glog=10&timeout=5m
// - 5 minutes later, the loglevel will be set back to 0.
// - No further changes to verbosity occur.
func (ls *logsetSrv) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	change, err := parseLoglevelReq(r)
	if err != nil {
		ls.err(w, "could not parse form: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := ls.handle(change); err != nil {
		ls.err(w, "could not update log: "+err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(w, "OK\n")
}

func (ls *logsetSrv) handle(req loglevelReq) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	var errs []error
	for typ, change := range req.updates {
		typ := typ // capture for closure

		resetFn, err := change.Apply()
		if err != nil {
			errs = append(errs, err)
		}

		// reset logic is kept as simple as possible by always cancelling a waiting reset.
		// The reset function is carried across in cases where the resetFn was not run.
		if ongoingReset, exists := ls.resetTo[typ]; exists {
			resetFn = ongoingReset.Clear()
			delete(ls.resetTo, typ)
		}

		if !req.reset {
			continue // nothing to do
		}

		cancel := make(chan struct{})
		rt := &resetState{cancel: cancel, do: resetFn}
		ls.resetTo[typ] = rt
		ls.wg.Add(1) // waitgroup used for testing only
		go func() {
			defer ls.wg.Done()
			timer := ls.timer(req.resetTimeout)
			select {
			case <-cancel:
				if !timer.Stop() {
					<-timer.C()
				}
				return
			case <-timer.C():
				ls.mu.Lock()
				defer ls.mu.Unlock()

				// we have to check cancel again here in case we got cancelled
				// while waiting for lock
				select {
				case <-rt.cancel:
					return
				default:
				}

				resetFn()
				delete(ls.resetTo, typ) // delete so resetFn is dropped
			}
		}()
	}

	return errors.Join(errs...)
}

type resetState struct {
	cancel chan struct{}
	do     func()
}

func (r *resetState) Clear() func() {
	if r.cancel != nil {
		close(r.cancel)
	}
	old := r.do
	r.cancel = nil
	r.do = nil
	return old
}

// logUpdater applys a log verbosity change
type logUpdater interface {
	// Apply changes the verbosity to the configured value.
	//
	// Apply should return a reset function if error is nil. This should reset the verbosity
	// to the value prior to a change.
	Apply() (func(), error)
}

type glogUpdater struct {
	v glog.Level
}

func (v glogUpdater) Apply() (func(), error) {
	prev := glog.SetVGlobal(v.v)
	return func() { glog.SetVGlobal(prev) }, nil
}

const glogV = "glog"

type loglevelReq struct {
	reset        bool
	resetTimeout time.Duration         // duration change should be active
	updates      map[string]logUpdater // log type as a string -> updater to apply change
}

func parseLoglevelReq(r *http.Request) (loglevelReq, error) {
	if r.Method != http.MethodPost {
		return loglevelReq{}, errors.New("HTTP method must be POST")
	}

	if err := r.ParseForm(); err != nil {
		return loglevelReq{}, err
	}
	opts := r.Form

	ll := loglevelReq{updates: map[string]logUpdater{}}

	if timeout := opts.Get("timeout"); timeout != "" {
		w, err := time.ParseDuration(timeout)
		if err != nil {
			return loglevelReq{}, fmt.Errorf("could not parse timeout: %v", err)
		}
		if w < time.Second {
			return loglevelReq{}, errors.New("timeout too small: valid between 1s-24h")
		} else if w > (time.Hour * 24) {
			return loglevelReq{}, errors.New("timeout too large: valid between 1s-24h")
		}
		ll.resetTimeout = w
		ll.reset = true
	}

	// parse glog options
	if setGlog := opts.Get(glogV); setGlog != "" {
		v, err := strconv.Atoi(setGlog)
		if err != nil {
			return loglevelReq{}, fmt.Errorf("invalid glog argument: %v", err)
		}
		if v < 0 {
			return loglevelReq{}, fmt.Errorf("invalid glog argument: %v", err)
		}
		ll.updates[glogV] = glogUpdater{v: glog.Level(v)}
	}

	if len(ll.updates) == 0 {
		return loglevelReq{}, errors.New("empty request")
	}
	return ll, nil
}

// newTimerFunc is an interface used to mock out time behavior for unit tests.
//
// this is preferred to adding a chunky time mock dependency to goarista.
type newTimerFunc func(time.Duration) timer

type timer interface {
	C() <-chan time.Time
	Stop() bool
}

type timerImpl struct {
	*time.Timer
}

func (t timerImpl) C() <-chan time.Time {
	return t.Timer.C
}

func realTimer(d time.Duration) timer {
	return timerImpl{time.NewTimer(d)}
}
