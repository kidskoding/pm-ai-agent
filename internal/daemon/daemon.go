package daemon

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"pm-agent/internal/workflows"
	"pm-agent/pkg/git"
	"pm-agent/pkg/models"
	"pm-agent/pkg/scheduler"
)

type Daemon struct {
	grooming    *workflows.GroomingWorkflow
	standup     *workflows.StandupWorkflow
	stakeholder *workflows.StakeholderWorkflow
	sched       *scheduler.Scheduler
	Events      chan models.WorkflowRun
	lastHash    string
}

func New(
	g *workflows.GroomingWorkflow,
	st *workflows.StandupWorkflow,
	sh *workflows.StakeholderWorkflow,
) *Daemon {
	d := &Daemon{
		grooming:    g,
		standup:     st,
		stakeholder: sh,
		sched:       scheduler.New(),
		Events:      make(chan models.WorkflowRun, 20),
	}

	stHour, stMin := parseTime(os.Getenv("STANDUP_TIME"), 9, 0)
	d.sched.Add(scheduler.Job{
		Name:   "standup",
		Hour:   stHour,
		Minute: stMin,
		Fn:     d.runStandup,
	})

	monday := time.Monday
	d.sched.Add(scheduler.Job{
		Name:    "stakeholder",
		Hour:    9,
		Minute:  0,
		Weekday: &monday,
		Fn:      d.runStakeholder,
	})

	return d
}

// Run starts the scheduler and the git polling loop. Blocks until ctx is cancelled.
func (d *Daemon) Run(ctx context.Context) {
	go d.sched.Run(ctx)

	hash, _ := git.LatestHash()
	d.lastHash = hash

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.checkNewCommit(ctx)
		}
	}
}

func (d *Daemon) checkNewCommit(ctx context.Context) {
	hash, err := git.LatestHash()
	if err != nil || hash == d.lastHash {
		return
	}
	d.lastHash = hash
	go d.runGrooming(ctx)
}

func (d *Daemon) runGrooming(ctx context.Context) {
	d.emit("grooming", models.WorkflowRunning, "")
	if err := d.grooming.Run(ctx); err != nil {
		log.Printf("grooming workflow error: %v", err)
		d.emit("grooming", models.WorkflowFailed, err.Error())
		return
	}
	d.emit("grooming", models.WorkflowDone, "")
}

func (d *Daemon) runStandup(ctx context.Context) {
	d.emit("standup", models.WorkflowRunning, "")
	if err := d.standup.Run(ctx); err != nil {
		log.Printf("standup workflow error: %v", err)
		d.emit("standup", models.WorkflowFailed, err.Error())
		return
	}
	d.emit("standup", models.WorkflowDone, "")
}

func (d *Daemon) runStakeholder(ctx context.Context) {
	d.emit("stakeholder", models.WorkflowRunning, "")
	if err := d.stakeholder.Run(ctx); err != nil {
		log.Printf("stakeholder workflow error: %v", err)
		d.emit("stakeholder", models.WorkflowFailed, err.Error())
		return
	}
	d.emit("stakeholder", models.WorkflowDone, "")
}

func (d *Daemon) emit(name string, status models.WorkflowRunStatus, errMsg string) {
	select {
	case d.Events <- models.WorkflowRun{
		Name:    name,
		Status:  status,
		LastRun: time.Now(),
		Err:     errMsg,
	}:
	default:
		// channel full — drop event rather than block
	}
}

func parseTime(s string, defaultHour, defaultMinute int) (int, int) {
	if s == "" {
		return defaultHour, defaultMinute
	}
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return defaultHour, defaultMinute
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return defaultHour, defaultMinute
	}
	return h, m
}
