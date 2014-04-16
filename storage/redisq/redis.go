//using key as queue

package redisq

import (
	. "github.com/ngaut/gearmand/common"
	//"github.com/ngaut/gearmand/storage"
	"encoding/json"
	"flag"
	log "github.com/ngaut/logging"
	redis "github.com/vmihailenco/redis/v2"
)

type RedisQ struct {
	client *redis.Client
}

func (self *RedisQ) Init() error {
	log.Debug("init redis queue")
	addr := flag.Lookup("redis").Value.(flag.Getter).Get().(string)
	self.client = redis.NewTCPClient(&redis.Options{
		Addr:     addr,
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	_, err := self.client.Ping().Result()

	return err
}

func (self *RedisQ) AddJob(j *Job) error {
	buf, err := json.Marshal(j)
	if err != nil {
		return err
	}

	_, err = self.client.Set(j.Handle, string(buf)).Result()

	return err
}

func (self *RedisQ) DoneJob(j *Job) error {
	_, err := self.client.Del(j.Handle).Result()

	return err
}

func (self *RedisQ) GetJobs() ([]*Job, error) {
	strs, err := self.client.Keys(JobPrefix + "*").Result()
	if err != nil {
		return nil, err
	}

	if len(strs) == 0 { //no jobs on redis
		return nil, nil
	}

	vals, err := self.client.MGet(strs...).Result()
	if err != nil {
		return nil, err
	}

	jobs := make([]*Job, len(strs), len(strs))
	for i, s := range vals {
		err := json.Unmarshal([]byte(s.(string)), &jobs[i])
		if err != nil {
			log.Error(s)
			return nil, err
		}
	}

	return jobs, nil
}
