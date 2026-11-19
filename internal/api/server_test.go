package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/RoundtableCI/MerlinCI/internal/artifacts"
	"github.com/RoundtableCI/MerlinCI/internal/blob"
	"github.com/RoundtableCI/MerlinCI/internal/config"
	"github.com/RoundtableCI/MerlinCI/internal/runtime"
	"github.com/RoundtableCI/MerlinCI/internal/store"
	"github.com/RoundtableCI/MerlinCI/internal/workflow"
	"github.com/RoundtableCI/MerlinCI/pkg/provider"
	"github.com/RoundtableCI/MerlinCI/schemas"
)

// fakeWorkflows records what the API asked the workflow engine to do.
type fakeWorkflows struct {
	started   []runtime.Task
	campaigns []workflow.CampaignInput
	decisions map[string]workflow.Decision
	cancelled []string
	err       error
}

func (f *fakeWorkflows) Start(_ context.Context, task runtime.Task) error {
	f.started = append(f.started, task)
	return f.err
}

func (f *fakeWorkflows) StartCampaign(_ context.Context, in workflow.CampaignInput) error {
	f.campaigns = append(f.campaigns, in)
	return f.err
}

func (f *fakeWorkflows) Decide(_ context.Context, id string, d workflow.Decision) error {
	f.decisions[id] = d
	return f.err
}

func (f *fakeWorkflows) Cancel(_ context.Context, id string) error {
	f.cancelled = append(f.cancelled, id)
	return f.err
}

const token = "a-test-token-that-is-long-enough"

type fixture struct {
	t     *testing.T
	srv   *httptest.Server
	store *store.Store
	wf    *fakeWorkflows
	blobs *blob.Memory
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	admin := os.Getenv("MERLIN_TEST_DATABASE_URL")
	if admin == "" {
		t.Skip("MERLIN_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("merlin_api_%d", time.Now().UnixNano())
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	c := conn.Config()
	st, err := store.Open(ctx, fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", c.User, c.Password, c.Host, c.Port, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Bundle{
		Digest:  "sha256:test",
		Profile: &config.Profile{Metadata: config.Metadata{Name: "example"}},
		Budget: &config.Budget{Spec: config.BudgetSpec{
			Currency:        "INR",
			Limits:          []config.Limit{{Category: "inference", Period: "day", Amount: 15000}},
			AlertThresholds: []float64{0.7, 0.9},
		}},
		Models: &config.Models{Spec: config.ModelsSpec{Default: "small"}},
		Roles: map[string]config.Role{
			"researcher": {Spec: config.RoleSpec{Enabled: true}},
			"writer":     {Spec: config.RoleSpec{Enabled: false, Model: "large"}},
		},
	}
	f := &fixture{t: t, store: st, wf: &fakeWorkflows{decisions: map[string]workflow.Decision{}}, blobs: blob.NewMemory()}
	server := &Server{
		Config: cfg, Store: st, Ledger: st.Ledger(cfg.Budget.Spec, nil, nil),
		Blobs: f.blobs, Workflows: f.wf, Token: token,
	}
	f.srv = httptest.NewServer(server.Handler())
	t.Cleanup(func() {
		f.srv.Close()
		st.Close()
		_, _ = conn.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")
		_ = conn.Close(ctx)
	})
	return f
}

// call sends a request with the given token and decodes the response.
func (f *fixture) call(tok, method, path, body string, out any) (int, http.Header) {
	f.t.Helper()
	var payload io.Reader
	if body != "" {
		payload = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, f.srv.URL+path, payload)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if p, ok := out.(*string); ok {
		*p = string(raw)
	} else if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			f.t.Fatalf("%s %s: unreadable response %q", method, path, raw)
		}
	}
	return resp.StatusCode, resp.Header
}

func (f *fixture) create(role string) string {
	f.t.Helper()
	var out struct {
		ID string `json:"id"`
	}
	if code, _ := f.call(token, http.MethodPost, "/v1/tasks", `{"role":"`+role+`","input":{"topic":"x"}}`, &out); code != http.StatusAccepted {
		f.t.Fatalf("create task: status %d", code)
	}
	return out.ID
}

var ctx = context.Background()

