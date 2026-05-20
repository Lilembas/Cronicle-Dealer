package worker

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/cronicle/cronicle-dealer/pkg/logger"
	"go.uber.org/zap"
)

// NodeIDStore hostname → node_id 持久化映射
type NodeIDStore struct {
	mu       sync.RWMutex
	FilePath string              `json:"-"`
	Entries  map[string]string   `json:"entries"`
}

// LoadNodeIDStore 从文件加载 node_id 映射
func LoadNodeIDStore(path string) (*NodeIDStore, error) {
	store := &NodeIDStore{
		FilePath: path,
		Entries:  make(map[string]string),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info("node_id 文件不存在，将创建新文件", zap.String("path", path))
			return store, nil
		}
		return store, err
	}

	if len(data) == 0 {
		return store, nil
	}

	if err := json.Unmarshal(data, store); err != nil {
		return store, err
	}

	if store.Entries == nil {
		store.Entries = make(map[string]string)
	}

	return store, nil
}

// SaveNodeIDStore 持久化 node_id 映射到文件
func SaveNodeIDStore(path string, store *NodeIDStore) error {
	store.mu.RLock()
	defer store.mu.RUnlock()

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GetNodeIDForHostname 查询 hostname 对应的 node_id
func GetNodeIDForHostname(store *NodeIDStore, hostname string) string {
	if store == nil {
		return ""
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.Entries[hostname]
}

// SetNodeIDForHostname 设置 hostname 对应的 node_id
func SetNodeIDForHostname(store *NodeIDStore, hostname, nodeID string) {
	if store == nil {
		return
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.Entries[hostname] = nodeID
}
