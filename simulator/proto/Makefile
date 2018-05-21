build:
	python -m grpc_tools.protoc -Isimulator --python_out=. --grpc_python_out=. simulator.proto
	protoc -I simulator/ simulator/simulator.proto --go_out=plugins=grpc:simulator

runserver:
	go run simulator_server.go

runclient:
	python simulator_client.py
