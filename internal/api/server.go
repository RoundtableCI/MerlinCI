package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/RoundtableCI/MerlinCI/internal/blob"
	"github.com/RoundtableCI/MerlinCI/internal/budget"
	"github.com/RoundtableCI/MerlinCI/internal/config"
	"github.com/RoundtableCI/MerlinCI/internal/runtime"
	"github.com/RoundtableCI/MerlinCI/internal/store"
	"github.com/RoundtableCI/MerlinCI/internal/workflow"
)

// Workflows starts and steers durable runs. Temporal implements it.
type Workflows interface {
	Start(ctx context.Context, task runtime.Task) error
	StartCampaign(ctx context.Context, in workflow.CampaignInput) error
	Decide(ctx context.Context, taskID string, d workflow.Decision) error
	Cancel(ctx context.Context, taskID string) error
}

// Server is the control plane: the way a person starts work, inspects what
// MerlinCI did and why, approves or refuses what it asks for, and pauses it
// (deck slides 6, 10, 14).
type Server struct {
	Config    *config.Bundle
	Store     *store.Store
	Ledger    *store.Ledger
	Blobs     blob.Store
	Workflows Workflows
	// Token is the bearer token every request must carry.
	Token string
	Log   *slog.Logger
}

// MinTokenLength is the shortest API token the server accepts.
const MinTokenLength = 24

// operator is who the single API token stands for. Named users arrive with
// the dashboard's sign-in.
const operator = "operator"

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		write(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	routes := map[string]func(http.ResponseWriter, *http.Request) error{
		"POST /v1/tasks":                 s.createTask,
		"GET /v1/tasks":                  s.listTasks,
		"GET /v1/tasks/{id}":             s.getTask,
		"GET /v1/tasks/{id}/events":      s.taskEvents,
		"POST /v1/tasks/{id}/decision":   s.decide,
		"POST /v1/tasks/{id}/cancel":     s.cancel,
		"GET /v1/artifacts/{id}":         s.getArtifact,
		"GET /v1/artifacts/{id}/content": s.artifactContent,
		"POST /v1/campaigns":             s.createCampaign,
		"GET /v1/campaigns":              s.listCampaigns,
		"GET /v1/campaigns/{id}":         s.getCampaign,
		"GET /v1/budget":                 s.budget,
		"GET /v1/control":                s.getControl,
		"POST /v1/control":               s.setControl,
		"GET /v1/config":                 s.config,
	}
	for pattern, h := range routes {
		mux.Handle(pattern, s.authed(h))
	}
	return mux
}

type apiError struct {
	status int
	msg    string
}

func (e *apiError) Error() string { return e.msg }

func fail(status int, msg string) error { return &apiError{status, msg} }

func (s *Server) authed(h func(http.ResponseWriter, *http.Request) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(s.Token)) != 1 {
			write(w, http.StatusUnauthorized, map[string]string{"error": "a valid bearer token is required"})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		err := h(w, r)
		if err == nil {
			return
		}
		var ae *apiError
		if errors.As(err, &ae) {
			write(w, ae.status, map[string]string{"error": ae.msg})
			return
		}
		// Internal errors are logged in full and reported without detail.
		if s.Log != nil {
			s.Log.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err.Error())
		}
		write(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
	})
}

func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func read(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fail(http.StatusBadRequest, "the request body is not valid: "+err.Error())
	}
	return nil
}

func newID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // the system has no randomness; nothing is safe to do
	}
	return prefix + hex.EncodeToString(b[:])
}

func (s *Server) record(ctx context.Context, taskID, role, kind string, data map[string]any) {
	ev := runtime.Event{TaskID: taskID, Role: role, Type: kind, Data: data}
	if err := s.Store.Events().Record(context.WithoutCancel(ctx), ev); err != nil && s.Log != nil {
		s.Log.Error("event not recorded", "task", taskID, "type", kind, "error", err.Error())
	}
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Role  string         `json:"role"`
		Input map[string]any `json:"input"`
	}
	if err := read(r, &req); err != nil {
		return err
	}
	role, ok := s.Config.Roles[req.Role]
	if !ok {
		return fail(http.StatusUnprocessableEntity, "role "+strconv.Quote(req.Role)+" is not defined")
	}
	if !role.Spec.Enabled {
		return fail(http.StatusUnprocessableEntity, "role "+strconv.Quote(req.Role)+" is not enabled")
	}
	if req.Input == nil {
		req.Input = map[string]any{}
	}
	task := runtime.Task{ID: newID("task_"), Role: req.Role, Input: req.Input}
	if err := s.Store.Tasks().Insert(r.Context(), task, operator); err != nil {
		return err
	}
	s.record(r.Context(), task.ID, task.Role, "task created", map[string]any{"by": operator})
	if err := s.Workflows.Start(r.Context(), task); err != nil {
		s.record(r.Context(), task.ID, task.Role, "task not started", map[string]any{"error": err.Error()})
		return err
	}
	write(w, http.StatusAccepted, map[string]string{"id": task.ID})
	return nil
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) error {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 500 {
			return fail(http.StatusBadRequest, "limit must be between 1 and 500")
		}
		limit = n
	}
	tasks, err := s.Store.Tasks().List(r.Context(), limit)
	if err != nil {
		return err
	}
	write(w, http.StatusOK, map[string]any{"tasks": tasks})
	return nil
}

