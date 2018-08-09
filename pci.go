package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"

	"github.com/pc-rocket/pci/common"
	"github.com/pc-rocket/pci/generate"
	"github.com/pc-rocket/pci/process"
	"github.com/spf13/cobra"
)

func main() {
	f, err := os.Create("cpu.pprof")
	if err != nil {
		log.Fatal(err)
	}
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	// command to generate data files
	cmdGenerate := &cobra.Command{
		Use:   "generate",
		Short: "generate data files",
		Long:  "generate user_items.bin and item_prices.bin for processing",
		Run: func(cmd *cobra.Command, args []string) {
			generate.Files()
		},
	}

	// command to process data files
	cmdProcess := &cobra.Command{
		Use:   "process",
		Short: "process generated data files",
		Long:  "process user_items.bin and item_prices.bin and create user_expenses.bin",
		Run: func(cmd *cobra.Command, args []string) {
			process.Expenses()
		},
	}

	// command to clean the data directory
	cmdClean := &cobra.Command{
		Use:   "clean",
		Short: "clean the genereated data files",
		Long:  "clean user_items.bin, item_prices.bin, user_expenses.csv and any leveldb directories from DATA_DIR",
		Run: func(cmd *cobra.Command, args []string) {
			// if err := os.RemoveAll(common.DataDir); err != nil {
			// 	log.Panicf("failed to remove %s (%v)", common.DataDir, err)
			// }

			files, err := filepath.Glob(common.DataDir + "/*")
			if err != nil {
				panic(err)
			}
			for _, f := range files {
				if err := os.RemoveAll(f); err != nil {
					panic(err)
				}
			}

			if err := os.MkdirAll(common.DataDir, os.ModePerm); err != nil {
				log.Panicf("failed to re-create %s (%v)", common.DataDir, err)
			}
		},
	}

	rootCmd := &cobra.Command{Use: "app"}
	rootCmd.AddCommand(cmdGenerate, cmdProcess, cmdClean)
	rootCmd.Execute()
}
