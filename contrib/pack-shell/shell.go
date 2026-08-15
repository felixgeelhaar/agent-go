// Package shell provides allowlisted command execution tools for agent-go.
//
// Security model (non-negotiable):
//   - No shell interpolation: commands run via exec.Command (argv), never sh -c
//   - argv[0] must match Config.Allowlist (basename or absolute path)
//   - Working directory is jailed under Config.BaseDir
//   - Output is capped; timeouts are enforced via context
//
// shell_script writes a temp file under BaseDir and runs an allowlisted
// interpreter. shell_exec_background starts a process and tracks it as a job
// until timeout or completion.
package shell

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// Common errors.
var (
	ErrNotAllowlisted = errors.New("command not in allowlist")
	ErrInvalidCwd     = errors.New("working directory outside BaseDir")
	ErrJobNotFound    = errors.New("background job not found")
)

// Config configures the shell pack. Allowlist and BaseDir are required.
type Config struct {
	// Allowlist is the set of permitted command names (basenames) or absolute paths.
	// Example: []string{"echo", "git", "/usr/bin/true"}
	Allowlist []string

	// BaseDir jails cwd and script temp files.
	BaseDir string

	// DefaultTimeout for foreground exec (default 30s).
	DefaultTimeout time.Duration

	// MaxOutputBytes caps combined stdout+stderr (default 1MiB).
	MaxOutputBytes int

	// MaxBackgroundJobs limits concurrent background processes (default 4).
	MaxBackgroundJobs int

	// BackgroundTimeout kills background jobs after this duration (default 5m).
	BackgroundTimeout time.Duration
}

// Pack returns shell tools. cfg.Allowlist and cfg.BaseDir are required.
func Pack(cfg Config) *pack.Pack {
	if len(cfg.Allowlist) == 0 {
		panic("shell.Pack: Allowlist is required")
	}
	if cfg.BaseDir == "" {
		panic("shell.Pack: BaseDir is required")
	}
	if cfg.DefaultTimeout <= 0 {
		cfg.DefaultTimeout = 30 * time.Second
	}
	if cfg.MaxOutputBytes <= 0 {
		cfg.MaxOutputBytes = 1 << 20
	}
	if cfg.MaxBackgroundJobs <= 0 {
		cfg.MaxBackgroundJobs = 4
	}
	if cfg.BackgroundTimeout <= 0 {
		cfg.BackgroundTimeout = 5 * time.Minute
	}
	abs, err := filepath.Abs(cfg.BaseDir)
	if err != nil {
		panic(fmt.Sprintf("shell.Pack: BaseDir: %v", err))
	}
	cfg.BaseDir = abs

	allowed := make(map[string]struct{}, len(cfg.Allowlist))
	for _, a := range cfg.Allowlist {
		allowed[a] = struct{}{}
	}
	p := &shellPack{cfg: cfg, allowed: allowed, jobs: make(map[string]*bgJob)}
	return pack.NewBuilder("shell").
		WithDescription("Allowlisted shell command execution tools").
		WithVersion("0.1.0").
		AddTools(
			p.shellExec(),
			p.shellExecBackground(),
			p.shellScript(),
		).
		AllowInState(agent.StateAct, "shell_exec", "shell_exec_background", "shell_script").
		Build()
}

type shellPack struct {
	cfg     Config
	allowed map[string]struct{}

	mu   sync.Mutex
	jobs map[string]*bgJob
	seq  int
}

type bgJob struct {
	ID      string
	PID     int
	Argv    []string
	Started time.Time
	cmd     *exec.Cmd
	cancel  context.CancelFunc
	done    chan struct{}
	result  *execResult
}

type execResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Error    string `json:"error,omitempty"`
	Duration string `json:"duration"`
}

func resultOK(v any) (tool.Result, error) {
	out, err := json.Marshal(v)
	if err != nil {
		return tool.Result{}, err
	}
	return tool.Result{Output: out}, nil
}

func decode[T any](raw json.RawMessage) (T, error) {
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, fmt.Errorf("%w: %v", tool.ErrInvalidInput, err)
	}
	return v, nil
}

