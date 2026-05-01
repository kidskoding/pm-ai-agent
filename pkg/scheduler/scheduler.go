package scheduler

import (
	"context"
	"time"
)

type Job struct {
	Name    string
	Hour    int
	Minute  int
	Weekday *time.Weekday // nil = every day
	Fn      func(context.Context)
}

type Scheduler struct {
	jobs []Job
}

func New() *Scheduler {
	return &Scheduler{}
}

func (s *Scheduler) Add(j Job) {
	s.jobs = append(s.jobs, j)
}

// Run checks every 30 seconds whether any job should fire.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	fired := map[string]time.Time{}

	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			for _, job := range s.jobs {
				if shouldFire(job, t, fired[job.Name]) {
					fired[job.Name] = t
					go job.Fn(ctx)
				}
			}
		}
	}
}

func shouldFire(job Job, now time.Time, lastFired time.Time) bool {
	if now.Hour() != job.Hour || now.Minute() != job.Minute {
		return false
	}
	if job.Weekday != nil && now.Weekday() != *job.Weekday {
		return false
	}
	if !lastFired.IsZero() && now.Sub(lastFired) < time.Minute {
		return false
	}
	return true
}
