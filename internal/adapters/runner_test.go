package adapters

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ferchosur/hilvan/internal/launchers"
	"github.com/ferchosur/hilvan/internal/process"
	"github.com/ferchosur/hilvan/internal/testutil"
)

// The test binary doubles as a fake agent CLI (testutil.FakeAgent).
func TestMain(m *testing.M) {
	testutil.RunFakeAgentIfRequested()
	os.Exit(m.Run())
}

// FakeInvocation runs the fake agent with a scenario.
func FakeInvocation(scenario string) launchers.Invocation {
	return launchers.Invocation{Command: os.Args[0], Env: testutil.FakeAgentEnv(process.CurrentEnv(), scenario)}
}

func run(t *testing.T, scenario string, opts RunOptions) (*RunHandle, *[]Event) {
	t.Helper()
	events := &[]Event{}
	h, err := StartRun(FakeInvocation(scenario), "hello prompt", ParseClaudeLine, func(e Event, _ string) { *events = append(*events, e) }, opts)
	if err != nil {
		t.Fatal(err)
	}
	return h, events
}

func TestRunnerCompletes(t *testing.T) {
	h, events := run(t, "success", RunOptions{})
	r := h.Wait()
	if r.State != "completed" || r.CostUSD == nil || *r.CostUSD != 0.0123 {
		t.Fatalf("outcome %+v", r.Map())
	}
	found := false
	for _, e := range *events {
		found = found || (e.Type == EvMessage && strings.HasPrefix(e.Text, "got 12 chars"))
	}
	if !found {
		t.Error("the prompt did not reach the agent through stdin")
	}
}

func TestRunnerBlocks(t *testing.T) {
	for scenario, reason := range map[string]string{"limit": "rate_limit", "auth": "auth"} {
		h, _ := run(t, scenario, RunOptions{})
		if r := h.Wait(); r.State != "blocked" || r.BlockedReason != reason {
			t.Errorf("%s: %+v", scenario, r.Map())
		}
	}
}

func TestRunnerCrashIsUncertain(t *testing.T) {
	h, _ := run(t, "crash", RunOptions{})
	if r := h.Wait(); r.State != "uncertain" || r.ExitCode != 3 {
		t.Errorf("crash: %+v exit %d", r.Map(), r.ExitCode)
	}
}

func TestRunnerCancel(t *testing.T) {
	h, _ := run(t, "slow", RunOptions{Grace: 2 * time.Second})
	time.AfterFunc(300*time.Millisecond, func() { h.Cancel("") })
	if r := h.Wait(); r.State != "cancelled" {
		t.Errorf("cancel: %+v", r.Map())
	}
}

func TestRunnerOutputLimit(t *testing.T) {
	h, _ := run(t, "huge", RunOptions{MaxLine: 1024 * 1024})
	r := h.Wait()
	if r.State != "uncertain" || r.Error == nil || !strings.Contains(*r.Error, "output limit") {
		t.Errorf("SR-009: %+v", r.Map())
	}
}

func TestRunnerTimeout(t *testing.T) {
	h, _ := run(t, "slow", RunOptions{Timeout: 300 * time.Millisecond, Grace: 200 * time.Millisecond})
	r := h.Wait()
	if r.State != "cancelled" || r.Error == nil || *r.Error != "timeout" {
		t.Errorf("SR-025: %+v", r.Map())
	}
}

// The watchdog cancels a run that goes silent after its last event (the "slow" scenario).
func TestRunnerInactivity(t *testing.T) {
	h, _ := run(t, "slow", RunOptions{Inactivity: 300 * time.Millisecond, Grace: 200 * time.Millisecond})
	r := h.Wait()
	if r.State != "cancelled" || r.Error == nil || *r.Error != "inactivity" {
		t.Errorf("watchdog: %+v", r.Map())
	}
}

// Output restarts the countdown: a run that talks every 150 ms survives a 400 ms limit.
func TestRunnerInactivityResetByOutput(t *testing.T) {
	h, _ := run(t, "chatty", RunOptions{Inactivity: 400 * time.Millisecond, Grace: 200 * time.Millisecond})
	if r := h.Wait(); r.State != "completed" {
		t.Errorf("live run was cut off: %+v", r.Map())
	}
}

// The silence measurement follows the output: the 150 ms gaps of "chatty" show up as the maximum.
func TestRunnerMeasuresSilence(t *testing.T) {
	h, _ := run(t, "chatty", RunOptions{})
	r := h.Wait()
	if r.MaxSilence < 100*time.Millisecond || r.MaxSilence > 2*time.Second {
		t.Errorf("max silence %v, want about 150 ms", r.MaxSilence)
	}
}

func TestRunnerMissingExecutable(t *testing.T) {
	h, err := StartRun(launchers.Invocation{Command: filepath.Join(t.TempDir(), "agent")}, "", ParseClaudeLine, func(Event, string) {}, RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r := h.Wait(); r.State != "failed" {
		t.Errorf("missing executable: %+v", r.Map())
	}
}

func TestRunnerRejectsUnsafeArguments(t *testing.T) {
	inv := FakeInvocation("success")
	inv.Args = []string{"a\nb"}
	if _, err := StartRun(inv, "", ParseClaudeLine, func(Event, string) {}, RunOptions{}); err == nil {
		t.Error("newline argument accepted")
	}
}
