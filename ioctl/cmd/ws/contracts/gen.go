package contracts

//go:generate abigen --abi abis/W3bstreamProver.json --pkg contracts --type W3bstreamProver -out ./w3bstreamprover.go
//go:generate abigen --abi abis/W3bstreamProject.json --pkg contracts --type W3bstreamProject -out ./w3bstreamproject.go
//go:generate abigen --abi abis/ProjectRegistrar.json --pkg contracts --type ProjectRegistrar -out ./projectregistrar.go
//go:generate abigen --abi abis/FleetManagement.json --pkg contracts --type FleetManagement -out ./fleetmanagement.go
//go:generate abigen --abi abis/ProjectDevice.json --pkg contracts --type ProjectDevice -out ./projectdevice.go
//go:generate abigen --abi abis/W3bstreamRouter.json --pkg contracts --type W3bstreamRouter -out ./w3bstreamrouter.go
//go:generate abigen --abi abis/W3bstreamVMType.json --pkg contracts --type W3bstreamVMType -out ./w3bstreamvmtype.go
