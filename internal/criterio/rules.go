package criterio

import (
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/ferchosur/hilvan/internal/jsfmt"
)

// Seed vocabulary (es/en), stemmed at load so both languages share roots.
var lexicon = map[string]string{
	"implement": "implementar implement anadir agregar add crear create nuevo nueva new feature funcionalidad soporte support construir build integrar integrate habilitar enable endpoint comando command",
	"fix":       "corregir fix arreglar bug error falla fallo crash roto broken regresion regression solucionar resolve problema issue excepcion exception",
	"refactor":  "refactor refactorizar renombrar rename extraer extract limpiar cleanup simplificar simplify reorganizar reorganize restructure mover move duplicado duplicate modularizar desacoplar decouple",
	"test":      "test tests prueba pruebas testing cobertura coverage unitario unit e2e fixture mock assert",
	"review":    "revisar review auditar audit inspeccionar inspect evaluar evaluate verificar verify validar validate feedback observaciones",
	"research":  "investigar research comparar compare explorar explore analizar analyze opciones options alternativas alternatives benchmark estudio study viabilidad feasibility encuesta fuentes fuente sources source detalle detail",
	"docs":      "documentar document documentacion documentation readme docs guia guide manual changelog comentarios comments tutorial explicar explain",
}

var stems = func() map[string][]string {
	m := map[string][]string{}
	for _, t := range TaskTypes {
		for _, w := range strings.Split(lexicon[t], " ") {
			s := Stem(w)
			m[s] = unique(append(append([]string{}, m[s]...), t))
		}
	}
	return m
}()

// Softmax is a numerically stable softmax.
func Softmax(xs []float64) []float64 {
	max := math.Inf(-1)
	for _, x := range xs {
		max = math.Max(max, x)
	}
	ex := make([]float64, len(xs))
	sum := 0.0
	for i, x := range xs {
		ex[i] = math.Exp(x - max)
		sum += ex[i]
	}
	for i := range ex {
		ex[i] /= sum
	}
	return ex
}

func sortByWeightDesc(why []Reason) []Reason {
	sort.SliceStable(why, func(i, j int) bool { return why[i].Weight > why[j].Weight })
	return why
}

func sortByAbsWeightDesc(why []Reason) []Reason {
	sort.SliceStable(why, func(i, j int) bool { return math.Abs(why[i].Weight) > math.Abs(why[j].Weight) })
	return why
}

func setOf(xs []string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}

// TaskTypeRules is the rule prior over task types: keyword hits through a tempered softmax.
func TaskTypeRules(s TaskSignals) (probs []float64, why []Reason) {
	hits := make([]float64, len(TaskTypes))
	why = []Reason{}
	titleToks := setOf(Tokenize(s.Title))
	for _, tok := range unique(Tokenize(s.Title + " " + s.Text)) {
		types, ok := stems[tok]
		if !ok {
			continue
		}
		w := 1.0
		if titleToks[tok] {
			w = 2 // the title says more than the body
		}
		for _, t := range types {
			hits[typeIndex(t)] += w
		}
		why = append(why, Reason{Signal: `palabra "` + tok + `" → ` + strings.Join(types, "/"), Weight: w})
	}
	hits[0] += 0.3 // mild prior: unlabeled work is usually implementation
	scaled := make([]float64, len(hits))
	for i, h := range hits {
		scaled[i] = h * 1.2
	}
	return Softmax(scaled), sortByWeightDesc(why)
}

var heavyStems = setOf(mapStem(strings.Fields("arquitectura architecture migracion migration seguridad security concurrencia concurrency paralelo parallel distribuido distributed rendimiento performance multiples multiple varios several sistema system rediseno redesign protocolo protocol database")))

func mapStem(ws []string) []string {
	out := make([]string, len(ws))
	for i, w := range ws {
		out[i] = Stem(w)
	}
	return out
}

// ComplexityRules scores 2..10 from text length, acceptance criteria and heavy-topic words.
func ComplexityRules(s TaskSignals) (score float64, why []Reason) {
	toks := Tokenize(s.Title + " " + s.Text)
	var heavyAll []string
	for _, t := range toks {
		if heavyStems[t] {
			heavyAll = append(heavyAll, t)
		}
	}
	heavy := unique(heavyAll)
	why = []Reason{}
	score = 3
	lenBoost := math.Min(2.5, math.Log2(1+float64(len(toks))/15))
	score += lenBoost
	if lenBoost > 0.5 {
		why = append(why, Reason{Signal: "brief de " + itoa(len(toks)) + " palabras", Weight: lenBoost})
	}
	if s.CriteriaCount != nil && *s.CriteriaCount != 0 {
		c := math.Min(2, float64(*s.CriteriaCount)*0.4)
		score += c
		why = append(why, Reason{Signal: itoa(*s.CriteriaCount) + " criterios de aceptación", Weight: c})
	}
	if len(heavy) > 0 {
		h := math.Min(3, float64(len(heavy)))
		score += h
		why = append(why, Reason{Signal: "temas exigentes: " + strings.Join(heavy, ", "), Weight: h})
	}
	return ClampScore(score), sortByWeightDesc(why)
}

