# Longhorn Backup Repacker

Longhorn backups are stored as incremental block-segments of the original data. This is done to deuplicate and provide a lightweight interface for backups.

`longhorn-engine` provides a restore the backup to a raw (or cow) file, but only supports this via supported filesystem drivers, NFS and S3. This utility allows you to use a mounted filesystem. 

## Usage

```
Usage of ./longhorn-backup-repacker:
  -backup-root string
        Backup root directory
  -outfile string
        Output file
  -target string
        Backup target
```

## Example
```bash 
./longhorn-backup-repacker -backup-root "/path/to/longhorn/backup/root" -outfile ./outfile.raw -target <volume_name>
```

## Limitations

* Only supports `ext4` and `xfs` filesystems
