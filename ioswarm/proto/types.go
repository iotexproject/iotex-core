// Package proto contains the IOSwarm gRPC types.
// In production, these would be generated from ioswarm.proto via protoc.
// For MVP, we define them manually to avoid the protoc dependency.
package proto

// TaskLevel represents validation complexity levels.
type TaskLevel int32

const (
	TaskLevel_L1_SIG_VERIFY   TaskLevel = 0
	TaskLevel_L2_STATE_VERIFY TaskLevel = 1
	TaskLevel_L3_FULL_EXECUTE TaskLevel = 2
	TaskLevel_L4_STATE_SYNC   TaskLevel = 3
)

// AccountSnapshot holds a point-in-time view of an account.
type AccountSnapshot struct {
	Address  string `json:"address"`
	Balance  string `json:"balance"` // big.Int as string
	Nonce    uint64 `json:"nonce"`
	CodeHash []byte `json:"code_hash,omitempty"`
}

// BlockCtx carries block-level context needed for EVM execution.
type BlockCtx struct {
	Timestamp uint64 `json:"timestamp"`
	GasLimit  uint64 `json:"gas_limit"`
	BaseFee   string `json:"base_fee"`  // big.Int as string
	Coinbase  string `json:"coinbase"`  // block producer address
	Number    uint64 `json:"number"`
}

// EvmTx carries structured EVM transaction fields.
type EvmTx struct {
	To       string `json:"to,omitempty"` // empty = contract creation
	Value    string `json:"value"`        // big.Int as string
	Data     []byte `json:"data,omitempty"`
	GasLimit uint64 `json:"gas_limit"`
	GasPrice string `json:"gas_price"` // big.Int as string
}

// StateChange records a single storage mutation during EVM execution.
type StateChange struct {
	Address  string `json:"address"`
	Slot     string `json:"slot"`      // hex
	OldValue string `json:"old_value"` // hex
	NewValue string `json:"new_value"` // hex
}

// LogEntry records a single EVM log emitted during execution.
type LogEntry struct {
	Address string   `json:"address"`
	Topics  []string `json:"topics"`
	Data    []byte   `json:"data"`
}

// TaskPackage is a single transaction validation task.
type TaskPackage struct {
	TaskID      uint32           `json:"task_id"`
	TxRaw       []byte           `json:"tx_raw"`
	Level       TaskLevel        `json:"level"`
	Sender      *AccountSnapshot `json:"sender"`
	Receiver    *AccountSnapshot `json:"receiver"`
	BlockHeight uint64           `json:"block_height"`

	// L3 EVM execution fields
	BlockContext *BlockCtx            `json:"block_context,omitempty"`
	ContractCode map[string][]byte    `json:"contract_code,omitempty"`   // address → bytecode
	StorageSlots map[string]map[string]string `json:"storage_slots,omitempty"` // address → slot → value (hex)
	EvmTx        *EvmTx               `json:"evm_tx,omitempty"`
}

// TaskBatch is a batch of tasks sent to an agent.
type TaskBatch struct {
	BatchID   string         `json:"batch_id"`
	Tasks     []*TaskPackage `json:"tasks"`
	Timestamp uint64         `json:"timestamp"`
}

// TaskResult is the validation result for a single task.
type TaskResult struct {
	TaskID       uint32 `json:"task_id"`
	Valid        bool   `json:"valid"`
	RejectReason string `json:"reject_reason,omitempty"`
	GasEstimate  uint64 `json:"gas_estimate"`
	LatencyUs    uint64 `json:"latency_us"`

	// L3 EVM execution results
	GasUsed      uint64         `json:"gas_used,omitempty"`
	ReturnData   []byte         `json:"return_data,omitempty"`
	StateChanges []*StateChange `json:"state_changes,omitempty"`
	Logs         []*LogEntry    `json:"logs,omitempty"`
	ExecError    string         `json:"exec_error,omitempty"`
}

// BatchResult is a batch of results from an agent.
type BatchResult struct {
	AgentID   string        `json:"agent_id"`
	BatchID   string        `json:"batch_id"`
	Results   []*TaskResult `json:"results"`
	Timestamp uint64        `json:"timestamp"`
}

