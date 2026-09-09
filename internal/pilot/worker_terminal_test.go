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
	"testing"
)

func TestWorkerCompleteIsNotForwardedBeforeExecutionConverges(t *testing.T) {
	store := NewMemorySkillExecutionStore()
	execution := SkillExecution{
		ID:      "skill-worker-complete",
		RobotID: "robot-worker-complete",
		Status:  SkillRunning,
	}
	if err := store.SaveSkillExecution(execution); err != nil {
		t.Fatal(err)
	}
	events := &recordingRuntimeEventSink{}
	runtime := NewSkillRuntime(nil, nil, nil, store, WorkerSupervisor{}, nil, nil, events)

	if _, err := runtime.handleWorker(context.Background(), execution.ID, nil, "complete",
		map[string]any{"status": "completed"}); err != nil {
		t.Fatal(err)
	}
	if len(events.events) != 0 {
		t.Fatalf("Worker complete must not be forwarded with stale execution state: %#v", events.events)
	}
}
