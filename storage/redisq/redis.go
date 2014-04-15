//using hset as storage
//name of hset: gearmand-bg-queue

package redisq

import (
	. "github.com/ngaut/gearmand/common"
	//"github.com/ngaut/gearmand/storage"
)

type RedisQ struct {
}

func (self *RedisQ) AddJob(j *Job) error {
	return nil
}

func (self *RedisQ) DoneJob(j *Job) error {
	return nil
}

func (self *RedisQ) GetJobs() ([]*Job, error) {
	return nil, nil
}
