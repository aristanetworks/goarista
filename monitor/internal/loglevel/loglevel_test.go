// Copyright (c) 2024 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package loglevel

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aristanetworks/glog"
)

func req(method string, params ...string) *http.Request {
	req := httptest.NewRequest(method, "/whatever", nil)
	q := req.URL.Query()
	for i := 0; i < len(params); i += 2 {
		q.Add(params[i], params[i+1])
	}
	req.URL.RawQuery = q.Encode()
	return req
}

func call(t *testing.T, srv *logsetSrv, req *http.Request) *http.Response {
	t.Helper()
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	t.Logf("req = %#v, resp = %q", req, string(body))
	return resp
}

func hasUpdate[T comparable](t *testing.T, req loglevelReq, key string, want *T) {
	t.Helper()
	got, ok := req.updates[key].(T)
	if !ok {
		if want == nil {
			return
		}
		t.Fatalf("hasUpdate(%q): got updates %#v, should contain %#v", key, req.updates, *want)
	}
	if want == nil {
		t.Fatalf("hasUpdate(%q): got updates %#v, should contain %#v", key, req.updates, want)
	}
	if got != *want {
		t.Fatalf("hasUpdate(%q): got updates %#v, should contain %#v", key, req.updates, *want)
	}
}

func TestRequestParsing(t *testing.T) {
	tcases := map[string]struct {
		req         *http.Request
		wantErr     string
		wantGlog    *glogUpdater
		wantVModule *vModuleUpdater
	}{
		"GET": {
			req:     req("GET"),
			wantErr: "method must be POST",
		},
		"empty POST": {
			req:     req("POST"),
			wantErr: "empty request",
		},
		"only timeout": {
			req:     req("POST", "timeout", "5m"),
			wantErr: "empty request",
		},
		"error small": {
			req:     req("POST", "timeout", ".1s"),
			wantErr: "timeout too small",
		},
		"error large": {
			req:     req("POST", "timeout", "24h1s"),
			wantErr: "timeout too large",
		},

		// glog parsing
		"invalid glog": {
			req:     req("POST", glogV, "??"),
			wantErr: "invalid glog argument",
		},
		"negative glog": {
			req:     req("POST", glogV, "-1"),
			wantErr: "invalid glog argument",
		},
		"glog": {
			req:      req("POST", glogV, "0"),
			wantGlog: &glogUpdater{v: 0},
		},
		"glog with timeout": {
			req:      req("POST", glogV, "1", "timeout", "10s"),
			wantGlog: &glogUpdater{v: 1},
		},
		"glog with 24h timeout": {
			req:      req("POST", glogV, "2", "timeout", "24h"),
			wantGlog: &glogUpdater{v: 2},
		},
		"glog with 1s timeout": {
			req:      req("POST", glogV, "3", "timeout", "1s"),
			wantGlog: &glogUpdater{v: 3},
		},

		// vmodule parsing
		"invalid vmodule": {
			req:     req("POST", glogVModule, "not valid"),
			wantErr: "invalid glog-vmodule argument",
		},
		"invalid vmodule 2": {
			req:     req("POST", glogVModule, "x="),
			wantErr: "invalid glog-vmodule argument",
		},
		"invalid vmodule 3": {
			req:     req("POST", glogVModule, "x=09,asdf"),
			wantErr: "invalid glog-vmodule argument",
		},
		"valid vmodule": {
			req:         req("POST", glogVModule, "x=09,y=100,x/y/z=0"),
			wantVModule: &vModuleUpdater{v: "x=09,y=100,x/y/z=0"},
		},
		"valid vmodule that should not work": {
			req:         req("POST", glogVModule, "invalid:;path:-_=10"),
			wantVModule: &vModuleUpdater{v: "invalid:;path:-_=10"},
		},
	}

	for name, tcase := range tcases {
		t.Run(name, func(t *testing.T) {
			req, err := parseLoglevelReq(tcase.req)
			if tcase.wantErr != "" && err == nil {
				t.Fatalf("expected error %v: got nil", tcase.wantErr)
			} else if err != nil && !strings.Contains(err.Error(), tcase.wantErr) {
				t.Fatalf("expected error to contain %q: got %q", tcase.wantErr, err.Error())
			} else if tcase.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			hasUpdate(t, req, glogV, tcase.wantGlog)
			hasUpdate(t, req, glogVModule, tcase.wantVModule)

		})
	}
}

