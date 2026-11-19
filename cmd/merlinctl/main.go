// Command merlinctl is the MerlinCI CLI: check configuration, start a task,
// and inspect its evidence, artifacts and cost (deck slides 14, 18 #13).
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/RoundtableCI/MerlinCI/internal/app"
	"github.com/RoundtableCI/MerlinCI/internal/config"
)

const usage = `merlinctl is the MerlinCI command line.

Check configuration (needs nothing running):
  merlinctl validate <path>...          Validate configuration files and directories

Operate a running MerlinCI (needs merlin-api; set MERLIN_API_URL and MERLIN_API_TOKEN):
  merlinctl run <role> [-input JSON] [-wait]   Start a task
  merlinctl campaign <workflow> [-input JSON] [-wait]   Start a campaign: a whole workflow
  merlinctl campaigns                   List recent campaigns
  merlinctl campaign-status <id>        Show where a campaign and each of its steps stand
  merlinctl tasks                       List recent tasks
  merlinctl status <task>               Show where a task stands
  merlinctl events <task>               Show everything a task did, in order
  merlinctl approve <task>              Let the action a task is waiting on go ahead
  merlinctl reject <task> [-reason R]   Refuse the action a task is waiting on
  merlinctl cancel <task>               Stop a task
  merlinctl artifact <id> [-content]    Show an artifact record, or its content
  merlinctl budget                      Show spend against the limits
  merlinctl pause | resume              Pause or resume all runs
  merlinctl config                      Show the configuration in force
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "validate":
		return validate(rest, stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	}

	c, err := newClient()
	if err != nil {
		fmt.Fprintln(stderr, "merlinctl:", err)
		return 1
	}
	switch cmd {
	case "run":
		err = c.run(rest, stdout)
	case "campaign":
		err = c.campaign(rest, stdout)
	case "campaigns":
		err = c.show(stdout, http.MethodGet, "/v1/campaigns", nil)
	case "campaign-status":
		err = c.withID(rest, func(id string) error { return c.show(stdout, http.MethodGet, "/v1/campaigns/"+id, nil) })
	case "tasks":
		err = c.show(stdout, http.MethodGet, "/v1/tasks", nil)
	case "status":
		err = c.withID(rest, func(id string) error { return c.show(stdout, http.MethodGet, "/v1/tasks/"+id, nil) })
	case "events":
		err = c.withID(rest, func(id string) error { return c.events(id, stdout) })
	case "approve":
		err = c.withID(rest, func(id string) error { return c.decide(id, true, "", stdout) })
	case "reject":
		err = c.reject(rest, stdout)
	case "cancel":
		err = c.withID(rest, func(id string) error { return c.show(stdout, http.MethodPost, "/v1/tasks/"+id+"/cancel", nil) })
	case "artifact":
		err = c.artifact(rest, stdout)
	case "budget":
		err = c.show(stdout, http.MethodGet, "/v1/budget", nil)
	case "pause":
		err = c.show(stdout, http.MethodPost, "/v1/control", map[string]any{"paused": true})
	case "resume":
		err = c.show(stdout, http.MethodPost, "/v1/control", map[string]any{"paused": false})
	case "config":
		err = c.show(stdout, http.MethodGet, "/v1/config", nil)
	default:
		fmt.Fprintf(stderr, "merlinctl: unknown command %q\n\n%s", cmd, usage)
		return 2
	}
	if err != nil {
		fmt.Fprintln(stderr, "merlinctl:", err)
		return 1
	}
	return 0
}

func validate(paths []string, stdout, stderr io.Writer) int {
	if len(paths) == 0 {
		fmt.Fprintln(stderr, "merlinctl validate: give at least one file or directory")
		return 2
	}
	bundle, problems := config.Load(paths...)
	for _, p := range problems {
		fmt.Fprintln(stderr, p)
	}
	summary := fmt.Sprintf("%d documents, %d errors, %d warnings",
		bundle.Documents(), problems.Count(config.Error), problems.Count(config.Warning))
	if problems.HasErrors() {
		fmt.Fprintln(stderr, "invalid: "+summary)
		return 1
	}
	if bundle.Documents() == 0 {
		fmt.Fprintln(stderr, "invalid: no configuration documents found")
		return 1
	}
	fmt.Fprintln(stdout, "valid: "+summary)
	fmt.Fprintln(stdout, "digest: "+bundle.Digest)
	return 0
}

// client talks to merlin-api.
type client struct {
	url   string
	token string
	http  *http.Client
}

func newClient() (*client, error) {
	env, err := app.LoadEnv()
	if err != nil {
		return nil, err
	}
	if env.APIToken == "" {
		return nil, fmt.Errorf("MERLIN_API_TOKEN is not set")
	}
	return &client{url: strings.TrimRight(env.APIURL, "/"), token: env.APIToken, http: &http.Client{Timeout: 60 * time.Second}}, nil
}

// do sends a request and returns the response body, or an error carrying
// the API's own message.
func (c *client) do(method, path string, body any) ([]byte, error) {
	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.url+path, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach merlin-api at %s: is it running?", c.url)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &e) == nil && e.Error != "" {
			return nil, fmt.Errorf("%s", e.Error)
		}
		return nil, fmt.Errorf("merlin-api returned status %d", resp.StatusCode)
	}
	return raw, nil
}

// show sends a request and prints the JSON response, indented.
func (c *client) show(w io.Writer, method, path string, body any) error {
	raw, err := c.do(method, path, body)
	if err != nil {
		return err
	}
	var out bytes.Buffer
	if json.Indent(&out, raw, "", "  ") != nil {
		_, err = w.Write(raw)
		return err
	}
	_, err = fmt.Fprintln(w, strings.TrimSpace(out.String()))
	return err
}

func (c *client) withID(args []string, fn func(id string) error) error {
	if len(args) != 1 {
		return fmt.Errorf("give exactly one ID")
	}
	return fn(args[0])
}

func (c *client) run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	input := fs.String("input", "{}", "task input as a JSON object")
	wait := fs.Bool("wait", false, "wait until the task finishes or needs a person")
	role, rest := "", args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		role, rest = args[0], args[1:]
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if role == "" {
		return fmt.Errorf("name the role to run, for example: merlinctl run market-researcher")
	}
	var in map[string]any
	if err := json.Unmarshal([]byte(*input), &in); err != nil {
		return fmt.Errorf("-input must be a JSON object: %w", err)
	}
	raw, err := c.do(http.MethodPost, "/v1/tasks", map[string]any{"role": role, "input": in})
	if err != nil {
		return err
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &created); err != nil {
		return err
	}
	fmt.Fprintln(stdout, created.ID)
	if !*wait {
		return nil
	}
	for {
		time.Sleep(2 * time.Second)
		raw, err := c.do(http.MethodGet, "/v1/tasks/"+created.ID, nil)
		if err != nil {
			return err
		}
		var view struct {
			Run *struct {
				Status string `json:"status"`
			} `json:"run"`
		}
		if err := json.Unmarshal(raw, &view); err != nil {
			return err
		}
		if view.Run != nil && view.Run.Status != "running" {
			return c.show(stdout, http.MethodGet, "/v1/tasks/"+created.ID, nil)
		}
	}
}

func (c *client) reject(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("reject", flag.ContinueOnError)
	reason := fs.String("reason", "", "why the action is refused; the agent is told")
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("give the task ID first")
	}
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	return c.decide(args[0], false, *reason, stdout)
}

func (c *client) artifact(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("artifact", flag.ContinueOnError)
	content := fs.Bool("content", false, "print the artifact's content instead of its record")
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("give the artifact ID first")
	}
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if !*content {
		return c.show(stdout, http.MethodGet, "/v1/artifacts/"+args[0], nil)
	}
	raw, err := c.do(http.MethodGet, "/v1/artifacts/"+args[0]+"/content", nil)
	if err != nil {
		return err
	}
	_, err = stdout.Write(raw)
	return err
}

func (c *client) events(id string, stdout io.Writer) error {
	raw, err := c.do(http.MethodGet, "/v1/tasks/"+id+"/events", nil)
	if err != nil {
		return err
	}
	var out struct {
		Events []struct {
			Step int            `json:"step"`
			Type string         `json:"type"`
			Data map[string]any `json:"data"`
			At   time.Time      `json:"at"`
		} `json:"events"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return err
	}
	for _, e := range out.Events {
		data, _ := json.Marshal(e.Data)
		fmt.Fprintf(stdout, "%s  step %-2d  %-22s %s\n", e.At.Format("15:04:05"), e.Step, e.Type, data)
	}
	return nil
}

