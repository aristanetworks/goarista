// Copyright (c) 2024 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

// Package loglevel exposes a HTTP handler to dynamically update log levels on a timer.
//
// Calling GET on this handler will return a html form which can be used to input params.
// The handler accepts URL or Form params which are documented when calling GET on this
// endpoint.
//
// The following verbositys can be set:
//
//   - glog: set "github.com/aristanetworks/glog" verbosity.
//   - glog-vmodule: set "github.com/aristanetworks/glog" verbosity on a per function basis.
//
// The following options control log resetting:
//   - timeout: A duration (e.g. "1m") for which the log should remain set at the verbosity
//     passed in. it's safe to send multiple: if you send another request with a timeout,
//     the ongoing timeout will be cancelled but the value will be reset to the original
//     value detected by this endpoint.
//
// This timeout logic is nuanced to handle cases where multiple updates are performed
// on the log at once. We allow timeout to change, but the original verbosity is preserved.
// See the following description:
//
//   - User wants to increase verbosity to find bug. Lets assume it starts at 0.
//   - They call /debug/loglevel?glog=1&timeout=10m
//   - User decides this glog verbosity is not enough, so decides to increase to 10.
//   - They call /debug/loglevel?glog=10&timeout=5m
//   - We update the verbosity to 10, but instead of reseting to 1 we keep the original
//     reset value from the first log level.
//   - After 5 mins, we reset to 0.
//
// Note that if you frequently change verbosity externally to loglevel, or run multiple loglevel
// handlers, you can introduce race conditions.
package loglevel

import (
	"embed"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"text/template"
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

// Handler returns a http handler for the loglevel request. See package docs.
func Handler() http.Handler {
	return newLogsetSrv()
}

func (ls *logsetSrv) err(w http.ResponseWriter, err string, code int) {
	err = fmt.Sprintf("loglevel error: %v (code %v)", err, code)
	glog.Error(err)
	http.Error(w, err, code)
}

// ServeHTTP serves the loglevel request.
//
// It parses options from a HTTP form or from URL params.
//
// See the package level docs or the self hosted documentation for a high level
// overview of the API features.
func (ls *logsetSrv) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// serve page to configure log level
		ls.form(w, r)
		return
	}

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
			continue
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

				if resetFn == nil {
					glog.Error("log reset error: nothing to reset to")
				} else {
					resetFn()
				}
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

type vModuleUpdater struct {
	v string
}

func (v vModuleUpdater) Apply() (func(), error) {
	prev, err := glog.SetVModule(v.v)
	if err != nil {
		return nil, err
	}
	return func() {
		// nothing we can do if we error now, just log it
		if _, err := glog.SetVModule(prev); err != nil {
			glog.Errorf("loglevel: cannot reset VModule: %v", err)
		}
	}, nil
}

type glogRateLimitUpdater struct {
	every time.Duration
	burst int
}

func (v glogRateLimitUpdater) Apply() (func(), error) {
	prevLimit, prevBurst := glog.SetRateLimit(v.every, v.burst)
	return func() { glog.SetRateLimit(prevLimit, prevBurst) }, nil
}

type noopUpdater struct{}

func (v noopUpdater) Apply() (func(), error) {
	return func() {}, nil
}

type loglevelReq struct {
	reset        bool
	resetTimeout time.Duration         // duration change should be active
	updates      map[string]logUpdater // log type as a string -> updater to apply change
}

const glogV = "glog"
const glogVModule = "glog-vmodule"
const noopUpdate = "noop"         // useful for test
const glogRateLimit = "glog-rate" // note: this is populated internally and not in the API

func parseLoglevelReq(r *http.Request) (loglevelReq, error) {
	if r.Method != http.MethodPost {
		return loglevelReq{}, errors.New("HTTP method must be POST")
	}

	if err := r.ParseForm(); err != nil {
		return loglevelReq{}, err
	}
	opts := r.Form

	ll := loglevelReq{updates: map[string]logUpdater{}}
	// parse glog options
	if setGlog := opts.Get(glogV); setGlog != "" {
		v, err := strconv.Atoi(setGlog)
		if err != nil {
			return loglevelReq{}, fmt.Errorf("invalid glog argument: %v", err)
		}
		if v < 0 {
			return loglevelReq{}, fmt.Errorf("invalid glog verbosity: %d", v)
		}

		ll.updates[glogV] = glogUpdater{v: glog.Level(v)}
	}

	if vmod := opts.Get(glogVModule); vmod != "" {
		// If vmod is invalid, user will see error from Apply function, so no parsing
		// required.
		ll.updates[glogVModule] = vModuleUpdater{v: vmod}
	}

	if noop := opts.Get(noopUpdate); noop != "" {
		ll.updates[noopUpdate] = noopUpdater{}
	}

	// this check must happen before we parse timeout
	if len(ll.updates) == 0 {
		return loglevelReq{}, errors.New("empty request")
	}

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

		// Raise glog rate limit when the reset is going to be less than 5 mins.
		//
		// Note that there is some more complexity when we enter this state: if somebody comes
		// along and updates glog without a timeout before the reset fires, that will remove
		// the original reset. But that only happens for glog, and our rate limit reset will
		// kick in eventually.
		if _, ok := ll.updates[glogV]; ok && w <= 5*time.Minute {
			duration, burst := glog.GetRateLimit()
			var doubleRateLimit = glogRateLimitUpdater{
				every: duration / 2, burst: burst * 2}
			ll.updates[glogRateLimit] = doubleRateLimit
		}

		ll.resetTimeout = w
		ll.reset = true
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

//go:embed form.html.tmpl
var loglevelTmpl embed.FS
var loglevelForm *template.Template

func init() {
	loglevelForm = template.Must(template.ParseFS(loglevelTmpl, "form.html.tmpl"))
}

type loglevelTmplArgs struct {
	Path        string
	GlogV       glog.Level
	GlogVModule string
}

func (ls *logsetSrv) form(w http.ResponseWriter, r *http.Request) {
	args := loglevelTmplArgs{
		Path:        r.URL.Path,
		GlogV:       glog.VGlobal(),
		GlogVModule: glog.VModule(),
	}
	if err := loglevelForm.Execute(w, args); err != nil {
		ls.err(w, "could not write form: "+err.Error(), http.StatusInternalServerError)
	}
}