func TestGlogLogset(t *testing.T) {
	t.Run("updater", func(t *testing.T) {
		defer glog.SetVGlobal(glog.SetVGlobal(42)) // init and reset
		updater := glogUpdater{v: glog.Level(100)}
		resetter, err := updater.Apply()
		if err != nil {
			t.Fatalf("error applying update: %v", err)
		}
		if got := glog.VGlobal(); got != 100 {
			t.Fatalf("glog verbosity should be 100, got %#v", got)
		}
		resetter()
		if got := glog.VGlobal(); got != 42 {
			t.Fatalf("glog verbosity should be 42, got %#v", got)
		}

	})

	t.Run("request", func(t *testing.T) {
		defer glog.SetVGlobal(glog.SetVGlobal(0)) // init and reset
		ls := newLogsetSrv()
		resp := call(t, ls, req("POST", glogV, "1"))
		if resp.StatusCode != 200 {
			t.Fatalf("expected status 200, wanted %v", resp.StatusCode)
		}
		if v := glog.VGlobal(); v != 1 {
			t.Fatalf("expected glog %v, got %v", v, 1)
		}
	})
}

func TestVModuleSet(t *testing.T) {
	t.Run("updater", func(t *testing.T) {
		// reset vmodule at end of test
		prev, err := glog.SetVModule("prev=99")
		if err != nil {
			t.Fatalf("vmodule call failed: %v", err)
		}
		defer glog.SetVModule(prev)
		updater := vModuleUpdater{v: "next=100"}
		resetter, err := updater.Apply()
		if err != nil {
			t.Fatalf("error applying vmodule update: %v", err)
		}
		if got := glog.VModule(); got != "next=100" {
			t.Fatalf("vmodule should be 'next=100', got %#v", got)
		}
		resetter()
		if got := glog.VModule(); got != "prev=99" {
			t.Fatalf("vmodule should be 'prev=99', got %#v", got)
		}

	})
	t.Run("request", func(t *testing.T) {
		// reset vmodule at end of test
		prev, err := glog.SetVModule("")
		if err != nil {
			t.Fatalf("vmodule call failed: %v", err)
		}
		defer glog.SetVModule(prev)
		// make request
		ls := newLogsetSrv()
		resp := call(t, ls, req("POST", glogVModule, "y=001"))
		if resp.StatusCode != 200 {
			t.Fatalf("expected status 200, wanted %v", resp.StatusCode)
		}
		if v := glog.VModule(); v != "y=1" {
			t.Fatalf("expected vmodule x=001, got %q", v)
		}
	})
}

type mockedRequest struct {
	timerCreated   chan time.Duration // recevies the time Duration when timer is created
	timerTrigger   chan time.Time     // channel that you can send a time to trigger timer
	timerCancelled chan struct{}      // will be closed if timer is cancelled
	logApplied     chan struct{}      // request has been applied when this channel is closed
	logReset       chan struct{}      // request has been reset when this channeel is closed
}

// Apply implements mockLogUpdater
func (m mockedRequest) Apply() (func(), error) {
	close(m.logApplied)
	return func() {
		close(m.logReset)
	}, nil
}

type mockTimerImpl struct {
	c chan time.Time
}

func (m *mockTimerImpl) C() <-chan time.Time {
	return m.c
}

func (m *mockTimerImpl) Stop() bool {
	return true // returning true means we won't try and drain the channel; thats okay
}

func newMockedRequest(t *testing.T, ls *logsetSrv, opts ...string) mockedRequest {
	m := mockedRequest{
		timerCreated: make(chan time.Duration, 1),
		timerTrigger: make(chan time.Time),
		logApplied:   make(chan struct{}),
		logReset:     make(chan struct{}),
	}
	// setup dependency injected timer
	newTimer := func(d time.Duration) timer {
		m.timerCreated <- d
		return &mockTimerImpl{c: m.timerTrigger}
	}

	ls.mu.Lock()
	ls.timer = newTimer // has to be set on each new request
	ls.mu.Unlock()

	args := []string{glogV, "1"}
	args = append(args, opts...)
	req := req("POST", args...)
	request, err := parseLoglevelReq(req)
	if err != nil {
		t.Fatalf("could not create glog request: %v", err)
	}
	request.updates[glogV] = m

	t.Log("send logrequest", opts)
	if err := ls.handle(request); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ls.mu.Lock()
	if v, exists := ls.resetTo[glogV]; exists {
		m.timerCancelled = v.cancel
	}
	ls.mu.Unlock()
	return m
}

