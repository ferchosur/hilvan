package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/gitops"
	"github.com/ferchosur/hilvan/internal/policy"
	"github.com/ferchosur/hilvan/internal/security"
	"github.com/ferchosur/hilvan/internal/store"
)

// M4 — continuity package between profiles (FR-003, FR-004, FR-005, NFR-002, NFR-003).
// Built mechanically from what Hilvan observed, so it works even when the source profile is
// blocked by a usage limit. Agent claims are kept apart as unverified.

// HandoffError is a user-facing refusal of a hand-off.
type HandoffError struct{ Msg string }

func (e *HandoffError) Error() string { return e.Msg }

// HandoffDraft is a package waiting for the user's approval.
type HandoffDraft struct {
	ID         string
	Task       domain.Task
	From       string
	To         *domain.Profile
	Evaluation policy.Evaluation
	Body       string
	// Secrets found (and already redacted) while building the draft.
	Secrets []security.SecretMatch
}

func section(title string, lines []string) string {
	body := "_(sin datos)_"
	if len(lines) > 0 {
		body = strings.Join(lines, "\n")
	}
	return "## " + title + "\n\n" + body + "\n"
}

func runLine(r domain.RunSummary) string {
	at := r.StartedAt
	if len(at) > 16 {
		at = at[:16]
	}
	bits := []string{strings.Replace(at, "T", " ", 1) + " · " + r.ProfileID + " · " + r.State}
	if r.BlockedReason != "" {
		bits = append(bits, "("+r.BlockedReason+")")
	}
	if r.Diff != nil {
		bits = append(bits, fmt.Sprintf("· %d archivos +%d −%d", r.Diff.FilesChanged, r.Diff.LinesAdded, r.Diff.LinesDeleted))
	}
	if r.Risk != nil {
		bits = append(bits, "· riesgo "+jsNum(*r.Risk)+"/10")
	}
	return "- " + strings.Join(bits, " ")
}

func gitFacts(s *store.ProjectStore, t domain.Task, git gitops.Git) (commits, files []string) {
	if t.Branch == nil || t.BaseBranch == nil {
		return nil, nil
	}
	lines := func(out string) []string {
		var l []string
		for _, x := range strings.Split(strings.TrimSpace(out), "\n") {
			if x != "" {
				l = append(l, x)
			}
		}
		return l
	}
	if log, err := git([]string{"log", "--oneline", "--no-decorate", "-n", "30", *t.BaseBranch + ".." + *t.Branch}, s.Root); err == nil && log.Code == 0 {
		commits = lines(log.Stdout)
	}
	if diff, err := git([]string{"diff", "--name-status", *t.BaseBranch + "..." + *t.Branch}, s.Root); err == nil && diff.Code == 0 {
		files = lines(diff.Stdout)
		if len(files) > 200 {
			files = files[:200]
		}
	}
	return commits, files
}

// DraftHandoff builds the package and evaluates the transfer against the policy first.
func DraftHandoff(s *store.ProjectStore, taskID string, to, from *domain.Profile, pol domain.EffectivePolicy, git gitops.Git) (HandoffDraft, error) {
	if git == nil {
		git = gitops.DefaultGit
	}
	t, err := s.LoadTask(taskID)
	if err != nil {
		return HandoffDraft{}, err
	}
	switch {
	case t.ProfileID == nil:
		return HandoffDraft{}, &HandoffError{"the task has not been assigned to a profile yet; just run it"}
	case *t.ProfileID == to.ID:
		return HandoffDraft{}, &HandoffError{"the task already belongs to " + to.ID}
	case t.State == domain.TaskRunning:
		return HandoffDraft{}, &HandoffError{"the task is running; cancel it first"}
	case t.State == domain.TaskAccepted || t.State == domain.TaskDiscarded:
		return HandoffDraft{}, &HandoffError{"the task is " + t.State}
	}
	ev := policy.Evaluate(pol, policy.TaskFacts{Classification: t.Classification, Requires: t.Requires}, to, from)
	brief, err := s.Brief(taskID)
	if err != nil {
		return HandoffDraft{}, err
	}
	commits, files := gitFacts(s, t, git)
	var last *domain.RunSummary
	if len(t.Runs) > 0 {
		last = &t.Runs[len(t.Runs)-1]
	}
	lastResult := ""
	for i := len(t.Runs) - 1; i >= 0; i-- {
		if t.Runs[i].Summary != "" {
			lastResult = t.Runs[i].Summary
			break
		}
	}
	state := "- Estado de la tarea: **" + t.State + "**"
	if last != nil && last.BlockedReason != "" {
		state += " (bloqueada por " + last.BlockedReason + ")"
	}
	branch := "- Sin rama de trabajo (tarea de solo lectura o sin ejecutar)"
	if t.Branch != nil {
		branch = "- Rama de trabajo: `" + *t.Branch + "` (base `" + deref(t.BaseBranch) + "`)"
	}
	facts := []string{}
	for _, r := range t.Runs {
		facts = append(facts, runLine(r))
	}
	for _, c := range commits {
		facts = append(facts, "- commit "+c)
	}
	claims := "_(el agente anterior no dejó resumen)_"
	if lastResult != "" {
		claims = "> " + strings.Join(strings.Split(strings.TrimSpace(lastResult), "\n"), "\n> ")
	}
	changes := []string{}
	for _, f := range files {
		changes = append(changes, "- "+strings.Replace(f, "\t", " ", 1))
	}
	next := "- Continuar la tarea según el brief y verificar lo hecho en la rama."
	if t.State == domain.TaskBlocked {
		next = "- Continuar la tarea donde quedó; el perfil anterior alcanzó un límite."
	}
	raw := strings.Join([]string{
		"# Paquete de continuidad — " + t.Title,
		"",
		"Tarea `" + t.ID + "` · de `" + *t.ProfileID + "` a `" + to.ID + "`",
		"",
		section("Objetivo", []string{"Ver el brief original al final. Título: " + t.Title}),
		section("Estado", []string{state, "- Clasificación: " + t.Classification + " · modo: " + t.Mode, branch}),
		section("Hechos verificados (observados por Hilvan)", facts),
		section("Decisiones", []string{"- _(añada aquí decisiones tomadas que el siguiente agente deba respetar)_"}),
		section("Supuestos y afirmaciones del agente anterior (NO verificados)", []string{claims}),
		section("Archivos y cambios", changes),
		section("Siguiente paso", []string{next}),
		"---",
		"",
		"## Brief original",
		"",
		brief,
	}, "\n")
	// SR-013: secrets never travel in a package; matches are redacted and reported.
	body, matches := security.RedactSecrets(raw)
	return HandoffDraft{ID: domain.NewHandoffID(), Task: t, From: *t.ProfileID, To: to, Evaluation: ev, Body: body, Secrets: matches}, nil
}

