package sql

import (
	"database/sql"
	"encoding/hex"
	"sync"
	"time"
)

var _createMptrieTable sync.Once

//InitTables init tables
func InitTables(db *sql.DB) error {
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS log_mptrie(id serial8, node_type varchar(2), node_key varchar(64) NOT NULL DEFAULT '', node_path varchar(64) NOT NULL DEFAULT '', node_children varchar(64) NOT NULL DEFAULT '', last_visited timestamp, PRIMARY KEY (id), UNIQUE (node_type,node_key,node_path,node_children))"); err != nil {
		return err
	}
	return nil
}

// StoreMptrieNode store mptrie node to db
func StoreMptrieNode(nodeType byte, nodeKey []byte, nodePath []byte, nodeChildren []byte) error {
	if !dbOpened {
		return nil
	}
	_, err := db.Exec("INSERT INTO log_mptrie (node_type, node_key, node_path, node_children,last_visited) VALUES ($1, $2, $3, $4,$5) ON CONFLICT (node_type,node_key,node_path,node_children) DO UPDATE SET last_visited = excluded.last_visited", string(nodeType), hex.EncodeToString(nodeKey), hex.EncodeToString(nodePath), hex.EncodeToString(nodeChildren), time.Now())
	return err
}
