rm -f chain*.db
go run consensus_sim_server.go -cpuprofile=goprof.prof
pprof -top goprof.prof > goprof
rm -f goprof.prof