func TestResetBehavior(t *testing.T) {
	t.Run("reset is called", func(t *testing.T) {
		ls := newLogsetSrv()
		req := newMockedRequest(t, ls, "timeout", "1s")
		<-req.logApplied
		if timerD := <-req.timerCreated; timerD != time.Second {
			t.Fatalf("expected timer to be set for %v, got %v", time.Second, timerD)
		}

		req.timerTrigger <- time.Time{}
		<-req.logReset

		t.Log("wait until all ongoing resets are done")
		ls.wg.Wait()
	})

	t.Run("updating loglevel twice with no overlapping timeout works", func(t *testing.T) {
		ls := newLogsetSrv()

		req1 := newMockedRequest(t, ls, "timeout", "33s")
		<-req1.logApplied
		if timerD := <-req1.timerCreated; timerD != time.Second*33 {
			t.Fatalf("expected timer to be set for %v, got %v", time.Second*33, timerD)
		}

		req1.timerTrigger <- time.Time{}
		t.Log("request 1 should be reset after timer is triggered")
		<-req1.logReset

		req2 := newMockedRequest(t, ls, "timeout", "45s")
		<-req2.logApplied
		if timerD := <-req2.timerCreated; timerD != 45*time.Second {
			t.Fatalf("expected timer to be set for %v, got %v", 45*time.Second, timerD)
		}
		req2.timerTrigger <- time.Time{}
		t.Log("request 2 should be reset after timer is triggered")
		<-req2.logReset

		t.Log("wait until all ongoing resets are done")
		ls.wg.Wait()
	})

	t.Run("updating loglevel with overlapping timeout behaves correctly", func(t *testing.T) {
		ls := newLogsetSrv()

		req1 := newMockedRequest(t, ls, "timeout", "10s")
		<-req1.logApplied
		if timerD := <-req1.timerCreated; timerD != time.Second*10 {
			t.Fatalf("expected timer to be set for %v, got %v", time.Second*10, timerD)
		}

		req2 := newMockedRequest(t, ls, "timeout", "100s")
		<-req2.logApplied
		if timerD := <-req2.timerCreated; timerD != 100*time.Second {
			t.Fatalf("expected timer to be set for %v, got %v", 100*time.Second, timerD)
		}

		t.Log("timer1 should be cancelled")
		<-req1.timerCancelled
		t.Log("timer should be triggered")
		select {
		case req1.timerTrigger <- time.Time{}:
			t.Fatal("first request timer goroutine should not be running after cancellation")
		default:
		}

		t.Log("triggering timer 2 should call the original reset function")
		req2.timerTrigger <- time.Time{}
		<-req1.logReset

		t.Log("wait until all ongoing resets are done")
		ls.wg.Wait()
		select {
		case <-req2.logReset:
			t.Fatal("should not call second reset function")
		default:
		}
	})

	t.Run("updating loglevel clears waiting reset timers", func(t *testing.T) {
		ls := newLogsetSrv()

		req1 := newMockedRequest(t, ls, "timeout", "11s")
		<-req1.logApplied
		if timerD := <-req1.timerCreated; timerD != time.Second*11 {
			t.Fatalf("expected timer to be set for %v, got %v", time.Second*11, timerD)
		}

		req2 := newMockedRequest(t, ls)
		<-req2.logApplied

		t.Log("timer 1 should be cancelled")
		<-req1.timerCancelled

		req3 := newMockedRequest(t, ls, "timeout", "12s")
		<-req3.logApplied

		if timerD := <-req3.timerCreated; timerD != time.Second*12 {
			t.Fatalf("expected timer to be set for %v, got %v", time.Second*12, timerD)
		}

		t.Log("triggering timer 3 should call the new reset function")
		req3.timerTrigger <- time.Time{}
		<-req3.logReset

		t.Log("wait until all ongoing resets are done")
		ls.wg.Wait()

		select {
		case req1.timerTrigger <- time.Time{}:
			t.Fatal("first request timer goroutine should not be running after cancellation")
		default:
		}

		t.Log("timer 2 should never be created")
		select {
		case <-req2.timerCreated:
			t.Fatal("timer should never be created on request with no timeout set")
		default:
		}

		select {
		case req2.timerTrigger <- time.Time{}:
			t.Fatal("second request timer should not be able to receive a time")
		default:
		}

		t.Log("ensure we did not call either reset function")
		select {
		case <-req1.logReset:
			t.Fatal("should not call first reset function")
		case <-req2.logReset:
			t.Fatal("should not call second reset function")
		default:
		}
	})
}