func TestEveryRouteNeedsTheToken(t *testing.T) {
	f := newFixture(t)
	routes := [][2]string{
		{http.MethodPost, "/v1/tasks"}, {http.MethodGet, "/v1/tasks"}, {http.MethodGet, "/v1/tasks/x"},
		{http.MethodGet, "/v1/tasks/x/events"}, {http.MethodPost, "/v1/tasks/x/decision"}, {http.MethodPost, "/v1/tasks/x/cancel"},
		{http.MethodGet, "/v1/artifacts/x"}, {http.MethodGet, "/v1/artifacts/x/content"}, {http.MethodGet, "/v1/budget"},
		{http.MethodGet, "/v1/control"}, {http.MethodPost, "/v1/campaigns"}, {http.MethodGet, "/v1/campaigns"}, {http.MethodGet, "/v1/campaigns/x"}, {http.MethodPost, "/v1/control"}, {http.MethodGet, "/v1/config"},
	}
	for _, r := range routes {
		for _, tok := range []string{"", "wrong", token + "x", token[:len(token)-1]} {
			if code, _ := f.call(tok, r[0], r[1], `{}`, nil); code != http.StatusUnauthorized {
				t.Errorf("%s %s with token %q: status %d, want 401", r[0], r[1], tok, code)
			}
		}
	}
	if len(f.wf.started) != 0 {
		t.Error("an unauthenticated request started a task")
	}
	if code, _ := f.call("", http.MethodGet, "/healthz", "", nil); code != http.StatusOK {
		t.Errorf("healthz: status %d", code)
	}
}

func TestCreateTask(t *testing.T) {
	f := newFixture(t)
	id := f.create("researcher")
	if len(f.wf.started) != 1 || f.wf.started[0].ID != id || f.wf.started[0].Role != "researcher" || f.wf.started[0].Input["topic"] != "x" {
		t.Fatalf("workflow started with %+v", f.wf.started)
	}

	var view struct {
		Task store.TaskRecord `json:"task"`
		Run  *RunView         `json:"run"`
	}
	f.call(token, http.MethodGet, "/v1/tasks/"+id, "", &view)
	if view.Task.Status != "pending" || view.Task.CreatedBy != "operator" || view.Run != nil {
		t.Errorf("new task = %+v, run %+v; want pending with no run yet", view.Task, view.Run)
	}
	if err := schemas.ValidateValue("Task", view.Task); err != nil {
		t.Errorf("the task the API returns does not match schemas/task.schema.json: %v", err)
	}

	bad := map[string]string{
		`{"role":"nobody"}`:               "is not defined",
		`{"role":"writer"}`:               "is not enabled",
		`{"role":"researcher","shell":1}`: "not valid",
		`not json`:                        "not valid",
	}
	for body, want := range bad {
		var out struct {
			Error string `json:"error"`
		}
		code, _ := f.call(token, http.MethodPost, "/v1/tasks", body, &out)
		if code < 400 || code >= 500 || !strings.Contains(out.Error, want) {
			t.Errorf("POST %s: status %d, error %q; want a 4xx mentioning %q", body, code, out.Error, want)
		}
	}
	if len(f.wf.started) != 1 {
		t.Errorf("%d workflows started, want 1: a refused request must start nothing", len(f.wf.started))
	}

	var list struct {
		Tasks []store.TaskRecord `json:"tasks"`
	}
	f.call(token, http.MethodGet, "/v1/tasks", "", &list)
	if len(list.Tasks) != 1 || list.Tasks[0].ID != id {
		t.Errorf("task list = %+v", list.Tasks)
	}
}