var docFile = regexp.MustCompile(`(?i)\.(md|txt|rst)$`)

// DiffRiskRules scores a diff 2..10: size, sensitive paths, deletions, untested changes.
func DiffRiskRules(d DiffSignals) (score float64, why []Reason) {
	lines := d.LinesAdded + d.LinesDeleted
	score = 2
	size := math.Min(3, math.Log10(1+float64(lines)))
	score += size
	why = []Reason{{Signal: itoa(lines) + " líneas cambiadas en " + itoa(d.FilesChanged) + " archivos", Weight: size}}
	var sensitive []string
	for _, p := range d.Paths {
		if IsSensitivePath(p) {
			sensitive = append(sensitive, p)
		}
	}
	if len(sensitive) > 0 {
		w := math.Min(3, 1.5+float64(len(sensitive))*0.5)
		score += w
		why = append(why, Reason{Signal: "rutas sensibles: " + strings.Join(sensitive[:min(3, len(sensitive))], ", "), Weight: w})
	}
	if lines > 0 && float64(d.LinesDeleted)/float64(lines) > 0.6 && lines > 30 {
		score += 1.5
		why = append(why, Reason{Signal: "predominan las eliminaciones", Weight: 1.5})
	}
	codeFiles, anyTest := 0, false
	for _, p := range d.Paths {
		if IsTestPath(p) {
			anyTest = true
		} else if !docFile.MatchString(p) {
			codeFiles++
		}
	}
	if codeFiles > 0 && !anyTest {
		score++
		why = append(why, Reason{Signal: "cambia código sin tocar tests", Weight: 1})
	}
	return ClampScore(score), sortByWeightDesc(why)
}

var corpStems = setOf(mapStem(strings.Fields("confidencial confidential interno internal cliente clientes customer customers jira confluence salesforce sap nomina payroll contrato contract nda propietario proprietary empresa company corporativo corporate produccion production financiero financial")))

var pii = []*regexp.Regexp{
	regexp.MustCompile(`\b\d{4}[ -]?\d{4}[ -]?\d{4}[ -]?\d{4}\b`), // card-like number
	regexp.MustCompile(`\b[A-Z]{2}\d{2}[A-Z0-9]{11,30}\b`),        // IBAN-like
	regexp.MustCompile(`\b[\w.+-]+@[\w-]+\.[\w.]+\b`),             // email
}

func sigmoid(z float64) float64 { return 1 / (1 + math.Exp(-z)) }

// SensitivityRules flags briefs that may carry corporate or personal data (only to harden).
func SensitivityRules(s TaskSignals) (p float64, why []Reason) {
	text := s.Title + " " + s.Text
	var hitsAll []string
	for _, t := range Tokenize(text) {
		if corpStems[t] {
			hitsAll = append(hitsAll, t)
		}
	}
	hits := unique(hitsAll)
	why = []Reason{}
	for _, h := range hits {
		why = append(why, Reason{Signal: `palabra "` + h + `"`, Weight: 1})
	}
	z := -2 + float64(len(hits))*1.2
	for _, re := range pii {
		if re.MatchString(text) {
			z += 2
			why = append(why, Reason{Signal: "posible dato personal o financiero en el texto", Weight: 2})
		}
	}
	return sigmoid(z), why
}

var failureHints = regexp.MustCompile(`(?i)\b(no pude|no fue posible|could not|couldn't|unable to|not implemented|todo|fixme|failed|fall[oó]|error)\b`)

// AcceptanceRules is the prior for "did the run satisfy the brief?".
func AcceptanceRules(r RunSignals) (p float64, why []Reason) {
	if r.State != "completed" {
		return 0.05, []Reason{{Signal: "la ejecución terminó en " + r.State, Weight: -3}}
	}
	z := 1.2
	why = []Reason{{Signal: "la ejecución terminó correctamente", Weight: 1.2}}
	if r.PermissionDenials != 0 {
		z -= math.Min(2, float64(r.PermissionDenials)*0.7)
		why = append(why, Reason{Signal: itoa(r.PermissionDenials) + " permisos denegados", Weight: -0.7})
	}
	if r.ResultText != "" && failureHints.MatchString(r.ResultText) {
		z -= 1.5
		why = append(why, Reason{Signal: "el resumen menciona problemas o pendientes", Weight: -1.5})
	}
	if r.Diff != nil && r.Diff.FilesChanged == 0 && r.CriteriaCount != nil && *r.CriteriaCount > 0 {
		z--
		why = append(why, Reason{Signal: "no hubo cambios pese a tener criterios", Weight: -1})
	}
	return sigmoid(z), sortByAbsWeightDesc(why)
}

// ClampScore bounds a score to 2..10 with one decimal.
func ClampScore(x float64) float64 {
	return jsfmt.Round(math.Min(10, math.Max(2, x))*10) / 10
}
