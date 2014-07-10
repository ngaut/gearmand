package sqlite3

import (
	"database/sql"
	"errors"
	_ "github.com/mattn/go-sqlite3"
	. "github.com/ngaut/gearmand/common"
	log "github.com/ngaut/logging"
)

const (
	createTableSQL = "CREATE TABLE IF NOT EXISTS job(Handle varchar(128),Id varchar(128),Priority INT, CreateAt TIMESTAMP,FuncName varchar(128),Data varchar(16384))"
	saveJobSQL     = "INSERT INTO job(Handle,Id,Priority,CreateAt,FuncName,Data) VALUES(?,?,?,?,?,?)"
	getJobsSQL     = "SELECT * FROM job" //need to get all jobs
	delJobSQL      = "DELETE FROM job WHERE Handle=?"
)

var (
	SQLite3NoDBErr = errors.New("can't get a sqlite3 db")
)

// SQLite3 Storage struct
type SQLite3Storage struct {
	db     *sql.DB
	Source string
}

func (self *SQLite3Storage) Init() error {
	var err error
	self.db, err = sql.Open("sqlite3", self.Source)
	if err != nil {
		log.Error(err)
		return err
	}

	_, create_err := self.db.Exec(createTableSQL)
	if create_err != nil {
		log.Error(create_err)
		return create_err
	}

	return self.db.Ping()
}

// Save implements the Storage Save method.
func (self *SQLite3Storage) AddJob(j *Job) error {
	_, err := self.db.Exec(saveJobSQL, j.Handle, j.Id, j.Priority, j.CreateAt.UTC(), j.FuncName, j.Data)
	if err != nil {
		log.Error(err)
		return err
	}

	return nil
}

// Get implements the Storage Get method.
func (self *SQLite3Storage) GetJobs() ([]*Job, error) {
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
func (self *SQLite3Storage) DoneJob(j *Job) error {
	log.Debug("DoneJob:", j.Handle)
	_, err := self.db.Exec(delJobSQL, j.Handle)
	if err != nil {
		log.Error(err, j.Handle)
		return err
	}

	return nil
}
