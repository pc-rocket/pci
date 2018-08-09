package generate

import (
	"encoding/binary"
	"log"
	"math/rand"
	"os"
	"path"
	"sync"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/pc-rocket/pci/common"
)

var (
	maxIndex   = common.FileSize / common.RecordSize
	blockCount = int(common.FileSize / common.BlockSize)
)

type record struct {
	userID uint64
	itemID uint64
	price  uint64
}

// Files generates the required data files. Note that it expects
// that if user_items.bin exists, then item_prices.bin must also
// exist and be valid for user_items.bin. If one file is missing,
// the user should remove the other as well and regenerate both.
func Files() {
	// create the user:item file
	userItems, err := os.Create(path.Join(common.DataDir, common.UserItemsFile))
	if err != nil {
		if err == os.ErrExist {
			log.Printf("%s exists - skipping generation\n", common.UserItemsFile)
			return
		}
		log.Panicf("failed to create %s (%v)", common.UserItemsFile, err)
	}

	if err = userItems.Truncate(common.FileSize); err != nil {
		log.Panicf("failed to allocate space for %s (%v)", common.UserItemsFile, err)
	}

	// create the item:price file
	itemPrices, err := os.Create(path.Join(common.DataDir, common.ItemPricesFile))
	if err != nil {
		if err == os.ErrExist {
			log.Printf("%s exists - skipping generation\n", common.ItemPricesFile)
		} else {
			log.Panicf("failed to create %s (%v)", common.ItemPricesFile, err)
		}
	}

	if err = itemPrices.Truncate(common.FileSize); err != nil {
		log.Panicf("failed to allocate space for %s (%v)", common.ItemPricesFile, err)
	}

	log.Println("generating data files...")

	start := time.Now()

	// preallocate the needed memory and recycle it
	var (
		userItemBytes  = make([]byte, common.BlockSize)
		itemPriceBytes = make([]byte, common.BlockSize)
		wg             sync.WaitGroup
		rec            record
		pct            = 0.0
	)

	log.Printf("writing %v records\n", humanize.Comma(maxIndex))

	for i := 0; i < blockCount; i++ {
		// fill the write block
		for j := 0; j < int(common.BlockSize); j += common.RecordSize {
			rec.userID = uint64(rand.Int63n(common.NumUsers))
			rec.itemID = uint64(rand.Int63n(common.NumItems))
			rec.price = uint64(rand.Int63n(1000))

			binary.LittleEndian.PutUint64(userItemBytes[j:], rec.userID)
			binary.LittleEndian.PutUint64(userItemBytes[j+common.RecordSize/2:], rec.itemID)

			binary.LittleEndian.PutUint64(itemPriceBytes[j:], rec.itemID)
			binary.LittleEndian.PutUint64(itemPriceBytes[j+common.RecordSize/2:], rec.price)
		}

		wg.Add(1)

		go func() {
			if _, err = userItems.Write(userItemBytes); err != nil {
				log.Panic("write failure", "file", common.UserItemsFile, "error", err)
			}

			wg.Done()
		}()

		wg.Add(1)

		go func() {
			if _, err = itemPrices.Write(itemPriceBytes); err != nil {
				log.Panic("write failure", "file", common.ItemPricesFile, "error", err)
			}

			wg.Done()
		}()

		wg.Wait()

		if (float64(i*int(common.BlockSize))/float64(maxIndex*common.RecordSize)*100 - pct) > 1 {
			pct = float64(i*int(common.BlockSize)) / float64(maxIndex*common.RecordSize) * 100
			log.Printf("%v%% complete (%.2f MB/sec)\n", int(pct), float64(i*int(common.BlockSize)/(1024*1024))/time.Now().Sub(start).Seconds())
		}

	}

	// sync the files to ensure everything is written to disk
	if err = userItems.Sync(); err != nil {
		log.Panicf("failed to sync user_items.bin to disk (%v)", err)
	}

	if err = itemPrices.Sync(); err != nil {
		log.Panicf("failed to sync item_prices.bin to disk(%v)", err)
	}

	elapsed := time.Now().Sub(start)

	log.Printf(
		"done in %v (%v records/sec)\n",
		elapsed,
		humanize.Comma(int64(2*float64(common.FileSize/common.RecordSize)/elapsed.Seconds())))
}