func (c *client) campaign(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("campaign", flag.ContinueOnError)
	input := fs.String("input", "{}", "campaign input as a JSON object")
	wait := fs.Bool("wait", false, "wait until the campaign finishes")
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("name the workflow to run, for example: merlinctl campaign first-release")
	}
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	var in map[string]any
	if err := json.Unmarshal([]byte(*input), &in); err != nil {
		return fmt.Errorf("-input must be a JSON object: %w", err)
	}
	raw, err := c.do(http.MethodPost, "/v1/campaigns", map[string]any{"workflow": args[0], "input": in})
	if err != nil {
		return err
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &created); err != nil {
		return err
	}
	fmt.Fprintln(stdout, created.ID)
	if !*wait {
		return nil
	}
	for {
		time.Sleep(3 * time.Second)
		raw, err := c.do(http.MethodGet, "/v1/campaigns/"+created.ID, nil)
		if err != nil {
			return err
		}
		var view struct {
			State struct {
				Status string `json:"status"`
			} `json:"state"`
		}
		if err := json.Unmarshal(raw, &view); err != nil {
			return err
		}
		if view.State.Status != "running" {
			return c.show(stdout, http.MethodGet, "/v1/campaigns/"+created.ID, nil)
		}
	}
}

// decide approves or refuses the action a task is waiting on. It reads the
// pending action first, prints it, and sends its number with the decision,
// so the API applies the decision to that action and to no other.
func (c *client) decide(id string, approve bool, reason string, stdout io.Writer) error {
	raw, err := c.do(http.MethodGet, "/v1/tasks/"+id, nil)
	if err != nil {
		return err
	}
	var view struct {
		Run *struct {
			Pending *struct {
				Seq   int             `json:"seq"`
				Tool  string          `json:"tool"`
				Input json.RawMessage `json:"input"`
			} `json:"pending"`
		} `json:"run"`
	}
	if err := json.Unmarshal(raw, &view); err != nil {
		return err
	}
	if view.Run == nil || view.Run.Pending == nil {
		return fmt.Errorf("the task is not waiting for a decision")
	}
	p := view.Run.Pending
	verb := "approving"
	if !approve {
		verb = "refusing"
	}
	fmt.Fprintf(stdout, "%s: %s %s\n", verb, p.Tool, p.Input)
	return c.show(stdout, http.MethodPost, "/v1/tasks/"+id+"/decision",
		map[string]any{"seq": p.Seq, "approve": approve, "reason": reason})
}
