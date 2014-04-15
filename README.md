gearmand
========

golang gearman-job-server clone

wip

	some benchmark results(GOMAXPROCS=2):

	[Current:   6994 jobs/s, Total:   4186 jobs/s]
	[Current:   2840 jobs/s, Total:   4186 jobs/s]
	[Current:    450 jobs/s, Total:   4184 jobs/s]
	[Current:   9751 jobs/s, Total:   4185 jobs/s]
	[Current:   3046 jobs/s, Total:   4185 jobs/s]
	[Current:    724 jobs/s, Total:   4183 jobs/s]
	[Current:  10481 jobs/s, Total:   4184 jobs/s]
	[Current:   3918 jobs/s, Total:   4184 jobs/s]
	[Current:   4118 jobs/s, Total:   4184 jobs/s]
	[Current:   4810 jobs/s, Total:   4184 jobs/s]
	
	some benchmark results(GOMAXPROCS=1):
	[Current:   8031 jobs/s, Total:   4662 jobs/s]
	[Current:   2478 jobs/s, Total:   4656 jobs/s]
	[Current:   7725 jobs/s, Total:   4662 jobs/s]
	[Current:   3025 jobs/s, Total:   4658 jobs/s]
	[Current:   6268 jobs/s, Total:   4661 jobs/s]
	[Current:   3381 jobs/s, Total:   4659 jobs/s]
	[Current:   1160 jobs/s, Total:   4651 jobs/s]
	[Current:   6639 jobs/s, Total:   4655 jobs/s]


	original c version:
	
	[Current:   8896 jobs/s, Total:   4764 jobs/s]
	[Current:   2976 jobs/s, Total:   4728 jobs/s]
	[Current:   5084 jobs/s, Total:   4739 jobs/s]
	[Current:  19970 jobs/s, Total:   4848 jobs/s]
	[Current:    180 jobs/s, Total:   4743 jobs/s]
	[Current:   9802 jobs/s, Total:   4783 jobs/s]
	[Current:   3163 jobs/s, Total:   4749 jobs/s]
	[Current:  20389 jobs/s, Total:   4843 jobs/s]
	[Current:    777 jobs/s, Total:   4755 jobs/s]
	[Current:   5097 jobs/s, Total:   4764 jobs/s]
	
	
	benchmark tools:
	gearmand-1.1.12/benchmark$ ./blobslap_client -c 1000 -n 10000
	gearmand-1.1.12/benchmark$ ./blobslap_worker

how to start gearmand?

	./gearmand --addr="0.0.0.0:4730"
	
then choose client librarys form

	http://gearman.org/download/


how to track stats:

	http://localhost:6060/debug/stats