package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// NFR-013: Hilvan works on modest machines. These budgets are generous for CI noise; the
// measured values on a developer laptop are ~1.4 ms and ~13 MB (plan 002, G0).
const (
	startupBudget = 50 * time.Millisecond
	memoryBudget  = 40 << 20 // bytes of peak RSS
)

func goTool() string {
	if p, err := exec.LookPath("go"); err == nil {
		return p
	}
	return filepath.Join(runtime.GOROOT(), "bin", "go")
}

func TestStartupAndMemoryBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	bin := filepath.Join(t.TempDir(), "hilvan")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, err := exec.Command(goTool(), "build", "-trimpath", "-ldflags", "-s -w", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	var times []time.Duration
	var peak int64
	for i := 0; i < 15; i++ {
		cmd := exec.Command(bin, "--version")
		cmd.Env = os.Environ()
		start := time.Now()
		out, err := cmd.Output()
		times = append(times, time.Since(start))
		if err != nil || strings.TrimSpace(string(out)) == "" {
			t.Fatalf("--version: %v %q", err, out)
		}
		peak = max(peak, maxRSS(cmd))
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	median := times[len(times)/2]
	t.Logf("hilvan --version: median %v, peak RSS %.1f MB", median, float64(peak)/(1<<20))
	if median > startupBudget {
		t.Errorf("startup %v exceeds the %v budget (NFR-013)", median, startupBudget)
	}
	if peak > memoryBudget {
		t.Errorf("peak RSS %d MB exceeds the %d MB budget (NFR-013)", peak>>20, memoryBudget>>20)
	}
}
