package storage

import (
	. "github.com/ngaut/gearmand/common"
)

type JobQueue interface {
	AddJob(j *Job)
	DoneJob(j *Job)
	ReplayJobs() []*Job
}
