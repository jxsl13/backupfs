# backupfs

WORK IN PROGRESS

backupfs is an implmentation of the spf13/afero interface in order to backup files that are to be overwritten in one filesystem to another filesystem.

The first file modifying access will backup the whole file.
Consecutive file changes are ignored.