// RunView is where a run stands, without the conversation itself.
type RunView struct {
	Status       string `json:"status"`
	Reason       string `json:"reason,omitempty"`
	Step         int    `json:"step"`
	Spent        string `json:"spent"`
	Currency     string `json:"currency"`
	InputTokens  int    `json:"inputTokens"`
	OutputTokens int    `json:"outputTokens"`
	Sources      int    `json:"sources"`
	ArtifactID   string `json:"artifactId,omitempty"`
	ConfigDigest string `json:"configDigest,omitempty"`
	// Pending is the action waiting for a person, when there is one.
	Pending *PendingView `json:"pending,omitempty"`
}

// PendingView is an action waiting for a person's decision.
type PendingView struct {
	Seq   int             `json:"seq"`
	Tool  string          `json:"tool"`
	Input json.RawMessage `json:"input"`
}

func (s *Server) runView(state runtime.State) RunView {
	v := RunView{
		Status: string(state.Status), Reason: state.Reason, Step: state.Step,
		Spent: state.Spent.String(), Currency: s.Config.Budget.Spec.Currency,
		InputTokens: state.InputTokens, OutputTokens: state.OutputTokens,
		Sources: len(state.Sources), ArtifactID: state.ArtifactID, ConfigDigest: state.ConfigDigest,
	}
	if state.Status == runtime.AwaitingApproval && state.PendingCall != nil {
		v.Pending = &PendingView{Seq: state.ApprovalSeq, Tool: state.PendingCall.Name, Input: state.PendingCall.Arguments}
	}
	return v
}

func (s *Server) task(r *http.Request) (store.TaskRecord, error) {
	rec, found, err := s.Store.Tasks().Get(r.Context(), r.PathValue("id"))
	if err != nil {
		return rec, err
	}
	if !found {
		return rec, fail(http.StatusNotFound, "no such task")
	}
	return rec, nil
}

func (s *Server) getTask(w http.ResponseWriter, r *http.Request) error {
	rec, err := s.task(r)
	if err != nil {
		return err
	}
	out := map[string]any{"task": rec}
	state, found, err := s.Store.Checkpoints().Load(r.Context(), rec.ID)
	if err != nil {
		return err
	}
	if found {
		out["run"] = s.runView(state)
	}
	write(w, http.StatusOK, out)
	return nil
}

func (s *Server) taskEvents(w http.ResponseWriter, r *http.Request) error {
	rec, err := s.task(r)
	if err != nil {
		return err
	}
	events, err := s.Store.Events().List(r.Context(), rec.ID)
	if err != nil {
		return err
	}
	write(w, http.StatusOK, map[string]any{"events": events})
	return nil
}

