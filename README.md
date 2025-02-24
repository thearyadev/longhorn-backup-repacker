# Longhorn Backup Repacker

A utility tool to repack Longhorn backup segments into a single raw disk image file using a mounted filesystem.

## Overview

Longhorn stores backups as incremental block-segments to optimize storage through deduplication. While `longhorn-engine` can restore backups using NFS and S3 filesystem drivers, this tool provides an alternative method using locally mounted filesystems.

## Key Features

- Converts Longhorn backup segments into a single raw disk image
- Works with locally mounted filesystems
- Supports `lz4` compression format

## Installation

Choose one of these methods:

1. **Build from source:**
   ```bash
   git clone https://github.com/thearyadev/longhorn-backup-repacker
   cd longhorn-backup-repacker
   # Build instructions here
   ```

2. **Download pre-built binary:**
   Visit the [releases page](https://github.com/thearyadev/longhorn-backup-repacker/releases)

## Usage

```bash
./longhorn-backup-repacker [flags]

Flags:
  -backup-root string   Path to Longhorn backup root directory
  -outfile string       Path for the output raw disk image
  -target string       Name of the volume to restore
```

### Example Command

```bash
./longhorn-backup-repacker \
  -backup-root "/path/to/longhorn/backup/root" \
  -outfile ./outfile.raw \
  -target volume_name
```

## Limitations

1. **Filesystem Support:**
   - Primary support for `ext4` filesystem
   - Other filesystems may result in apparently corrupted devices (fixable by zero-filling or filesystem shrinking)

2. **Compression Support:**
   - Currently supports `lz4` decompression only
   - `gzip` support is in development

3. **Transport Protocols:**
   - Does not support NFS or S3
   - Works only with locally mounted filesystems

## Important Notice

This tool is currently in development and intended for testing purposes only. It was developed through reverse engineering of Longhorn v1.7 backups and may not be compatible with other versions. Use in production environments is not recommended.
