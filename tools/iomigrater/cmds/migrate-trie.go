package cmd

import (
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/schollz/progressbar/v2"
	"github.com/spf13/cobra"
	"go.etcd.io/bbolt"

	"github.com/iotexproject/iotex-core/tools/iomigrater/common"
)

// Multi-language support
var (
	migrateTrieCmdShorts = map[string]string{
		"english": "Sub-Command for migration IoTeX trie db file.",
		"chinese": "迁移IoTeX trie db 文件的子命令",
	}
	migrateTrieCmdLongs = map[string]string{
		"english": "Sub-Command for migration IoTeX trie db file from bolt db to pebble db.",
		"chinese": "迁移IoTeX trie db 文件从 bolt db 到 pebble db",
	}
	migrateTrieCmdUse = map[string]string{
		"english": "migratetrie",
		"chinese": "migratetrie",
	}
	migrateTrieFlagoldTrieFileUse = map[string]string{
		"english": "The file you want to migrate.",
		"chinese": "您要迁移的文件。",
	}
	migrateTrieFlagnewTrieFileUse = map[string]string{
		"english": "The path you want to migrate to",
		"chinese": "您要迁移到的路径。",
	}
)

var (
	// MigrateTrie Used to Sub command.
	MigrateTrie = &cobra.Command{
		Use:   common.TranslateInLang(migrateTrieCmdUse),
		Short: common.TranslateInLang(migrateTrieCmdShorts),
		Long:  common.TranslateInLang(migrateTrieCmdLongs),
		RunE: func(cmd *cobra.Command, args []string) error {
			return migrateTrieFile()
		},
	}
)

var (
	oldTrieFile = ""
	newTrieFile = ""
)

func init() {
	MigrateTrie.PersistentFlags().StringVarP(&oldTrieFile, "old-file", "o", "", common.TranslateInLang(migrateTrieFlagoldTrieFileUse))
	MigrateTrie.PersistentFlags().StringVarP(&newTrieFile, "new-file", "n", "", common.TranslateInLang(migrateTrieFlagnewTrieFileUse))
}

func migrateTrieFile() (err error) {
	// Check flags
	if oldTrieFile == "" {
		return fmt.Errorf("--old-file is empty")
	}
	if newTrieFile == "" {
		return fmt.Errorf("--new-file is empty")
	}
	if oldTrieFile == newTrieFile {
		return fmt.Errorf("the values of --old-file --new-file flags cannot be the same")
	}
	size := 200000

	newdb, err := pebble.Open(newTrieFile, nil)
	if err != nil {
		return err
	}
	defer newdb.Close()
	batch := newdb.NewBatchWithSize(size)
	triedb, err := bbolt.Open(oldTrieFile, 0666, nil)
	if err != nil {
		return err
	}
	defer triedb.Close()
	keys := [][]byte{}
	if err := triedb.View(func(tx *bbolt.Tx) error {
		if err := tx.ForEach(func(name []byte, b *bbolt.Bucket) error {
			fmt.Printf("migrating namespace: %s %d\n", name, b.Stats().KeyN)
			bar := progressbar.New(b.Stats().KeyN + size)
			if err := b.ForEach(func(k, v []byte) error {
				if v == nil {
					panic("unexpected nested bucket")
				}
				keys = append(keys, append(name, k...))
				if err := batch.Set(append(name, k...), v, nil); err != nil {
					return err
				}
				if batch.Count() >= uint32(size) {
					if err := bar.Add(size); err != nil {
						return err
					}
					if err := newdb.Apply(batch, nil); err != nil {
						return err
					}
					batch = newdb.NewBatchWithSize(size)
				}
				return nil
			}); err != nil {
				return err
			}
			if err := bar.Finish(); err != nil {
				return err
			}
			fmt.Println()
			return nil
		}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	if batch.Count() > 0 {
		if err := newdb.Apply(batch, nil); err != nil {
			return err
		}
	}
	if err := newdb.Flush(); err != nil {
		return err
	}
	return nil
}
