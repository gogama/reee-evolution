package reeeuse

import (
	"path"
)

const name = "reee"

func File(dir, suffix string) string {
	return path.Join(dir, name+suffix)
}

func Dir(dirFunc func() (string, error), create bool) (string, error) {
	dir, err := dirFunc()
	if err != nil {
		return "", err
	}
	return path.Join(dir, name), nil
}