func (s *Server) decide(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Seq     *int   `json:"seq"`
		Approve *bool  `json:"approve"`
		Reason  string `json:"reason"`
	}
	if err := read(r, &req); err != nil {
		return err
	}
	if req.Approve == nil {
		return fail(http.StatusBadRequest, "approve must be true or false")
	}
	if req.Seq == nil {
		return fail(http.StatusBadRequest, "seq is required: it names the pending action you looked at, from the task's status")
	}
	rec, err := s.task(r)
	if err != nil {
		return err
	}
	state, found, err := s.Store.Checkpoints().Load(r.Context(), rec.ID)
	if err != nil {
		return err
	}
	if !found || state.Status != runtime.AwaitingApproval || state.PendingCall == nil {
		return fail(http.StatusConflict, "the task is not waiting for a decision")
	}
	// A decision answers the action its author looked at, and no other. If
	// the task has moved on to a different action since, a request that was
	// delayed or sent twice must not decide that one unseen.
	if *req.Seq != state.ApprovalSeq {
		return fail(http.StatusConflict, "the task is now waiting on a different action; look at its status again before deciding")
	}
	d := workflow.Decision{Seq: *req.Seq, Approve: *req.Approve, Reason: req.Reason}
	s.record(r.Context(), rec.ID, rec.Role, "decision", map[string]any{
		"by": operator, "approve": d.Approve, "reason": d.Reason, "seq": d.Seq, "tool": state.PendingCall.Name,
	})
	if err := s.Workflows.Decide(r.Context(), rec.ID, d); err != nil {
		return err
	}
	write(w, http.StatusAccepted, map[string]any{"seq": d.Seq, "approve": d.Approve})
	return nil
}

func (s *Server) cancel(w http.ResponseWriter, r *http.Request) error {
	rec, err := s.task(r)
	if err != nil {
		return err
	}
	cps := s.Store.Checkpoints()
	state, found, err := cps.Load(r.Context(), rec.ID)
	if err != nil {
		return err
	}
	if found && state.Done() {
		return fail(http.StatusConflict, "the task has already ended as "+string(state.Status))
	}
	if !found {
		state = runtime.State{TaskID: rec.ID, Role: rec.Role}
	}
	// The database is what stops the run. Once this checkpoint says
	// cancelled, no worker can replace it, so a worker in the middle of the
	// task fails at its next checkpoint and goes no further. Stopping the
	// workflow afterwards only tidies up.
	state.Status, state.Reason, state.PendingCall = runtime.Cancelled, "cancelled by a person", nil
	if err := cps.Save(r.Context(), state); err != nil {
		return err
	}
	s.record(r.Context(), rec.ID, rec.Role, "task cancelled", map[string]any{"by": operator})
	if err := s.Workflows.Cancel(r.Context(), rec.ID); err != nil && s.Log != nil {
		s.Log.Warn("task is cancelled, but its workflow could not be stopped", "task", rec.ID, "error", err.Error())
	}
	write(w, http.StatusAccepted, map[string]string{"status": string(runtime.Cancelled)})
	return nil
}

func (s *Server) getArtifact(w http.ResponseWriter, r *http.Request) error {
	rec, found, err := s.Store.Artifacts().Get(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	if !found {
		return fail(http.StatusNotFound, "no such artifact")
	}
	write(w, http.StatusOK, rec)
	return nil
}

func (s *Server) artifactContent(w http.ResponseWriter, r *http.Request) error {
	rec, found, err := s.Store.Artifacts().Get(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	if !found {
		return fail(http.StatusNotFound, "no such artifact")
	}
	body, err := s.Blobs.Get(r.Context(), rec.Content.BlobKey)
	if err != nil {
		return err
	}
	// Artifact content is model output. Serve it as plain text so that a
	// browser never runs it.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(body)
	return nil
}

func (s *Server) budget(w http.ResponseWriter, r *http.Request) error {
	type line struct {
		Category string `json:"category"`
		Period   string `json:"period"`
		Limit    string `json:"limit"`
		Spent    string `json:"spent"`
	}
	spec := s.Config.Budget.Spec
	lines := []line{}
	for _, lim := range spec.Limits {
		spent, err := s.Ledger.Spent(r.Context(), lim.Category, lim.Period)
		if err != nil {
			return err
		}
		lines = append(lines, line{lim.Category, lim.Period, budget.FromAmount(lim.Amount).String(), spent.String()})
	}
	write(w, http.StatusOK, map[string]any{"currency": spec.Currency, "alertThresholds": spec.AlertThresholds, "limits": lines})
	return nil
}

func (s *Server) getControl(w http.ResponseWriter, r *http.Request) error {
	paused, err := s.Store.Controls().Paused(r.Context())
	if err != nil {
		return err
	}
	write(w, http.StatusOK, map[string]bool{"paused": paused})
	return nil
}

func (s *Server) setControl(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Paused *bool `json:"paused"`
	}
	if err := read(r, &req); err != nil {
		return err
	}
	if req.Paused == nil {
		return fail(http.StatusBadRequest, "paused must be true or false")
	}
	if err := s.Store.Controls().SetPaused(r.Context(), *req.Paused, operator); err != nil {
		return err
	}
	kind := "resumed"
	if *req.Paused {
		kind = "paused"
	}
	s.record(r.Context(), "-", "-", kind, map[string]any{"by": operator})
	write(w, http.StatusOK, map[string]bool{"paused": *req.Paused})
	return nil
}

func (s *Server) config(w http.ResponseWriter, r *http.Request) error {
	type role struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
		Model   string `json:"model"`
	}
	roles := []role{}
	for name, doc := range s.Config.Roles {
		model := doc.Spec.Model
		if model == "" {
			model = s.Config.Models.Spec.Default
		}
		roles = append(roles, role{name, doc.Spec.Enabled, model})
	}
	sort.Slice(roles, func(i, j int) bool { return roles[i].Name < roles[j].Name })
	write(w, http.StatusOK, map[string]any{
		"digest":          s.Config.Digest,
		"profile":         s.Config.Profile.Metadata.Name,
		"profileApproved": s.Config.Profile.Spec.Approved(),
		"roles":           roles,
	})
	return nil
}

