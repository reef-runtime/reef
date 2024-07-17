package logic

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

type TemplateDataset struct {
	// Path relative from the manifest, to the dataset file.
	Path string `json:"path"`
	Name string `json:"name"`
}

type TemplateManifest struct {
	Name string `json:"name"`
	// Path relative from the manifest, to the code file.
	CodePath string `json:"codePath"`
	// Uses the empty dataset if this is `nil`.
	Dataset  *TemplateDataset `json:"dataset"`
	Language string           `json:"language"`
}

type Template struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	CodeStr   string                 `json:"code"`
	DatasetID string                 `json:"dataset"`
	Language  JobProgrammingLanguage `json:"language"`
}

const templateManifestExtension = ".json"

func ReadTemplates(templatesDir string, m *DatasetManagerT) ([]Template, error) {
	log.Tracef("Searching for templates in directory `%s`", templatesDir)

	items, err := os.ReadDir(templatesDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Infof("No templates loaded from `%s`: %s", templatesDir, err.Error())
			return make([]Template, 0), nil
		}
		return nil, err
	}

	templates := make([]Template, 0)

	for _, entry := range items {
		candidateName := path.Join(entry.Name())
		ext := path.Ext(candidateName)
		if entry.IsDir() || ext != templateManifestExtension {
			log.Debugf(
				"    -> Skipping file `%s` in templates directory: filetype `%s` !- `%s`",
				entry.Name(),
				ext,
				templateManifestExtension,
			)
			continue
		}

		//
		// Read manifest.
		//
		manifestPath := path.Join(templatesDir, candidateName)
		manifestString, err := os.ReadFile(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("read manifest `%s`: %s", manifestPath, err.Error())
		}

		var manifestStruct TemplateManifest
		if err := json.Unmarshal(manifestString, &manifestStruct); err != nil {
			return nil, fmt.Errorf("unmarshal `%s`: %s", manifestPath, err.Error())
		}

		//
		// Read code file.
		//

		codePath := path.Join(templatesDir, manifestStruct.CodePath)
		codeString, err := os.ReadFile(codePath)
		if err != nil {
			return nil, fmt.Errorf("read code file `%s` specified in `%s`: %s", codePath, manifestPath, err.Error())
		}

		lang := JobProgrammingLanguage(manifestStruct.Language)
		if err := lang.Validate(); err != nil {
			return nil, fmt.Errorf("invalid language `%s` specified in `%s`", lang, manifestPath)
		}

		//
		// Read dataset file.
		//
		dsID := *m.EmptyDatasetID

		if manifestStruct.Dataset != nil {
			dsPath := path.Join(templatesDir, manifestStruct.Dataset.Path)
			dsFile, err := os.ReadFile(dsPath)
			if err != nil {
				return nil, fmt.Errorf("read DS file `%s` specified in `%s`: %s", dsPath, manifestPath, err.Error())
			}

			dsID, err = m.AddDataset(manifestStruct.Dataset.Name, dsFile)
			if err != nil {
				return nil, fmt.Errorf(
					"create DS `%s` specified in `%s`: %s",
					manifestStruct.Dataset.Name,
					manifestPath, err.Error(),
				)
			}
		}

		templates = append(templates, Template{
			ID:        candidateName,
			Name:      manifestStruct.Name,
			CodeStr:   string(codeString),
			DatasetID: dsID,
			Language:  lang,
		})
	}

	log.Infof("Loaded %d templates from `%s`", len(templates), templatesDir)

	for idx, tmpl := range templates {
		log.Debugf("    -> [%d] `%s` (%s)", idx, tmpl.ID, tmpl.Name)
	}

	return templates, nil
}

func Init(
	logger *logrus.Logger,
	compilerConfig CompilerConfig,
	datasetDirPath string,
	templatesDirPath string,
	maxJobRuntimeSecs uint64,
	nodesBlackList []string,
) error {
	log = logger
	log.Trace("Initializing logic package...")

	compiler, err := NewCompiler(compilerConfig)
	if err != nil {
		logger.Errorf("Failed to connect to remote compiler service: %s", err.Error())
		return fmt.Errorf("compiler system error: %s", err.Error())
	}

	UIManager = NewUIManager()
	go UIManager.WaitAndNotify()

	dsManager, err := newDatasetManager(datasetDirPath)
	if err != nil {
		return fmt.Errorf("initialize dataset subsystem: %s", err.Error())
	}
	DatasetManager = dsManager

	//
	// Read templates.
	//

	templates, err := ReadTemplates(templatesDirPath, &DatasetManager)
	if err != nil {
		return fmt.Errorf("read templates: %s", err.Error())
	}

	JobManager = newJobManager(
		templates,
		&compiler,
		UIManager.FromDatasources,
		UIManager.TriggerDataSourceChan,
		maxJobRuntimeSecs,
		nodesBlackList,
	)
	if err := JobManager.Init(); err != nil {
		return err
	}

	log.Debug("Logic package successfully initialized")
	return nil
}
