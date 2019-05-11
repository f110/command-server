package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/f110/command-server/pkg/config"
)

type command struct {
	Def       config.Command
	Exclusion bool
	mutex     sync.Mutex
}

type status struct {
	Name       string    `json:"name"`
	Args       []string  `json:"args"`
	Success    bool      `json:"success"`
	ExitCode   int       `json:"exit_code"`
	StartAt    time.Time `json:"start_at"`
	FinishedAt time.Time `json:"finished_at"`
}

type CommandServer struct {
	statusMutex sync.Mutex
	Status      map[int]*status
	lastId      int

	mux      *http.ServeMux
	commands map[string]*command

	crawlerInterval time.Duration
}

type NewCommandRequest struct {
	Name string   `json:"name"`
	Args []string `json:"args"`
}

func NewCommandServer(commands []config.Command) *CommandServer {
	m := make(map[string]*command)
	for _, v := range commands {
		if _, ok := m[v.Name]; ok {
			log.Printf("%s is duplicate", v.Name)
		}
		m[v.Name] = &command{Def: v, Exclusion: v.Exclusion}
	}
	s := &CommandServer{commands: m, Status: make(map[int]*status), crawlerInterval: 10 * time.Minute}

	mux := http.NewServeMux()
	mux.HandleFunc("/new", s.newCommand)
	mux.HandleFunc("/status/", s.status)
	s.mux = mux

	go s.crawler(context.Background())
	return s
}

func (cs *CommandServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	cs.mux.ServeHTTP(w, req)
}

func (cs *CommandServer) newCommand(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var reqObj NewCommandRequest
	if err := json.NewDecoder(req.Body).Decode(&reqObj); err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	cmd, ok := cs.commands[reqObj.Name]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err := cs.executeCommand(req.Context(), w, cmd, reqObj.Args); err != nil {
		if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == -1 {
			return
		}
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (cs *CommandServer) status(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s := strings.Split(req.URL.Path, "/")
	if len(s) < 2 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(s[2])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	cs.statusMutex.Lock()
	defer cs.statusMutex.Unlock()
	v, ok := cs.Status[id]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err := json.NewEncoder(w).Encode(v); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (cs *CommandServer) executeCommand(ctx context.Context, w http.ResponseWriter, cmd *command, args []string) error {
	if cmd.Exclusion {
		cmd.mutex.Lock()
		defer cmd.mutex.Unlock()
	}

	id := cs.getId()
	w.Header().Set("X-Status-Id", strconv.Itoa(id))
	w.Header().Set("Content-Type", "application/octet-stream")

	var commandArgs []string
	if len(cmd.Def.Command) > 1 {
		commandArgs = cmd.Def.Command[1:]
	}
	if len(args) > 0 {
		commandArgs = append(commandArgs, args...)
	}

	if cmd.Def.Timeout > 0 {
		ctxWithTimeout, cancelFunc := context.WithTimeout(ctx, time.Duration(cmd.Def.Timeout)*time.Second)
		defer cancelFunc()
		ctx = ctxWithTimeout
	}

	e := exec.CommandContext(ctx, cmd.Def.Command[0], commandArgs...)
	env := make([]string, 0, len(cmd.Def.Env))
	for k, v := range cmd.Def.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	e.Env = env
	writer := NewWriter(w)
	e.Stdout = writer
	e.Stderr = writer
	status := &status{Name: cmd.Def.Name, Args: args, StartAt: time.Now()}
	if err := e.Run(); err != nil {
		return err
	}
	status.Success = e.ProcessState.Success()
	status.ExitCode = e.ProcessState.ExitCode()
	status.FinishedAt = time.Now()
	cs.statusMutex.Lock()
	cs.Status[id] = status
	cs.statusMutex.Unlock()

	return nil
}

func (cs *CommandServer) getId() int {
	cs.statusMutex.Lock()
	defer cs.statusMutex.Unlock()

	cs.lastId++
	return cs.lastId
}

func (cs *CommandServer) crawler(ctx context.Context) {
	timer := time.NewTimer(0)
Crawler:
	for {
		select {
		case <-ctx.Done():
			break Crawler
		case <-timer.C:
			timer.Stop()

			cs.statusMutex.Lock()
			for k, v := range cs.Status {
				if v.FinishedAt.Before(time.Now().Add(-1 * time.Hour)) {
					delete(cs.Status, k)
				}
			}
			cs.statusMutex.Unlock()

			start := time.Now()
			d := start.Add(cs.crawlerInterval).Sub(time.Now())
			timer = time.NewTimer(d)
		}
	}

}
