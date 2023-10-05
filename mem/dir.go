package mem

import "sort"

type Dir interface {
	Len() int
	Names() []string
	Files() []*FileData
	Add(*FileData)
	Remove(*FileData)
}

func initDirMap(d *FileData) {
	if d.memDir == nil {
		d.dir = true
		d.memDir = &DirMap{}
	}
}

type DirMap map[string]*FileData

func (m DirMap) Len() int           { return len(m) }
func (m DirMap) Add(f *FileData)    { m[f.name] = f }
func (m DirMap) Remove(f *FileData) { delete(m, f.name) }
func (m DirMap) Files() (files []*FileData) {
	for _, f := range m {
		files = append(files, f)
	}
	sort.Sort(filesSorter(files))
	return files
}

// implement sort.Interface for []*FileData
type filesSorter []*FileData

func (s filesSorter) Len() int           { return len(s) }
func (s filesSorter) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s filesSorter) Less(i, j int) bool { return s[i].name < s[j].name }

func (m DirMap) Names() (names []string) {
	for x := range m {
		names = append(names, x)
	}
	return names
}
