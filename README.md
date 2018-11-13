# sabo - bandwidth limiting pipe with collaborative capability

sabo is bandwidth limiting pipe like throttle command. And has collaborative capability like trickle

throttle - bandwidth limiting pipe https://linux.die.net/man/1/throttle
trickle - a lightweight userspace bandwidth shaper https://www.systutorials.com/docs/linux/man/1-trickle/

## Usage

```
% sabo -h
Usage:
  sabo [OPTIONS]

Application Options:
      --max-bandwidth= max bandwidth (Bytes/sec)
      --work-dir=      directory for control bandwidth
  -v, --version        Show version

Help Options:
  -h, --help           Show this help message
```

sabo creates a lock file in work dir. sabo checks number of files in work dirr in 1 sec.
Each process's bandwidth limitation become `max-bandwidth / number of file`.

## example

```
MAX_BW=100M
WORK_DIR=/tmp/sabo_for_dump
mkdir -p /tmp/sabo_for_dump
mysqldump -h backup1 db table1  | sabo --max-bandwidth $MAX_BW --work-dir $WORK_DIR > /tmp/sabo_for_dump/table1.sql &
mysqldump -h backup2 db table2  | sabo --max-bandwidth $MAX_BW --work-dir $WORK_DIR > /tmp/sabo_for_dump/table2.sql &
wait
```

Each mysqldump's bandwidth limitation is 50MB/sec.
When either of mysqldump finishes, the bandwidth of the remaining process becomes 100 MB/s.
