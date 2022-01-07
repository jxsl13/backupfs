package backupfs

import "testing"

func Test_removeStrings(t *testing.T) {
	type args struct {
		lookupIn string
		applyTo  string
		remove   []string
	}
	tests := []struct {
		name        string
		args        args
		wantCleaned string
		wantFound   bool
	}{
		{"#1", args{"..prefixfs_test0txt/../..", "..prefixfs_test0txt/../..", []string{"../", "./", ".."}}, "prefixfs_test0txt/", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCleaned, gotFound := removeStrings(tt.args.lookupIn, tt.args.applyTo, tt.args.remove...)
			if gotCleaned != tt.wantCleaned {
				t.Errorf("removeStrings() gotCleaned = '%v', want '%v'", gotCleaned, tt.wantCleaned)
			}
			if gotFound != tt.wantFound {
				t.Errorf("removeStrings() gotFound = %v, want %v", gotFound, tt.wantFound)
			}
		})
	}
}