func TestDecision(t *testing.T) {
	f := newFixture(t)
	id := f.create("researcher")
	decide := func(body string) (int, string) {
		var out string
		code, _ := f.call(token, http.MethodPost, "/v1/tasks/"+id+"/decision", body, &out)
		return code, out
	}

	// Nothing is waiting yet.
	if code, _ := decide(`{"approve":true,"seq":1}`); code != http.StatusConflict {
		t.Fatalf("decision on a task that is not waiting: status %d, want 409", code)
	}

	// The run stops for a person, at its third stop.
	waiting := runtime.State{
		TaskID: id, Role: "researcher", Status: runtime.AwaitingApproval, ApprovalSeq: 3, Spent: 126000,
		PendingCall: &provider.ToolCall{ID: "call_1_0", Name: "publish", Arguments: json.RawMessage(`{"title":"x"}`)},
	}
	if err := f.store.Checkpoints().Save(ctx, waiting); err != nil {
		t.Fatal(err)
	}
	var view struct {
		Run RunView `json:"run"`
	}
	f.call(token, http.MethodGet, "/v1/tasks/"+id, "", &view)
	if view.Run.Pending == nil || view.Run.Pending.Tool != "publish" || view.Run.Pending.Seq != 3 || view.Run.Spent != "0.126000" {
		t.Fatalf("run view = %+v, pending %+v", view.Run, view.Run.Pending)
	}

	if code, _ := decide(`{"seq":3}`); code != http.StatusBadRequest {
		t.Errorf("decision without approve: status %d, want 400", code)
	}
	// A decision must name the pending action its author looked at.
	if code, _ := decide(`{"approve":true}`); code != http.StatusBadRequest {
		t.Errorf("decision without seq: status %d, want 400", code)
	}
	// A decision for an earlier stop, arriving late or sent twice, must not
	// decide the action the task is waiting on now.
	for _, stale := range []string{`{"approve":true,"seq":2}`, `{"approve":true,"seq":4}`, `{"approve":true,"seq":0}`} {
		if code, body := decide(stale); code != http.StatusConflict || !strings.Contains(body, "different action") {
			t.Errorf("decision %s at stop 3: status %d (%s), want 409", stale, code, body)
		}
	}
	if len(f.wf.decisions) != 0 {
		t.Fatal("a refused request reached the workflow")
	}

	if code, _ := decide(`{"approve":false,"seq":3,"reason":"not this week"}`); code != http.StatusAccepted {
		t.Fatalf("reject: status %d", code)
	}
	if d := f.wf.decisions[id]; d.Seq != 3 || d.Approve || d.Reason != "not this week" {
		t.Errorf("decision sent to the workflow = %+v; want a rejection of stop 3", d)
	}

	// Who decided what is on the record.
	var log struct {
		Events []store.EventRecord `json:"events"`
	}
	f.call(token, http.MethodGet, "/v1/tasks/"+id+"/events", "", &log)
	last := log.Events[len(log.Events)-1]
	if last.Type != "decision" || last.Data["by"] != "operator" || last.Data["approve"] != false || last.Data["tool"] != "publish" {
		t.Errorf("last event = %+v", last)
	}
}

func TestCancel(t *testing.T) {
	f := newFixture(t)
	id := f.create("researcher")
	if err := f.store.Checkpoints().Save(ctx, runtime.State{TaskID: id, Role: "researcher", Status: runtime.Running, Step: 2}); err != nil {
		t.Fatal(err)
	}
	if code, _ := f.call(token, http.MethodPost, "/v1/tasks/"+id+"/cancel", "", nil); code != http.StatusAccepted {
		t.Fatalf("cancel: status %d", code)
	}
	if len(f.wf.cancelled) != 1 || f.wf.cancelled[0] != id {
		t.Errorf("cancelled = %v", f.wf.cancelled)
	}
	state, _, _ := f.store.Checkpoints().Load(ctx, id)
	if state.Status != runtime.Cancelled || state.Reason != "cancelled by a person" {
		t.Errorf("state after cancel = %s (%s)", state.Status, state.Reason)
	}
	// The worker that was running the task tries to save its next step.
	err := f.store.Checkpoints().Save(ctx, runtime.State{TaskID: id, Role: "researcher", Status: runtime.Running, Step: 3})
	if !errors.Is(err, runtime.ErrCancelled) {
		t.Errorf("a worker saved over a cancelled task: err = %v", err)
	}
	if code, _ := f.call(token, http.MethodPost, "/v1/tasks/"+id+"/cancel", "", nil); code != http.StatusConflict {
		t.Errorf("cancelling an ended task: status %d, want 409", code)
	}

	// A task that has not started yet can be cancelled too, and then never starts.
	pending := f.create("researcher")
	if code, _ := f.call(token, http.MethodPost, "/v1/tasks/"+pending+"/cancel", "", nil); code != http.StatusAccepted {
		t.Fatalf("cancel of a pending task: status %d", code)
	}
	err = f.store.Checkpoints().Save(ctx, runtime.State{TaskID: pending, Role: "researcher", Status: runtime.Running})
	if !errors.Is(err, runtime.ErrCancelled) {
		t.Errorf("a cancelled pending task was started: err = %v", err)
	}
	var view struct {
		Task store.TaskRecord `json:"task"`
	}
	f.call(token, http.MethodGet, "/v1/tasks/"+pending, "", &view)
	if view.Task.Status != "cancelled" {
		t.Errorf("task status = %q, want cancelled", view.Task.Status)
	}
	if code, _ := f.call(token, http.MethodPost, "/v1/tasks/task_missing/cancel", "", nil); code != http.StatusNotFound {
		t.Errorf("cancel of an unknown task: status %d, want 404", code)
	}
}

