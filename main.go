package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/pierrec/lz4/v4"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

type Superblock struct {
	TotalBlocks int
	BlockSize   int
}

type superblockRaw struct {
	SInodesCount     uint32
	SBlocksCount     uint32
	SRBlocksCount    uint32
	SFreeBlocksCount uint32
	SFreeInodesCount uint32
	SFirstDataBlock  uint32
	SLogBlockSize    uint32
}

type Block struct {
	Offset   int64  `json:"Offset"`
	Checksum string `json:"BlockChecksum"`
}

type BackupConfig struct {
	CreatedTime       string  `json:"CreatedTime"`
	Size              string  `json:"Size"`
	CompressionMethod string  `json:"CompressionMethod"`
	Blocks            []Block `json:"Blocks"`
}

type Backup struct {
	Identifier  string
	Timestamp   time.Time
	Size        int64
	Compression string
	Blocks      []Block
}

type VolumeBackup struct {
	Name       string
	BackupPath string
	Backups    []Backup
}

func findVolumeBackupPath(backupStorePath string, volumeName string) (string, error) {
	pattern := filepath.Join(backupStorePath, "volumes", "**", "**", volumeName)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("could not find backup for %s", volumeName)
	}
	return matches[0], nil
}
func readSuperblock(f *os.File) (Superblock, error) {
	const superblockOffset = 1024

	_, err := f.Seek(superblockOffset, 0)
	if err != nil {
		return Superblock{}, err
	}

	var raw superblockRaw
	err = binary.Read(f, binary.LittleEndian, &raw)
	if err != nil {
		return Superblock{}, err
	}

	return Superblock{
		TotalBlocks: int(raw.SBlocksCount),
		BlockSize:   int(1024 << raw.SLogBlockSize),
	}, nil
}

func decompressLZ4(data []byte) ([]byte, error) {
	r := lz4.NewReader(bytes.NewReader(data))
	return io.ReadAll(r)
}

func readBackups(path string) (*VolumeBackup, error) {
	backupCfgPattern := filepath.Join(path, "backups", "*.cfg")
	backupCfgPaths, err := filepath.Glob(backupCfgPattern)
	if err != nil {
		return nil, err
	}

	volumeBackup := &VolumeBackup{
		Name:       filepath.Base(path),
		BackupPath: path,
		Backups:    make([]Backup, 0),
	}

	for _, cfgPath := range backupCfgPaths {
		cfgFile, err := os.Open(cfgPath)
		defer cfgFile.Close()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(cfgFile)

		var cfg BackupConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}

		timestamp, err := time.Parse(time.RFC3339, cfg.CreatedTime)
		if err != nil {
			return nil, err
		}

		size, err := strconv.Atoi(cfg.Size)
		if err != nil {
			return nil, err
		}

		backup := Backup{
			Identifier:  cfgPath,
			Timestamp:   timestamp,
			Size:        int64(size),
			Compression: cfg.CompressionMethod,
			Blocks:      cfg.Blocks,
		}

		volumeBackup.Backups = append(volumeBackup.Backups, backup)
	}

	sort.Slice(volumeBackup.Backups, func(i, j int) bool {
		return volumeBackup.Backups[i].Timestamp.Before(volumeBackup.Backups[j].Timestamp)
	})

	return volumeBackup, nil
}

func resolveBlockPath(backupPath, checksum string) (string, error) {
	pattern := filepath.Join(backupPath, "blocks", "**", "**", checksum+".blk")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("could not find block %s", checksum)
	}
	return matches[0], nil
}

func writeBlockToBuffer(blockData []byte, offset int64, fileDiscriptor *os.File) {
	fileDiscriptor.Seek(offset, io.SeekStart)
	fileDiscriptor.Write(blockData)
}

func main() {
	backupRoot := flag.String("backup-root", "", "Backup root directory")
	target := flag.String("target", "", "Backup target")
	outfile := flag.String("outfile", "", "Output file")
	flag.Parse()

	if *backupRoot == "" || *target == "" || *outfile == "" {
		flag.Usage()
		os.Exit(1)
	}

	backupStorePath := filepath.Join(*backupRoot, "backupstore")
	if _, err := os.Stat(backupStorePath); os.IsNotExist(err) {
		fmt.Printf("Backup root %s does not contain backupstore\n", *backupRoot)
		os.Exit(1)
	}

	if _, err := os.Stat(filepath.Dir(*outfile)); os.IsNotExist(err) {
		fmt.Printf("Output directory for %s does not exist\n", *outfile)
		os.Exit(1)
	}

	if _, err := os.Stat(*outfile); err == nil {
		fmt.Printf("Output file %s already exists\n", *outfile)
		os.Exit(1)
	}
	outfile_descriptor, err := os.Create(*outfile)
	defer outfile_descriptor.Close()
	if err != nil {
		fmt.Printf("Failed to create output file %s\n", *outfile)
		os.Exit(1)
	}

	fmt.Printf("Looking for backups in %s\n", backupStorePath)
	volumeBackups, err := findVolumeBackupPath(backupStorePath, *target)
	if err != nil {
		fmt.Printf("Failed to find backups for %s\n", *target)
		os.Exit(1)
	}
	fmt.Printf("Found backups for %s at %s\n", *target, volumeBackups)
	volumeBackup, err := readBackups(volumeBackups)
	if err != nil {
		fmt.Printf("Failed to read backups for %s\n", *target)
		os.Exit(1)
	}
	for _, backup := range volumeBackup.Backups {
		fmt.Printf("Backup %s\n", backup.Identifier)
		totalBlocks := len(backup.Blocks)

		for i, block := range backup.Blocks {
			percentage := float64(i+1) / float64(totalBlocks) * 100
			fmt.Printf("[%.2f%%] Processing Block %s {offset=%d}\n", percentage, block.Checksum, block.Offset)

			blockPath, err := resolveBlockPath(volumeBackup.BackupPath, block.Checksum)
			if err != nil {
				fmt.Printf("Failed to resolve block %s\n", block.Checksum)
				os.Exit(1)
			}

			blockData, err := os.ReadFile(blockPath)
			if err != nil {
				fmt.Printf("Failed to read block %s\n", block.Checksum)
				os.Exit(1)
			}

			blockData, err = decompressLZ4(blockData)
			if err != nil {
				fmt.Printf("Failed to decompress block %s\n", block.Checksum)
				os.Exit(1)
			}

			writeBlockToBuffer(blockData, block.Offset, outfile_descriptor)
		}
		fmt.Printf("\n")
	}
	superblock, err := readSuperblock(outfile_descriptor)
	if err != nil {
		fmt.Printf("Failed to read superblock. This tool only works with ext4 filesystems. The raw filesystem has been created, but you may need to resize the filesystem or extend the physical data with zeroes.\n")
		os.Exit(1)
	}
	fmt.Printf("Superblock: %d blocks of size %d\n", superblock.TotalBlocks, superblock.BlockSize)
	fmt.Printf("Total size of backup: %d\n", superblock.TotalBlocks*superblock.BlockSize)
	fmt.Println("Truncating block file")
	outfile_descriptor.Truncate(int64(superblock.TotalBlocks * superblock.BlockSize))
	fmt.Println("Done")
}
