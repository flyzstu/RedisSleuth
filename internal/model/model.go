package model

import "time"

type Metadata struct {
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	Duration    string    `json:"duration"`
	Calculation string    `json:"calculation,omitempty"`
}

type Cluster struct {
	State      string `json:"state"`
	KnownNodes int    `json:"known_nodes"`
	Masters    int    `json:"masters"`
	Replicas   int    `json:"replicas"`
}

type Node struct {
	ID               string   `json:"id"`
	Addr             string   `json:"addr"`
	Role             string   `json:"role"`
	MasterID         string   `json:"master_id,omitempty"`
	Slots            []string `json:"slots,omitempty"`
	LinkState        string   `json:"link_state"`
	Flags            []string `json:"flags,omitempty"`
	ClusterState     string   `json:"cluster_state,omitempty"`
	ReplicationState string   `json:"replication_state,omitempty"`
}

type CPUStats struct {
	Node                string        `json:"node"`
	Role                string        `json:"role"`
	CPUPercent          float64       `json:"cpu_percent"`
	ChildrenCPUPercent  float64       `json:"children_cpu_percent"`
	OPS                 int64         `json:"ops"`
	CommandsDelta       int64         `json:"commands_delta"`
	CommandDeltas       []CommandStat `json:"command_deltas,omitempty"`
	Slowlog             []SlowEntry   `json:"slowlog,omitempty"`
	BGSaveActive        bool          `json:"bgsave_active"`
	AOFRewriteActive    bool          `json:"aof_rewrite_active"`
	FullSyncLikely      bool          `json:"full_sync_likely"`
	ReplicationAbnormal bool          `json:"replication_abnormal"`
}

type CommandStat struct {
	Name        string  `json:"name"`
	CallsDelta  int64   `json:"calls_delta"`
	UsecDelta   int64   `json:"usec_delta"`
	UsecPerCall float64 `json:"usec_per_call"`
}

type SlowEntry struct {
	ID       int64         `json:"id"`
	Time     time.Time     `json:"time"`
	Duration time.Duration `json:"duration"`
	Command  string        `json:"command"`
	Client   string        `json:"client,omitempty"`
	Name     string        `json:"name,omitempty"`
}

type MemoryStats struct {
	Node               string  `json:"node"`
	Role               string  `json:"role"`
	Master             string  `json:"master,omitempty"`
	UsedMemory         int64   `json:"used_memory"`
	UsedMemoryRSS      int64   `json:"used_memory_rss"`
	UsedMemoryPeak     int64   `json:"used_memory_peak"`
	MaxMemory          int64   `json:"maxmemory"`
	TotalSystemMemory  int64   `json:"total_system_memory"`
	MemoryPercent      float64 `json:"memory_percent"`
	MemoryPercentBasis string  `json:"memory_percent_basis"`
	FragmentationRatio float64 `json:"mem_fragmentation_ratio"`
	AllocatorFragRatio float64 `json:"allocator_frag_ratio"`
	AllocatorRSSRatio  float64 `json:"allocator_rss_ratio"`
	ClientsNormal      int64   `json:"mem_clients_normal"`
	ClientsReplicas    int64   `json:"mem_clients_slaves"`
	ReplicationBacklog int64   `json:"mem_replication_backlog"`
	EvictedKeysDelta   int64   `json:"evicted_keys_delta"`
	ExpiredKeysDelta   int64   `json:"expired_keys_delta"`
}

type SlotStats struct {
	Master       string       `json:"master"`
	Ranges       []string     `json:"ranges"`
	SlotCount    int          `json:"slot_count"`
	KeyCount     int64        `json:"key_count"`
	UsedMemory   int64        `json:"used_memory"`
	OPS          int64        `json:"ops"`
	Skewed       bool         `json:"skewed"`
	SampledSlots []SlotSample `json:"sampled_slots,omitempty"`
}

type SlotSample struct {
	Slot        int   `json:"slot"`
	KeyCount    int   `json:"key_count"`
	MemoryBytes int64 `json:"memory_bytes"`
}

type KeySample struct {
	Key         string    `json:"key"`
	Type        string    `json:"type"`
	Slot        int       `json:"slot"`
	MemoryBytes int64     `json:"memory_bytes"`
	Elements    int64     `json:"elements"`
	TTLMillis   int64     `json:"ttl_millis"`
	Master      string    `json:"master"`
	ScanNode    string    `json:"scan_node"`
	SampledAt   time.Time `json:"sampled_at"`
}

type Client struct {
	IP        string `json:"ip"`
	Port      string `json:"port"`
	Name      string `json:"name"`
	Age       int64  `json:"age"`
	Idle      int64  `json:"idle"`
	Flags     string `json:"flags"`
	DB        int    `json:"db"`
	Command   string `json:"command"`
	InputBuf  int64  `json:"input_buffer"`
	OutputBuf int64  `json:"output_buffer"`
}

type ClientAggregate struct {
	IP              string         `json:"ip"`
	Connections     int            `json:"connections"`
	Active          int            `json:"active"`
	Idle            int            `json:"idle"`
	Commands        map[string]int `json:"commands"`
	BufferBytes     int64          `json:"buffer_bytes"`
	Storm           bool           `json:"connection_storm"`
	ConnectionDelta int            `json:"connection_delta,omitempty"`
}

type Finding struct {
	Time           time.Time      `json:"time"`
	Severity       string         `json:"severity"`
	Category       string         `json:"category"`
	Node           string         `json:"node,omitempty"`
	Slot           *int           `json:"slot,omitempty"`
	ClientIP       string         `json:"client_ip,omitempty"`
	Key            string         `json:"key,omitempty"`
	Evidence       map[string]any `json:"evidence"`
	Conclusion     string         `json:"conclusion"`
	Recommendation string         `json:"recommendation"`
	Confidence     string         `json:"confidence"`
}

type Report struct {
	Metadata        Metadata          `json:"metadata"`
	Cluster         Cluster           `json:"cluster"`
	Nodes           []Node            `json:"nodes"`
	CPU             []CPUStats        `json:"cpu,omitempty"`
	Memory          []MemoryStats     `json:"memory,omitempty"`
	Slots           []SlotStats       `json:"slots,omitempty"`
	Clients         []ClientAggregate `json:"clients,omitempty"`
	ClientDetails   []Client          `json:"client_details,omitempty"`
	Keys            []KeySample       `json:"keys,omitempty"`
	Findings        []Finding         `json:"findings"`
	Recommendations []string          `json:"recommendations"`
}