// SHA256 is the hex digest of a UTF-8 text.
func SHA256(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// ApproveHandoff stores exactly the approved text (after any user edit), records its hash and
// moves the task to the target profile. The caller must have shown the text and obtained approval.
func ApproveHandoff(s *store.ProjectStore, d HandoffDraft, approved string) (task domain.Task, hash, file string, err error) {
	if !d.Evaluation.Eligible {
		return task, "", "", &HandoffError{fmt.Sprintf("profile %s is not eligible: %s", d.To.ID, reasons(d.Evaluation))}
	}
	if left := security.FindSecrets(approved); len(left) > 0 {
		kinds := make([]string, len(left))
		for i, m := range left {
			kinds[i] = m.Kind
		}
		return task, "", "", &HandoffError{"the edited package contains possible secrets (" + strings.Join(kinds, ", ") + ")"}
	}
	hash = SHA256(approved)
	header := strings.Join([]string{
		"---",
		"id: " + d.ID,
		"task: " + d.Task.ID,
		"from: " + d.From,
		"to: " + d.To.ID,
		"classification: " + d.Task.Classification,
		"sha256: " + hash,
		"approvedAt: " + store.Now(),
		fmt.Sprintf("secretsRedacted: %d", len(d.Secrets)),
		"---",
		"",
	}, "\n")
	if file, err = s.HandoffFile(d.Task.ID, d.ID); err != nil {
		return task, "", "", err
	}
	if err = os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return task, "", "", err
	}
	if err = security.WriteFileAtomic(file, []byte(header+approved), 0o600); err != nil {
		return task, "", "", err
	}
	current, err := s.LoadTask(d.Task.ID)
	if err != nil {
		return task, "", "", err
	}
	current.ProfileID = &d.To.ID
	// The model pinned on the first run belongs to the profile that pinned it (D-030). Another
	// application cannot use it, so the continuing profile uses its own model.
	current.Model = nil
	if current.State != domain.TaskInReview {
		current.State = domain.TaskReady
	}
	current.Handoffs = append(current.Handoffs, d.ID)
	if !contains(current.ApprovedProfiles, d.To.ID) {
		current.ApprovedProfiles = append(current.ApprovedProfiles, d.To.ID)
	}
	if task, err = s.SaveTask(current, nil); err != nil {
		return task, "", "", err
	}
	err = s.Audit(store.AuditEvent{Type: "handoff.approved", TaskID: task.ID, ProfileID: d.To.ID,
		Data: map[string]any{"handoff": d.ID, "from": d.From, "sha256": hash, "secretsRedacted": len(d.Secrets)}})
	return task, hash, file, err
}

// CancelHandoff records that nothing was sent.
func CancelHandoff(s *store.ProjectStore, d HandoffDraft) error {
	return s.Audit(store.AuditEvent{Type: "handoff.cancelled", TaskID: d.Task.ID, ProfileID: d.To.ID,
		Data: map[string]any{"handoff": d.ID, "from": d.From}})
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
