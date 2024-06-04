package logic

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/reef-runtime/reef/reef_manager/database"
)

const dataSetFilePermission = 0600
const dataSetFileEnding = ".bin"

type DatasetManagerT struct {
	DatasetPath string
}

var DatasetManager DatasetManagerT

//
// Saves and loads datasets from the database and interacts with the filesystem to store the
// contents of the dataset.
//

func (m *DatasetManagerT) AddDataset(name string, data []byte) (id string, err error) {
	idBinary := sha256.Sum256(append([]byte(name), data...))
	id = hex.EncodeToString(idBinary[0:])

	if err := database.AddDataset(database.Dataset{
		ID:   id,
		Name: name,
		Size: uint32(len(data)),
	}); err != nil {
		return "", err
	}

	path := filepath.Join(m.DatasetPath, fmt.Sprintf("%s%s", id, dataSetFileEnding))

	if err := os.WriteFile(path, data, dataSetFilePermission); err != nil {
		return "", err
	}

	return id, nil
}

func (m *DatasetManagerT) DeleteDataset(id string) (found bool, err error) {
	if found, err := database.DeleteDataset(id); err != nil || !found {
		return found, err
	}
	if err := os.Remove(m.DatasetPath + id + ".bin"); err != nil {
		return found, err
	}
	return true, nil
}

func (m *DatasetManagerT) LoadDataset(id string) (data []byte, found bool, err error) {
	if data, err = os.ReadFile(m.DatasetPath + id + ".bin"); err != nil {
		return []byte{}, false, err
	}
	return data, true, nil
}

func newDatasetManager(datasetPath string) DatasetManagerT {
	return DatasetManagerT{
		DatasetPath: datasetPath,
	}
}
