package storage

import (
	. "github.com/ngaut/gearmand/common"
)

type JobQueue interface {
	AddJob(j *Job) error
	DoneJob(j *Job) error
	GetJobs() ([]*Job, error)
}
