package logic

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

func Init(logger *logrus.Logger, compilerConfig CompilerConfig, datasetDirPath string) error {
	log = logger
	log.Trace("Initializing logic package...")

	compiler, err := NewCompiler(compilerConfig)
	if err != nil {
		logger.Errorf("Failed to connect to remote compiler service: %s", err.Error())
		return fmt.Errorf("compiler system error: %s", err.Error())
	}

	JobManager = newJobManager(&compiler)
	if err := JobManager.init(); err != nil {
		return err
	}

	dsManager, err := newDatasetManager(datasetDirPath)
	if err != nil {
		return fmt.Errorf("initialize dataset subsystem: %s", err.Error())
	}
	DatasetManager = dsManager

	log.Debug("Logic package successfully initialized")
	return nil
}