func (p *shellPack) isAllowed(cmdPath string) bool {
	base := filepath.Base(cmdPath)
	if _, ok := p.allowed[cmdPath]; ok {
		return true
	}
	if _, ok := p.allowed[base]; ok {
		return true
	}
	return false
}

func (p *shellPack) resolveCwd(userCwd string) (string, error) {
	cwd := p.cfg.BaseDir
	if userCwd != "" {
		if filepath.IsAbs(userCwd) {
			cwd = filepath.Clean(userCwd)
		} else {
			cwd = filepath.Join(p.cfg.BaseDir, filepath.Clean(userCwd))
		}
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(p.cfg.BaseDir, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("%w: %s", ErrInvalidCwd, userCwd)
	}
	return abs, nil
}

func (p *shellPack) buildCmd(ctx context.Context, argv []string, cwd string, env []string) (*exec.Cmd, error) {
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return nil, fmt.Errorf("%w: argv is required", tool.ErrInvalidInput)
	}
	if !p.isAllowed(argv[0]) {
		return nil, fmt.Errorf("%w: %s", ErrNotAllowlisted, argv[0])
	}
	// Look up on PATH if basename; still only if basename is allowlisted.
	bin := argv[0]
	if !filepath.IsAbs(bin) {
		if resolved, err := exec.LookPath(bin); err == nil {
			bin = resolved
		}
	}
	cmd := exec.CommandContext(ctx, bin, argv[1:]...)
	cmd.Dir = cwd
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	return cmd, nil
}

func capOutput(b []byte, max int) string {
	if len(b) > max {
		return string(b[:max]) + "\n...[truncated]"
	}
	return string(b)
}