func (s *Server) createCampaign(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Workflow string         `json:"workflow"`
		Input    map[string]any `json:"input"`
	}
	if err := read(r, &req); err != nil {
		return err
	}
	tmpl, ok := s.Config.Workflows[req.Workflow]
	if !ok {
		return fail(http.StatusUnprocessableEntity, "workflow "+strconv.Quote(req.Workflow)+" is not defined")
	}
	steps, err := s.plan(tmpl.Spec.Steps)
	if err != nil {
		return err
	}
	if req.Input == nil {
		req.Input = map[string]any{}
	}
	in := workflow.CampaignInput{ID: newID("camp_"), Workflow: req.Workflow, Steps: steps, Input: req.Input}
	if err := s.Store.Campaigns().Insert(r.Context(), in, operator); err != nil {
		return err
	}
	s.record(r.Context(), in.ID, "-", "campaign created", map[string]any{"by": operator, "workflow": in.Workflow})
	if err := s.Workflows.StartCampaign(r.Context(), in); err != nil {
		failed := workflow.CampaignResult{ID: in.ID, Workflow: in.Workflow, Status: workflow.StepFailed, Reason: "the campaign could not be started"}
		_ = s.Store.Campaigns().Save(context.WithoutCancel(r.Context()), failed)
		return err
	}
	write(w, http.StatusAccepted, map[string]string{"id": in.ID})
	return nil
}

// plan puts a template's steps in an order that satisfies every step's
// needs, and refuses a template that cannot run as configured: every role
// step needs a defined, enabled role before the campaign starts, not when
// it reaches that step.
func (s *Server) plan(steps []config.Step) ([]workflow.Step, error) {
	for _, st := range steps {
		if st.Role == "" {
			continue
		}
		role, ok := s.Config.Roles[st.Role]
		if !ok {
			return nil, fail(http.StatusUnprocessableEntity, "step "+strconv.Quote(st.ID)+": there is no runnable role "+strconv.Quote(st.Role))
		}
		if !role.Spec.Enabled {
			return nil, fail(http.StatusUnprocessableEntity, "step "+strconv.Quote(st.ID)+": role "+strconv.Quote(st.Role)+" is not enabled")
		}
	}
	done := map[string]bool{}
	var out []workflow.Step
	for len(out) < len(steps) {
		progressed := false
		for _, st := range steps {
			if done[st.ID] {
				continue
			}
			ready := true
			for _, n := range st.Needs {
				ready = ready && done[n]
			}
			if ready {
				done[st.ID], progressed = true, true
				out = append(out, workflow.Step{ID: st.ID, Role: st.Role, Evaluator: st.Evaluator, Needs: st.Needs})
			}
		}
		if !progressed {
			return nil, fail(http.StatusUnprocessableEntity, "the workflow's steps cannot be ordered")
		}
	}
	return out, nil
}

func (s *Server) listCampaigns(w http.ResponseWriter, r *http.Request) error {
	campaigns, err := s.Store.Campaigns().List(r.Context(), 50)
	if err != nil {
		return err
	}
	write(w, http.StatusOK, map[string]any{"campaigns": campaigns})
	return nil
}

func (s *Server) getCampaign(w http.ResponseWriter, r *http.Request) error {
	rec, found, err := s.Store.Campaigns().Get(r.Context(), r.PathValue("id"))
	if err != nil {
		return err
	}
	if !found {
		return fail(http.StatusNotFound, "no such campaign")
	}
	write(w, http.StatusOK, rec)
	return nil
}
