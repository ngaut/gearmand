gearmand
========

golang gearman-job-server clone


[![Build Status](https://drone.io/github.com/ngaut/gearmand/status.png)](https://drone.io/github.com/ngaut/gearmand/latest)
[![Coverage Status](https://coveralls.io/repos/ngaut/gearmand/badge.png?branch=master)](https://coveralls.io/r/ngaut/gearmand)


	some benchmark results(GOMAXPROCS=2):
	...
	[Total:   4186 jobs/s]
	
	some benchmark results(GOMAXPROCS=1):
	...
	[4656 jobs/s]


	original c version:
	...
	[5002 jobs/s]
	
	
	benchmark tools:
	gearmand-1.1.12/benchmark$ ./blobslap_client -c 1000 -n 10000
	gearmand-1.1.12/benchmark$ ./blobslap_worker

how to start gearmand?

	./gearmand --addr="0.0.0.0:4730"
	
## how to using redis as storage?
	
	./gearmand --storage=redis --addr="0.0.0.0:4730" --redis="localhost:6379"
	
## how to using mysql as storage?

	1. create database
		mysql> create database gogearmand;
		Query OK, 1 row affected (0.00 sec)
		
		mysql> use gogearmand;
	
	2. create table
		mysql> CREATE TABLE job(Handle varchar(128),Id varchar(128),Priority INT, CreateAt TIMESTAMP,FuncName varchar(128),Data varchar(16384)) ENGINE=InnoDB;
	
	./gearmand --storage=mysql --addr="0.0.0.0:4730" --source=user:password@tcp(localhost:3306)/gogearmand?parseTime=true
	
then choose client librarys form

	http://gearman.org/download/


how to track stats:

	http://localhost:6060/debug/stats
	
how to list workers by "cando" ?

	http://localhost:6060/worker/function
	
how to list all workers ?

	http://localhost:6060/worker

how to query job status ?

	http://localhost:6060/job/jobhandle
	
how to list all jobs ?

	http://localhost:6060/job
		
	
## TODO:

	worker timeout 
	queue max length limit
	more tests
	rest api
	web monitor
	priority
	write design documents, data structs
	
## LICENSE

gearmand is distributed under the terms of the MIT License. See [LICENSE](https://github.com/coocood/jas/blob/master/LICENSE) for details.