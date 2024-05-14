package logic

import (
	"crypto/sha256"
	"encoding/hex"
	"os"

	"github.com/reef-runtime/reef/reef_manager/database"
)

type DatasetManagerT struct {
	DatasetPath string
}

var DatasetManager DatasetManagerT

//
// Saves and loads datasets from the database and interacts with the filesystem to store the
// contents of the dataset.
//

func AddDataset(name string, data []byte) (id string, err error) {
	idBinary := sha256.Sum256(append([]byte(name), data...))
	id = hex.EncodeToString(idBinary[0:])
	dataset := database.Dataset{
		ID:   id,
		Name: name,
		Size: uint32(len(data)),
	}
	if err := database.AddDataset(dataset); err != nil {
		return "", err
	}
	if err := os.WriteFile(DatasetManager.DatasetPath+id+".bin", data, 0755); err != nil {
		return "", err
	}
	return id, nil
}

func DeleteDataset(id string) (found bool, err error) {
	if found, err := database.DeleteDataset(id); err != nil || !found {
		return found, err
	}
	if err := os.Remove(DatasetManager.DatasetPath + id + ".bin"); err != nil {
		return found, err
	}
	return true, nil
}

func LoadDataset(id string) (data []byte, found bool, err error) {
	if data, err = os.ReadFile(DatasetManager.DatasetPath + id + ".bin"); err != nil {
		return []byte{}, false, err
	}
	return data, true, nil
}

func (m *DatasetManagerT) init(path string) error {
	m.DatasetPath = path
	return nil
}

func newDatasetManager() DatasetManagerT {
	return DatasetManagerT{
		DatasetPath: "",
	}
}
