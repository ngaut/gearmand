gearmand
========

golang gearman-job-server clone


[![Build Status](https://drone.io/github.com/ngaut/gearmand/status.png)](https://drone.io/github.com/ngaut/gearmand/latest)

[![Build Status](https://travis-ci.org/ngaut/gearmand.svg?branch=master)](https://travis-ci.org/ngaut/gearmand)

wip

	some benchmark results(GOMAXPROCS=2):
	...
	[Current:   6994 jobs/s, Total:   4186 jobs/s]
	[Current:   2840 jobs/s, Total:   4186 jobs/s]
	[Current:    450 jobs/s, Total:   4184 jobs/s]
	
	some benchmark results(GOMAXPROCS=1):
	...
	[Current:   8031 jobs/s, Total:   4662 jobs/s]
	[Current:   2478 jobs/s, Total:   4656 jobs/s]
	[Current:   7725 jobs/s, Total:   4662 jobs/s]


	original c version:
	...
	[Current:   8896 jobs/s, Total:   4764 jobs/s]
	[Current:   2976 jobs/s, Total:   4728 jobs/s]
	[Current:  19970 jobs/s, Total:   4848 jobs/s]
	
	
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
	
	
TODO:

	worker timeout 
	queue max length limit
	mysql support
	more tests
	rest api
	web monitor
	priority
	write design documents, data structs