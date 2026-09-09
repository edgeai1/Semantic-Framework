// Copyright 2026 InsightOS
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pilot

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestWorkerStdoutEOFFailsPendingCall(t *testing.T) {
	response := make(chan rpcMessage, 1)
	process := &WorkerProcess{
		pending:  map[string]chan rpcMessage{"pilot-1": response},
		requests: make(chan rpcMessage, 1),
	}

	process.readStdout(strings.NewReader(""))

	select {
	case message := <-response:
		if !strings.Contains(stringValue(message.Error["message"]), "stdout closed") {
			t.Fatalf("unexpected EOF error: %#v", message.Error)
		}
	case <-time.After(time.Second):
		t.Fatal("stdout EOF did not wake the pending JSON-RPC call")
	}
}

func TestWorkerResponseWaitsForPreviouslyReadRequests(t *testing.T) {
	process := &WorkerProcess{
		done:            make(chan struct{}),
		requestProgress: make(chan struct{}, 1),
	}
	process.requestQueued.Store(2)

	finished := make(chan error, 1)
	go func() {
		finished <- process.waitRequestsHandled(context.Background(), 2)
	}()

	process.markRequestHandled(1)
	select {
	case err := <-finished:
		t.Fatalf("barrier returned before all requests were handled: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	process.markRequestHandled(2)
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("barrier returned an error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("barrier did not unblock after the final request")
	}
}

func TestWorkerStdoutEOFWithoutPendingCallIsNormal(t *testing.T) {
	process := &WorkerProcess{
		pending:  make(map[string]chan rpcMessage),
		requests: make(chan rpcMessage, 1),
	}

	process.readStdout(strings.NewReader(""))

	process.waitErrMu.RLock()
	err := process.waitErr
	process.waitErrMu.RUnlock()
	if err != nil {
		t.Fatalf("normal stdout EOF was recorded as worker failure: %v", err)
	}
}
