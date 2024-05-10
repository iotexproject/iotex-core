package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/schollz/progressbar/v2"
	"github.com/spf13/cobra"
	"go.etcd.io/bbolt"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/db/trie/mptrie"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/tools/iomigrater/common"
)

// Multi-language support
var (
	stateDB2FactoryCmdShorts = map[string]string{
		"english": "Sub-Command for migration state db to factory db.",
		"chinese": "迁移 IoTeX 区块链 state db 到 factory db 的子命令",
	}
	stateDB2FactoryCmdLongs = map[string]string{
		"english": "Sub-Command for migration state db to factory db.",
		"chinese": "迁移 IoTeX 区块链 state db 到 factory db 的子命令",
	}
	stateDB2FactoryCmdUse = map[string]string{
		"english": "state2factory",
		"chinese": "state2factory",
	}
	stateDB2FactoryFlagStateDBFileUse = map[string]string{
		"english": "The statedb file you want to migrate.",
		"chinese": "您要迁移的 statedb 文件。",
	}
	stateDB2FactoryFlagFactoryFileUse = map[string]string{
		"english": "The path you want to migrate to",
		"chinese": "您要迁移到的路径。",
	}
)

var (
	// StateDB2Factory Used to Sub command.
	StateDB2Factory = &cobra.Command{
		Use:   common.TranslateInLang(stateDB2FactoryCmdUse),
		Short: common.TranslateInLang(stateDB2FactoryCmdShorts),
		Long:  common.TranslateInLang(stateDB2FactoryCmdLongs),
		RunE: func(cmd *cobra.Command, args []string) error {
			return statedb2Factory()
		},
	}
)

var (
	statedbFile = ""
	factoryFile = ""
)

func init() {
	StateDB2Factory.PersistentFlags().StringVarP(&statedbFile, "statedb", "s", "", common.TranslateInLang(stateDB2FactoryFlagStateDBFileUse))
	StateDB2Factory.PersistentFlags().StringVarP(&factoryFile, "factory", "f", "", common.TranslateInLang(stateDB2FactoryFlagFactoryFileUse))
}

func statedb2Factory() (err error) {
	// Check flags
	if statedbFile == "" {
		return fmt.Errorf("--statedb is empty")
	}
	if factoryFile == "" {
		return fmt.Errorf("--factory is empty")
	}
	if statedbFile == factoryFile {
		return fmt.Errorf("the values of --statedb --factory flags cannot be the same")
	}

	size := 200000

	statedb, err := bbolt.Open(statedbFile, 0666, nil)
	if err != nil {
		return err
	}
	defer statedb.Close()

	factorydb, err := db.CreateKVStore(db.DefaultConfig, factoryFile)
	if err != nil {
		return errors.Wrap(err, "failed to create db")
	}
	dbForTrie, err := trie.NewKVStore(factory.ArchiveTrieNamespace, factorydb)
	if err != nil {
		return errors.Wrap(err, "failed to create db for trie")
	}
	if err = dbForTrie.Start(context.Background()); err != nil {
		return errors.Wrap(err, "failed to start db for trie")
	}
	tlt := mptrie.NewTwoLayerTrie(dbForTrie, factory.ArchiveTrieRootKey)
	if err = tlt.Start(context.Background()); err != nil {
		return errors.Wrap(err, "failed to start tlt")
	}

	bat := batch.NewBatch()
	writeBatch := func(bat batch.KVStoreBatch) error {
		if err = factorydb.WriteBatch(bat); err != nil {
			return errors.Wrap(err, "failed to write batch")
		}
		for i := 0; i < bat.Size(); i++ {
			e, err := bat.Entry(i)
			if err != nil {
				return errors.Wrap(err, "failed to get entry")
			}
			nsHash := hash.Hash160b([]byte(e.Namespace()))
			keyLegacy := hash.Hash160b(e.Key())
			if err = tlt.Upsert(nsHash[:], keyLegacy[:], e.Value()); err != nil {
				return errors.Wrap(err, "failed to upsert tlt")
			}
		}
		return nil
	}
	if err := statedb.View(func(tx *bbolt.Tx) error {
		if err := tx.ForEach(func(name []byte, b *bbolt.Bucket) error {
			fmt.Printf("migrating namespace: %s %d\n", name, b.Stats().KeyN)
			bar := progressbar.NewOptions(b.Stats().KeyN, progressbar.OptionThrottle(time.Second))
			b.ForEach(func(k, v []byte) error {
				if v == nil {
					panic("unexpected nested bucket")
				}
				bat.Put(string(name), k, v, "failed to put")
				if uint32(bat.Size()) >= uint32(size) {
					if err := bar.Add(size); err != nil {
						return errors.Wrap(err, "failed to add progress bar")
					}
					if err = writeBatch(bat); err != nil {
						return err
					}
					bat = batch.NewBatch()
				}
				return nil
			})
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
	if bat.Size() > 0 {
		fmt.Printf("write the last batch %d\n", bat.Size())
		if err := writeBatch(bat); err != nil {
			return err
		}
	}
	fmt.Printf("stop tlt\n")
	if err = tlt.Stop(context.Background()); err != nil {
		return errors.Wrap(err, "failed to stop tlt")
	}
	fmt.Printf("stop db\n")
	if err = dbForTrie.Stop(context.Background()); err != nil {
		return errors.Wrap(err, "failed to stop db for trie")
	}
	return nil
}
