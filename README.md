# backupfs & prefixfs

(both filesystem abstractions implement the spf13/afero interfaces)

This package provides two filesystem abstractions.
A structure called `PrefixFs`. As the name already suggests, PrefixFS forces an file system to have a specific prefix.
Any attempt to escape the prefix path by directory traversal should (TM) be prevented, forcing the application to stay within the designated prefix directory.


The second and more important part of this library is `BackupFs`.
It is a filesystem anbstraction that consists of two parts.
A base filesystem and a backup filesystem.
Any attempt to modify a file in the base filesystem leads to the file being backed up to the backup filesystem.

Consecutive file modifications are ignored as the file has already been backed up.

## Fuzzing (beta)

[https://go.dev/blog/fuzz-beta](https://go.dev/blog/fuzz-beta)

Example
```
// TODO
```

Small roadmap:

- testing of the backup fs with memory mapped fs
- some fuzzing would be interesting