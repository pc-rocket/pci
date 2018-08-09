package process

import (
	"encoding/binary"
	"encoding/csv"
	"log"
	"os"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/bmatsuo/lmdb-go/lmdb"
	humanize "github.com/dustin/go-humanize"
	"github.com/pc-rocket/pci/common"
	"github.com/pc-rocket/pci/pool"
)

var (
	itemKV, userKV   *lmdb.Env
	itemDBI, userDBI lmdb.DBI
	wg               sync.WaitGroup
	maxIndex         = common.FileSize / common.RecordSize
	blockCount       = int(common.FileSize / common.BlockSize)
)

func initDB() {
	var err error

	itemKV, err = lmdb.NewEnv()
	if err != nil {
		log.Panicf("failed to open lmdb (%v)", err)
	}

	itemKV.SetMaxDBs(1)
	itemKV.SetFlags(lmdb.NoSync)
	itemKV.SetMapSize(common.FileSize)
	itemKV.SetFlags(lmdb.NoOverwrite)

	os.MkdirAll(path.Join(common.DataDir, "item-prices"), 0777)

	err = itemKV.Open(path.Join(common.DataDir, "item-prices"), 0, 0644)
	if err != nil {
		log.Panicf("failed to open lmdb (%v)", err)
	}

	err = itemKV.Update(func(txn *lmdb.Txn) (err error) {
		itemDBI, err = txn.CreateDBI("user-items")
		return err
	})

	userKV, err = lmdb.NewEnv()
	if err != nil {
		log.Panicf("failed to open lmdb (%v)", err)
	}

	userKV.SetMaxDBs(1)
	userKV.SetMapSize(common.FileSize)
	userKV.SetFlags(lmdb.NoMetaSync)
	userKV.SetFlags(lmdb.NoSync)
	userKV.SetFlags(lmdb.FixedMap)
	userKV.SetFlags(lmdb.CopyCompact)
	userKV.SetFlags(lmdb.AppendDup)
	userKV.SetFlags(lmdb.DupFixed)

	os.MkdirAll(path.Join(common.DataDir, "user-items"), 0777)

	err = userKV.Open(path.Join(common.DataDir, "user-items"), 0, 0644)
	if err != nil {
		log.Panicf("failed to open lmdb (%v)", err)
	}

	err = userKV.Update(func(txn *lmdb.Txn) (err error) {
		userDBI, err = txn.CreateDBI("user-items")
		return err
	})
}

func Expenses() {
	initDB()

	log.Println("calculating user expenses...")

	start := time.Now()

	// read the item prices to KV store
	{
		wg.Add(1)
		go readPrices()

		wg.Add(1)
		go readUsers()

		wg.Wait()
	}

	// calculate
	calculateExpenses()

	itemKV.Close()
	userKV.Close()

	// cleanup
	os.Remove(path.Join(common.DataDir, "item-prices"))
	os.Remove(path.Join(common.DataDir, "user-items"))

	elapsed := time.Now().Sub(start)

	log.Printf("done in %v\n", elapsed)
}

type record struct {
	key   []byte
	value []byte
}

func readUsers() {
	defer wg.Done()

	userItems, err := os.Open(path.Join(common.DataDir, common.UserItemsFile))
	if err != nil {
		log.Panicf("failed to open %s (%v)", common.UserItemsFile, err)
	}

	buf := make([]byte, common.BlockSize)

	pct := 0.0

	p := pool.NewPool(int(common.MaxCPUs), storeUserBlock)

	c := make(chan interface{})

	go p.Work(c)

	for i := 0; i < blockCount; i++ {
		if _, err = userItems.Read(buf); err != nil {
			log.Panicf("failed to read %s (%v)", common.UserItemsFile, err)
		}

		c <- buf

		if float64(i)/float64(blockCount)*100-pct > 1 {
			pct = float64(i) / float64(blockCount) * 100

			log.Printf(
				"%v%% complete (%v records processed)\n",
				int(pct),
				humanize.Comma(int64(i*int(common.BlockSize)/common.RecordSize)))
		}
	}

	close(c)

	p.Wait()

	userItems.Close()

	os.Remove(path.Join(common.DataDir, common.UserItemsFile))
}

