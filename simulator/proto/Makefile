build:
	python -m grpc_tools.protoc -Isimulator --python_out=. --grpc_python_out=. simulator.proto
	protoc -I simulator/ simulator/simulator.proto --go_out=plugins=grpc:simulator
	sed -i.bak "s/import simulator_pb2 as simulator__pb2/from . import simulator_pb2 as simulator__pb2/g" simulator_pb2_grpc.py
