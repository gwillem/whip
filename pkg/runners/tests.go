package runners

import "github.com/spf13/afero"

func createTestFS() {
	fs = afero.NewMemMapFs()
	fsutil = &afero.Afero{Fs: fs}
}
