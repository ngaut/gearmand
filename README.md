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
	
how to using redis as storage?
	
	./gearmand --addr="0.0.0.0:4730" --redis="localhost:6379"
	
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
	
## License
Go-MySQL-Driver is licensed under the [Mozilla Public License Version 2.0](https://raw.github.com/go-sql-driver/mysql/master/LICENSE)

Mozilla summarizes the license scope as follows:
> MPL: The copyleft applies to any files containing MPLed code.


That means:
  * You can **use** the **unchanged** source code both in private as also commercial
  * You **needn't publish** the source code of your library as long the files licensed under the MPL 2.0 are **unchanged**
  * You **must publish** the source code of any **changed files** licensed under the MPL 2.0 under a) the MPL 2.0 itself or b) a compatible license (e.g. GPL 3.0 or Apache License 2.0)

Please read the [MPL 2.0 FAQ](http://www.mozilla.org/MPL/2.0/FAQ.html) if you have further questions regarding the license.

You can read the full terms here: [LICENSE](https://raw.github.com/go-sql-driver/mysql/master/LICENSE)	