// RegisterRequest is sent by agents to register with the coordinator.
type RegisterRequest struct {
	AgentID       string    `json:"agent_id"`
	Capability    TaskLevel `json:"capability"`
	Region        string    `json:"region"`
	Version       string    `json:"version"`
	WalletAddress string    `json:"wallet_address,omitempty"`
}

// RegisterResponse is sent by the coordinator in reply to registration.
type RegisterResponse struct {
	Accepted             bool   `json:"accepted"`
	Reason               string `json:"reason,omitempty"`
	HeartbeatIntervalSec uint32 `json:"heartbeat_interval_sec"`
}

// GetTasksRequest is sent by agents to start receiving task streams.
type GetTasksRequest struct {
	AgentID      string    `json:"agent_id"`
	MaxLevel     TaskLevel `json:"max_level"`
	MaxBatchSize uint32    `json:"max_batch_size"`
}

// SubmitResponse is the coordinator's response to submitted results.
type SubmitResponse struct {
	Accepted bool   `json:"accepted"`
	Reason   string `json:"reason,omitempty"`
}

// HeartbeatRequest is the periodic health check from agents.
type HeartbeatRequest struct {
	AgentID        string  `json:"agent_id"`
	TasksProcessed uint32  `json:"tasks_processed"`
	TasksPending   uint32  `json:"tasks_pending"`
	CPUUsage       float64 `json:"cpu_usage"`
	MemUsage       float64 `json:"mem_usage"`
}

// HeartbeatResponse is the coordinator's heartbeat reply.
type HeartbeatResponse struct {
	Alive     bool   `json:"alive"`
	Directive string `json:"directive"` // "continue", "drain", "shutdown"

	// Payout info (set when a new epoch payout was distributed since last heartbeat)
	Payout *PayoutInfo `json:"payout,omitempty"`
}

// PayoutInfo carries per-agent payout data from coordinator to agent.
type PayoutInfo struct {
	Epoch       uint64  `json:"epoch"`
	AmountIOTX  float64 `json:"amount_iotx"`
	Rank        int     `json:"rank"`
	TotalAgents int     `json:"total_agents"`
}

// StreamStateDiffsRequest is sent by L4 agents to subscribe to state diffs.
type StreamStateDiffsRequest struct {
	AgentID    string `json:"agent_id"`
	FromHeight uint64 `json:"from_height"` // resume from this height (0 = latest only)
}

// StateDiffResponse is a single block's state diff streamed to agents.
type StateDiffResponse struct {
	Height      uint64            `json:"height"`
	Entries     []*StateDiffEntry `json:"entries"`
	DigestBytes []byte            `json:"digest_bytes"` // for verification
}

// StateDiffEntry is a single state mutation in a block.
type StateDiffEntry struct {
	WriteType uint8  `json:"write_type"` // 0=Put, 1=Delete
	Namespace string `json:"namespace"`
	Key       []byte `json:"key"`
	Value     []byte `json:"value,omitempty"`
}

// DownloadSnapshotRequest is sent by L4 agents to download a full state snapshot.
type DownloadSnapshotRequest struct {
	AgentID string `json:"agent_id"`
}

// SnapshotChunk is a chunk of snapshot data streamed to agents.
type SnapshotChunk struct {
	ChunkIndex  uint32 `json:"chunk_index"`
	Data        []byte `json:"data"`
	TotalChunks uint32 `json:"total_chunks"` // 0 until last chunk
	Height      uint64 `json:"height"`       // snapshot height
}

// ============== L5 Block Building Types ==============

// BlockCandidate is a block built by an L5 agent, submitted to the coordinator.
type BlockCandidate struct {
	AgentID      string          `json:"agent_id"`
	BlockHeight  uint64          `json:"block_height"`
	BlockProto    []byte         `json:"block_proto"`    // serialized iotextypes.Block
	Receipts      []*ReceiptData `json:"receipts"`
	GasUsed       uint64         `json:"gas_used"`
	TxCount       int            `json:"tx_count"`
	Timestamp     uint64         `json:"timestamp"`
	Signature     []byte         `json:"signature"`      // agent's signature on block hash
	BlockHash     []byte         `json:"block_hash"`     // block hash for verification
}

