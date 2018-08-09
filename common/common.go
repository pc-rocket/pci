package common

import (
	"strconv"

	"github.com/pc-rocket/pci/env"
)

const (
	UserItemsFile    = "user_items.bin"
	ItemPricesFile   = "item_prices.bin"
	UserExpensesFile = "user_expenses.csv"
	RecordSize       = 16 // bytes
)

var (
	DataDir   string
	FileSize  int64
	BlockSize int64
	MaxCPUs   int64
	NumItems  int64
	NumUsers  int64
)

func init() {
	env.RegisterDefault("DATA_DIR", "/project/pci/data/")
	env.RegisterDefault("FILE_SIZE", strconv.FormatInt(1024*1024*1024, 10)) // 1 GB default
	env.RegisterDefault("BLOCK_SIZE", strconv.FormatInt(1024*1024*10, 10))  // 10 MB default
	env.RegisterDefault("MAX_CPUS", strconv.FormatInt(8, 10))               // 8 cores default
	env.RegisterDefault("UNIQUE_USERS", strconv.FormatInt(1000000, 10))     // 1M unique users default
	env.RegisterDefault("UNIQUE_ITEMS", strconv.FormatInt(1000000, 10))     // 1M unique items default

	setEnv()
}

func setEnv() {
	DataDir = env.GetVar("DATA_DIR")
	FileSize, _ = strconv.ParseInt(env.GetVar("FILE_SIZE"), 10, 64)
	BlockSize, _ = strconv.ParseInt(env.GetVar("BLOCK_SIZE"), 10, 64)
	// block size can't be larger than file size
	if BlockSize > FileSize {
		BlockSize = FileSize
	}
	MaxCPUs, _ = strconv.ParseInt(env.GetVar("MAX_CPUS"), 10, 64)
	NumItems, _ = strconv.ParseInt(env.GetVar("UNIQUE_ITEMS"), 10, 64)
	NumUsers, _ = strconv.ParseInt(env.GetVar("UNIQUE_USERS"), 10, 64)
}
