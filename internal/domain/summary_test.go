package domain

import (
	"strings"
	"testing"
)

// summaryIssues returns only the validation issues about the run summary.
func summaryIssues(task Task) Issues {
	var out Issues
	for _, is := range task.Validate() {
		if strings.Contains(is.Path, "summary") {
			out = append(out, is)
		}
	}
	return out
}

// A long research answer must still be a valid run record. The record limit used to be 4000
// characters while the core kept 50 000, so saving a long answer failed (2026-10-13).
func TestLongRunSummaryIsValid(t *testing.T) {
	task := Task{ID: "tsk_01JABCDEFGHJKMNPQRSTVWXY01", Title: "x", Classification: "personal", Mode: "read", State: TaskInReview,
		Runs: []RunSummary{{ID: "run_01JABCDEFGHJKMNPQRSTVWXYZ0", ProfileID: "p", State: RunCompleted,
			Summary: strings.Repeat("a", MaxRunSummary), StartedAt: "2026-10-13T00:00:00.000Z"}}}
	if issues := summaryIssues(task); len(issues) > 0 {
		t.Errorf("a summary of %d characters is rejected: %v", MaxRunSummary, issues)
	}
	task.Runs[0].Summary = strings.Repeat("a", MaxRunSummary+1)
	if issues := summaryIssues(task); len(issues) == 0 {
		t.Error("a summary above the limit must be rejected")
	}
}