func (p *shellPack) runForeground(ctx context.Context, argv []string, cwd string, env []string, timeout time.Duration) (execResult, error) {
	if timeout <= 0 {
		timeout = p.cfg.DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd, err := p.buildCmd(ctx, argv, cwd, env)
	if err != nil {
		return execResult{}, err
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	start := time.Now()
	runErr := cmd.Run()
	res := execResult{
		Stdout:   capOutput(stdout.Bytes(), p.cfg.MaxOutputBytes),
		Stderr:   capOutput(stderr.Bytes(), p.cfg.MaxOutputBytes/4),
		Duration: time.Since(start).String(),
	}
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	if runErr != nil {
		res.Error = runErr.Error()
		// Still return structured result for non-zero exits.
		if _, ok := runErr.(*exec.ExitError); ok {
			return res, nil
		}
		if errors.Is(runErr, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			res.Error = "timeout"
			return res, nil
		}
		return res, runErr
	}
	return res, nil
}

func (p *shellPack) shellExec() tool.Tool {
	return tool.NewBuilder("shell_exec").
		WithDescription("Execute an allowlisted command (argv form, no shell) and return stdout/stderr").
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"argv":         json.RawMessage(`{"type":"array","items":{"type":"string"}}`),
			"cwd":          json.RawMessage(`{"type":"string"}`),
			"env":          json.RawMessage(`{"type":"array","items":{"type":"string"}}`),
			"timeout_secs": json.RawMessage(`{"type":"number"}`),
		}, []string{"argv"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decode[struct {
				Argv        []string `json:"argv"`
				Cwd         string   `json:"cwd"`
				Env         []string `json:"env"`
				TimeoutSecs float64  `json:"timeout_secs"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			cwd, err := p.resolveCwd(in.Cwd)
			if err != nil {
				return tool.Result{}, err
			}
			timeout := p.cfg.DefaultTimeout
			if in.TimeoutSecs > 0 {
				timeout = time.Duration(in.TimeoutSecs * float64(time.Second))
			}
			res, err := p.runForeground(ctx, in.Argv, cwd, in.Env, timeout)
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(res)
		}).
		MustBuild()
}

func (p *shellPack) shellExecBackground() tool.Tool {
	return tool.NewBuilder("shell_exec_background").
		WithDescription("Start an allowlisted command in the background; returns a job id").
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"argv": json.RawMessage(`{"type":"array","items":{"type":"string"}}`),
			"cwd":  json.RawMessage(`{"type":"string"}`),
			"env":  json.RawMessage(`{"type":"array","items":{"type":"string"}}`),
		}, []string{"argv"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decode[struct {
				Argv []string `json:"argv"`
				Cwd  string   `json:"cwd"`
				Env  []string `json:"env"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			cwd, err := p.resolveCwd(in.Cwd)
			if err != nil {
				return tool.Result{}, err
			}

			p.mu.Lock()
			active := 0
			for _, j := range p.jobs {
				select {
				case <-j.done:
				default:
					active++
				}
			}
			if active >= p.cfg.MaxBackgroundJobs {
				p.mu.Unlock()
				return tool.Result{}, fmt.Errorf("max background jobs (%d) reached", p.cfg.MaxBackgroundJobs)
			}
			p.seq++
			id := fmt.Sprintf("job-%d", p.seq)
			p.mu.Unlock()

			jobCtx, cancel := context.WithTimeout(context.Background(), p.cfg.BackgroundTimeout)
			cmd, err := p.buildCmd(jobCtx, in.Argv, cwd, in.Env)
			if err != nil {
				cancel()
				return tool.Result{}, err
			}
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Start(); err != nil {
				cancel()
				return tool.Result{}, err
			}
			job := &bgJob{
				ID:      id,
				PID:     cmd.Process.Pid,
				Argv:    append([]string{}, in.Argv...),
				Started: time.Now().UTC(),
				cmd:     cmd,
				cancel:  cancel,
				done:    make(chan struct{}),
			}
			p.mu.Lock()
			p.jobs[id] = job
			p.mu.Unlock()

			go func() {
				runErr := cmd.Wait()
				res := &execResult{
					Stdout:   capOutput(stdout.Bytes(), p.cfg.MaxOutputBytes),
					Stderr:   capOutput(stderr.Bytes(), p.cfg.MaxOutputBytes/4),
					Duration: time.Since(job.Started).String(),
				}
				if cmd.ProcessState != nil {
					res.ExitCode = cmd.ProcessState.ExitCode()
				}
				if runErr != nil {
					res.Error = runErr.Error()
				}
				job.result = res
				cancel()
				close(job.done)
			}()

			return resultOK(map[string]any{
				"job_id":  id,
				"pid":     job.PID,
				"argv":    in.Argv,
				"started": job.Started,
			})
		}).
		MustBuild()
}

func (p *shellPack) shellScript() tool.Tool {
	return tool.NewBuilder("shell_script").
		WithDescription("Write a script under BaseDir and run it with an allowlisted interpreter").
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"interpreter":  json.RawMessage(`{"type":"string"}`),
			"script":       json.RawMessage(`{"type":"string"}`),
			"cwd":          json.RawMessage(`{"type":"string"}`),
			"timeout_secs": json.RawMessage(`{"type":"number"}`),
		}, []string{"interpreter", "script"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decode[struct {
				Interpreter string  `json:"interpreter"`
				Script      string  `json:"script"`
				Cwd         string  `json:"cwd"`
				TimeoutSecs float64 `json:"timeout_secs"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if !p.isAllowed(in.Interpreter) {
				return tool.Result{}, fmt.Errorf("%w: %s", ErrNotAllowlisted, in.Interpreter)
			}
			cwd, err := p.resolveCwd(in.Cwd)
			if err != nil {
				return tool.Result{}, err
			}
			f, err := os.CreateTemp(p.cfg.BaseDir, "agent-script-*.sh")
			if err != nil {
				return tool.Result{}, err
			}
			scriptPath := f.Name()
			defer func() { _ = os.Remove(scriptPath) }()
			if _, err := f.WriteString(in.Script); err != nil {
				_ = f.Close()
				return tool.Result{}, err
			}
			if err := f.Close(); err != nil {
				return tool.Result{}, err
			}
			_ = os.Chmod(scriptPath, 0o700)

			timeout := p.cfg.DefaultTimeout
			if in.TimeoutSecs > 0 {
				timeout = time.Duration(in.TimeoutSecs * float64(time.Second))
			}
			res, err := p.runForeground(ctx, []string{in.Interpreter, scriptPath}, cwd, nil, timeout)
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(res)
		}).
		MustBuild()
}