// ReceiptData is a simplified receipt for transmission.
type ReceiptData struct {
	Status           uint64   `json:"status"`
	GasUsed          uint64   `json:"gas_used"`
	ContractAddress  string   `json:"contract_address"`
	Logs             []*LogEntry `json:"logs"`
}

// SubmitBlockRequest is sent by L5 agents to submit a built block.
type SubmitBlockRequest struct {
	AgentID string          `json:"agent_id"`
	Candidate *BlockCandidate `json:"candidate"`
}

// SubmitBlockResponse is the coordinator's response to block submission.
type SubmitBlockResponse struct {
	Accepted bool   `json:"accepted"`
	Reason   string `json:"reason,omitempty"`
	Reward   float64 `json:"reward,omitempty"` // IOTX reward if accepted
}

// SystemActionVerifyResponse is the verification result.
type SystemActionVerifyResponse struct {
	Valid         bool   `json:"valid"`
	InvalidIndices []int `json:"invalid_indices,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

// ============== L5 gRPC Request/Response Types ==============

// PendingAction represents a pending action from actpool for block template.
type PendingAction struct {
	Hash     string `json:"hash"`
	From     string `json:"from"`
	To       string `json:"to"`
	Nonce    uint64 `json:"nonce"`
	GasLimit uint64 `json:"gas_limit"`
	GasPrice string `json:"gas_price"` // big.Int as string
	RawBytes []byte `json:"raw_bytes"` // serialized SealedEnvelope
}

// BlockContext provides block-level context for EVM execution.
type BlockContext struct {
	GasLimit uint64 `json:"gas_limit"`
	BaseFee  string `json:"base_fee"` // big.Int as string
	Coinbase string `json:"coinbase"` // block producer address
}

// BlockTemplate is the response containing everything an agent needs to build a block.
type BlockTemplate struct {
	BlockHeight    uint64            `json:"block_height"`
	ParentHash     []byte            `json:"parent_hash"`
	Timestamp      uint64            `json:"timestamp"`
	ProducerAddress string           `json:"producer_address"`
	PendingActions []*PendingAction  `json:"pending_actions"`
	BlockContext   *BlockContext     `json:"block_context"`
}

// GetBlockTemplateRequest is sent by L5 agents to request a block template.
type GetBlockTemplateRequest struct {
	AgentID         string `json:"agent_id"`
	LastBuiltHeight uint64 `json:"last_built_height"`
}

// SubmitBlockCandidateRequest is sent by L5 agents to submit a built block.
type SubmitBlockCandidateRequest struct {
	AgentID   string          `json:"agent_id"`
	Candidate *BlockCandidate `json:"candidate"`
}

// SubmitBlockCandidateResponse is the coordinator's response to block submission.
type SubmitBlockCandidateResponse struct {
	Accepted bool    `json:"accepted"`
	Reason   string  `json:"reason,omitempty"`
	Reward   float64 `json:"reward,omitempty"`
}

// RegisterL5Request is sent by L5 agents to register as block builders.
type RegisterL5Request struct {
	AgentID          string `json:"agent_id"`
	WalletAddress    string `json:"wallet_address"`
	Region           string `json:"region"`
	Version          string `json:"version"`
	CapabilityFlags uint32 `json:"capability_flags"`
}

// RegisterL5Response is the coordinator's response to L5 registration.
type RegisterL5Response struct {
	Accepted            bool   `json:"accepted"`
	Reason              string `json:"reason,omitempty"`
	SelectionTimeoutMs uint32 `json:"selection_timeout_ms"`
}

// HeartbeatL5Request is the periodic health check from L5 agents.
type HeartbeatL5Request struct {
	AgentID          string  `json:"agent_id"`
	BlocksBuilt      uint64  `json:"blocks_built"`
	BlocksAccepted   uint64  `json:"blocks_accepted"`
	LastBuiltHeight  uint64  `json:"last_built_height"`
	CPUUsage         float64 `json:"cpu_usage"`
	MemUsage         float64 `json:"mem_usage"`
}

// HeartbeatL5Response is the coordinator's heartbeat reply to L5 agents.
type HeartbeatL5Response struct {
	Alive     bool       `json:"alive"`
	Directive string     `json:"directive"` // "continue", "pause", "shutdown"
	Payout    *PayoutInfo `json:"payout,omitempty"`
}