// Artifact content is model output. It must reach a browser as inert text.
func TestArtifactContentIsServedAsPlainText(t *testing.T) {
	f := newFixture(t)
	body := []byte("<script>alert('x')</script>\n# Report")
	_ = f.blobs.Put(ctx, "artifacts/art_1", body)
	rec := artifacts.Artifact{ID: "art_1", Kind: "note", TaskID: "t", Role: "researcher", Title: "Report",
		Content: artifacts.Content{MediaType: "text/markdown", BlobKey: "artifacts/art_1"}, CreatedAt: "2026-11-13T09:00:00Z"}
	if err := f.store.Artifacts().Insert(ctx, rec); err != nil {
		t.Fatal(err)
	}
	var got string
	code, header := f.call(token, http.MethodGet, "/v1/artifacts/art_1/content", "", &got)
	if code != http.StatusOK || !bytes.Equal([]byte(got), body) {
		t.Fatalf("content: status %d, %q", code, got)
	}
	if ct := header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") || header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("Content-Type %q, X-Content-Type-Options %q; want text/plain and nosniff", ct, header.Get("X-Content-Type-Options"))
	}
	var record artifacts.Artifact
	if code, _ := f.call(token, http.MethodGet, "/v1/artifacts/art_1", "", &record); code != http.StatusOK || record.Title != "Report" {
		t.Errorf("record: status %d, %+v", code, record)
	}
	if code, _ := f.call(token, http.MethodGet, "/v1/artifacts/art_missing", "", nil); code != http.StatusNotFound {
		t.Errorf("missing artifact: status %d, want 404", code)
	}
}

func TestPauseBudgetAndConfig(t *testing.T) {
	f := newFixture(t)
	var control struct {
		Paused bool `json:"paused"`
	}
	f.call(token, http.MethodGet, "/v1/control", "", &control)
	if control.Paused {
		t.Fatal("a new deployment is paused")
	}
	if code, _ := f.call(token, http.MethodPost, "/v1/control", `{}`, nil); code != http.StatusBadRequest {
		t.Errorf("control without paused: status %d, want 400", code)
	}
	f.call(token, http.MethodPost, "/v1/control", `{"paused":true}`, nil)
	f.call(token, http.MethodGet, "/v1/control", "", &control)
	if paused, _ := f.store.Controls().Paused(ctx); !control.Paused || !paused {
		t.Error("pause did not take effect")
	}

	var budget struct {
		Currency string `json:"currency"`
		Limits   []struct {
			Category, Period, Limit, Spent string
		} `json:"limits"`
	}
	f.call(token, http.MethodGet, "/v1/budget", "", &budget)
	if budget.Currency != "INR" || len(budget.Limits) != 1 || budget.Limits[0].Limit != "150.000000" || budget.Limits[0].Spent != "0.000000" {
		t.Errorf("budget = %+v", budget)
	}

	var cfg struct {
		Digest          string `json:"digest"`
		ProfileApproved bool   `json:"profileApproved"`
		Roles           []struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
			Model   string `json:"model"`
		} `json:"roles"`
	}
	f.call(token, http.MethodGet, "/v1/config", "", &cfg)
	if cfg.Digest != "sha256:test" || cfg.ProfileApproved || len(cfg.Roles) != 2 ||
		cfg.Roles[0].Name != "researcher" || cfg.Roles[0].Model != "small" || cfg.Roles[1].Model != "large" {
		t.Errorf("config = %+v", cfg)
	}
}

// What went wrong inside stays inside: the caller learns only that it did.
func TestInternalErrorsAreNotLeaked(t *testing.T) {
	f := newFixture(t)
	f.wf.err = errors.New("dial tcp 10.0.0.7:7233: connection refused (password=hunter2)")
	var out string
	code, _ := f.call(token, http.MethodPost, "/v1/tasks", `{"role":"researcher"}`, &out)
	if code != http.StatusInternalServerError || strings.Contains(out, "10.0.0.7") || strings.Contains(out, "hunter2") {
		t.Errorf("status %d, body %q; want a 500 with no internal detail", code, out)
	}
}