func readPrices() {
	defer wg.Done()

	itemPrices, err := os.Open(path.Join(common.DataDir, common.ItemPricesFile))
	if err != nil {
		log.Panicf("failed to open %s (%v)", common.ItemPricesFile, err)
	}

	buf := make([]byte, common.BlockSize)

	p := pool.NewPool(int(common.MaxCPUs), storeItemBlock)

	c := make(chan interface{})

	go p.Work(c)

	for i := 0; i < blockCount; i++ {
		if _, err = itemPrices.Read(buf); err != nil {
			log.Panicf("failed to read %s (%v)", common.ItemPricesFile, err)
		}

		c <- buf
	}

	close(c)

	p.Wait()

	itemPrices.Close()

	os.Remove(path.Join(common.DataDir, common.ItemPricesFile))
}

func storeItemBlock(i interface{}) {
	block := i.([]byte)

	itemKV.Update(func(txn *lmdb.Txn) (err error) {
		for j := 0; j < int(common.BlockSize); j += common.RecordSize {
			if err = txn.Put(
				itemDBI,
				block[j:j+common.RecordSize/2],
				block[j+common.RecordSize/2:j+common.RecordSize],
				0); err != nil {
				log.Panicf("failed to update badger (%v)", err)
			}
		}
		return
	})
}

func storeUserBlock(i interface{}) {
	block := i.([]byte)

	userKV.Update(func(tx *lmdb.Txn) (err error) {
		for j := 0; j < int(common.BlockSize); j += common.RecordSize {
			key := block[j : j+common.RecordSize/2]                     // key
			value := block[j+common.RecordSize/2 : j+common.RecordSize] // value

			// appends if exists
			if err = tx.Put(userDBI, key, value, 0); err != nil {
				log.Panicf("kv storage failure (%v)", err)
			}
		}
		return
	})
}

var (
	userExpenses *os.File
	w            *csv.Writer
	csvLock      sync.Mutex // csv writer not thread safe
)

func initCSV() {
	var err error

	userExpenses, err = os.Create(path.Join(common.DataDir, common.UserExpensesFile))
	if err != nil {
		log.Panicf("failed to create %s (%v)", common.UserExpensesFile, err)
	}

	w = csv.NewWriter(userExpenses)
}

func calculateExpenses() {
	initCSV()

	p := pool.NewPool(int(common.MaxCPUs), expense)

	c := make(chan interface{}, common.MaxCPUs)

	go p.Work(c)

	err := userKV.View(func(txn *lmdb.Txn) error {
		cur, err := txn.OpenCursor(userDBI)
		if err != nil {
			return err
		}

		defer cur.Close()

		for {
			k, v, err := cur.Get(nil, nil, lmdb.Next)
			if lmdb.IsNotFound(err) {
				return nil
			}
			if err != nil {
				return err
			}

			c <- kv{k: k, v: v}
		}

		return nil
	})

	if err != nil {
		log.Panicf("kv iterator failure (%v)", err)
	}
}

type kv struct {
	k []byte
	v []byte
}

func expense(i interface{}) {
	record := i.(kv)
	key := record.k
	value := record.v

	expenses := uint64(0)

	err := itemKV.View(func(txn *lmdb.Txn) error {
		// tally up expenses
		for j := 0; j < len(value); j += common.RecordSize / 2 {
			itemID := value[j : common.RecordSize/2]

			if allZero(itemID) {
				break
			}

			price, err := txn.Get(itemDBI, itemID)
			if err != nil {
				return err
			}

			expenses += binary.LittleEndian.Uint64(price)
		}

		return nil
	})

	if err != nil {
		log.Panicf("failed to expense (%v)", err)
	}

	// lock csv for writing (it's not thread-safe)
	csvLock.Lock()
	if err := w.Write([]string{
		strconv.FormatUint(binary.LittleEndian.Uint64(key), 10),
		strconv.FormatUint(expenses, 10),
	}); err != nil {
		log.Panicf("failed to write to %s (%v)", common.UserExpensesFile, err)
	}

	csvLock.Unlock()
}

func allZero(s []byte) bool {
	for _, v := range s {
		if v != 0 {
			return false
		}
	}
	return true
}
