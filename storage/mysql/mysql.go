package mysql

import (
	"database/sql"
	"errors"
	_ "github.com/go-sql-driver/mysql"
	. "github.com/ngaut/gearmand/common"
	log "github.com/ngaut/logging"
)

const (
	saveJobSQL = "INSERT INTO job(Handle,Id,Priority,CreateAt,FuncName,Data) VALUES(?,?,?,?,?,?)"
	getJobsSQL = "SELECT * FROM job" //need to get all jobs
	delJobSQL  = "DELETE FROM job WHERE Handle==?"
)

var (
	MYSQLNoDBErr = errors.New("can't get a mysql db")
)

// MySQL Storage struct
type MYSQLStorage struct {
	db     *sql.DB
	Source string
}

func (self *MYSQLStorage) Init() error {
	var err error
	self.db, err = sql.Open("mysql", self.Source)
	if err != nil {
		log.Error(err)
		return err
	}

	return self.db.Ping()
}

// Save implements the Storage Save method.
func (self *MYSQLStorage) AddJob(j *Job) error {
	_, err := self.db.Exec(saveJobSQL, j.Handle, j.Id, j.Priority, j.CreateAt.UTC(), j.FuncName, j.Data)
	if err != nil {
		log.Error(err)
		return err
	}

	return nil
}

// Get implements the Storage Get method.
func (self *MYSQLStorage) GetJobs() ([]*Job, error) {
	rows, err := self.db.Query(getJobsSQL)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	defer rows.Close()

	jobs := make([]*Job, 0)

	for rows.Next() {
		j := &Job{}
		if err := rows.Scan(&j.Handle, &j.Id, &j.Priority, &j.CreateAt, &j.FuncName, &j.Data); err != nil {
			log.Error("rows.Scan() failed (%v)", err)
			return nil, err
		}
		jobs = append(jobs, j)
	}

	return jobs, nil
}

// DelKey implements the Storage DelKey method.
func (self *MYSQLStorage) DoneJob(j *Job) error {
	log.Debug("DoneJob:", j.Handle)
	_, err := self.db.Exec(delJobSQL, j.Handle)
	if err != nil {
		log.Error(err, j.Handle)
		return err
	}

	return nil
}
