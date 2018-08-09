***Background***

**Goal**

Calculate how much money each user has spent efficiently

**Setup**

In a machine with 8 CPU cores, 1GB memory, and 4TB HDD disk, self-generate two files with data of the following type:

File 1: a 1TB file with (item_id: uint64, item_price: uint64), which contains a lot of records about the price of each item.
File 2: a 1TB file with (user_id: uint64, item_id: uint64), which contains a lot of records about the items each user purchased.

- The items in the files should be unordered.
- Each item has a unique item_id.
- Each user has a unique user_id.
- Each user can purchase one or more different or the same item.

**Technical requirement**

Please write a program to calculate how much money each user has spent in the most efficient way possible and output the result to the disk.

***Design***

**Initial Thoughts**

The first thing that jumped out at me about this problem was the machine definition. In short, the system is light on memory, has massive disk capacity, and decent CPU concurrency. The operation to be done, defined in database terms, is in essence a JOIN() operation, followed by a SUM().

With the machine constraints in mind, the best way to execute this type of operation would be to do a parallel hybrid hash join. A hash join makes sense because of the lack of memory, and large amount of disk, and I choose hybrid to utilize the memory that is available. This should be done in parallel to leverage the CPU cores as well. Also, since the disks are HDD rather than SSD, sequential reads and writes will need to be done instead of random since HDD's are not fast for random reads/writes.

**Implementation**

The CLI program I wrote is written in Go, and I have used dep for dependency management. Also, for the sake of brevity, rather than defining my own hashing and storage implementation for the hash join, I chose to leverage LMDB. I have also used cobra CLI for a clean CLI experience.

The storage format is binary, to optimize for sequential reads and writes. The raw uint64 pair bytes are written in 16 byte records in each file, and read sequentially into memory in blocks, and written to the KV store. Once the files have been hashed, they are then joined, summed and written to a CSV file for legibility.

**Usage**

The CLI program has two commands, and is configured using environment variables. All configuration parameters are specified in `common.go`. To generate the binary files run:

```
pci generate
```

This will generate the files (`user_items.bin`, `item_prices.bin`) according to the following environment variables:

```
FILE_SIZE       the file size of each generated file  (default 1 GB)
BLOCK_SIZE      the block size by which the filesystem is read  (default 10 MB)
DATA_DIR        where to put the files (default /project/pci/data)
UNIQUE_USERS    number of unique users (default 1M)
UNIQUE_ITEMS    number of unique items (default 1M)
MAX_PRICE       maximum price of an item (default 1K)
```

Once the files are generated, they can be processed by running the following command:

```
pci process
```

This will hash join the files, sum the prices into a CSV file called `user_expenses.csv`, and place it in the directory specified by `DATA_DIR`.

If you would like to re-run the processing, the data directory must be cleaned, and the data files regenerated. This clean operation can be done by running:

```
pci clean
```

**Docker**

I have included a Dockerfile in this repo as it made the most sense for me to test the performance, and also will make it easy for any evaluators to run the cli as well. The typical command flow is as follows:

```
$ docker build pci -t .

$ docker run -it --rm -v /path/to/large/hdd:/project/pci/data -m 1g --cpus="8" pci pci clean

$ docker run -it --rm -v /path/to/large/hdd:/project/pci/data -m 1g --cpus="8" pci pci generate
2018/08/09 07:10:18 generating data files...
2018/08/09 07:10:18 writing 67,108,864 records
2018/08/09 07:10:18 1% complete (105.74 MB/sec)
2018/08/09 07:10:18 3% complete (129.05 MB/sec)
...
...
...
2018/08/09 07:10:28 95% complete (100.66 MB/sec)
2018/08/09 07:10:28 97% complete (101.39 MB/sec)
2018/08/09 07:10:35 done in 17.189685258s (7,808,038 records/sec)

$ docker run -it --rm -v /path/to/large/hdd:/project/pci/data -m 1g --cpus="8" pci pci process
2018/08/09 07:10:49 calculating user expenses...
2018/08/09 07:10:49 1% complete (1,310,720 records processed)
2018/08/09 07:10:49 3% complete (2,621,440 records processed)
...
...
...
2018/08/09 07:11:41 96% complete (64,225,280 records processed)
2018/08/09 07:11:42 98% complete (65,536,000 records processed)
2018/08/09 07:11:50 done in 1m1.326214837s
```

Note that above the docker options include limiting the memory to 1 GB and the CPU cores to 8.

***Performance***

As shown in the previous section, the default configured setup generates two 1GB files in ~17 seconds, and processes these files to produce a join-summed csv file in just over 1 minute. This equates to a record/sec join and sum rate of just over 1 million records per second. The design of the code implies that this performance should be linear with the size of the files, and this also holds true for 10 GB sized files:

```
$ docker run -it --rm -v /path/to/large/hdd:/project/pci/data -m 1g --cpus="8" -e FILE_SIZE=10737418240 pci pci generate
2018/08/09 07:24:34 generating data files...
2018/08/09 07:24:34 writing 671,088,640 records
2018/08/09 07:24:35 1% complete (147.81 MB/sec)
2018/08/09 07:24:36 2% complete (140.87 MB/sec)
...
...
...
2018/08/09 07:26:47 98% complete (76.54 MB/sec)
2018/08/09 07:26:48 99% complete (76.67 MB/sec)
2018/08/09 07:26:55 done in 2m20.438118016s (9,557,072 records/sec)

$ docker run -it --rm -v /path/to/large/hdd:/project/pci/data -m 1g --cpus="8" -e FILE_SIZE=10737418240 pci pci process
2018/08/09 07:28:00 calculating user expenses...
2018/08/09 07:28:01 1% complete (7,208,960 records processed)
2018/08/09 07:28:08 2% complete (14,417,920 records processed)
...
...
...
2018/08/09 07:37:45 98% complete (663,224,320 records processed)
2018/08/09 07:37:51 99% complete (670,433,280 records processed)
2018/08/09 07:37:58 done in 9m58.705823882s
```

As we can see, the performance does appear to be linear according to file size, with a 10 GB file test taking ~10x the 1 GB test. Based on these findings, I expect the 1 TB example, as posed in the initial requirements to join the approximately 70 billion generated records in ~16.5 hours.

Note that both this test and the 1 GB test were carried out setting UNIQUE_USERS and UNIQUE_ITEMS to 1M each. Increasing these will also impact performance. Also note that all of these test were run on Ubuntu 16.04 with two 6TB HDD's in RAID0, and a Ryzen 1800x CPU.
