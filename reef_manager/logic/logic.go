package logic

import "github.com/sirupsen/logrus"

var log *logrus.Logger

func Init(logger *logrus.Logger) error {
	log = logger
	log.Trace("Initializing logic package...")

	JobManager = newJobManager()
	if err := JobManager.init(); err != nil {
		return err
	}

	NodeManager = newNodeManager()

	log.Debug("Logic package sucessfully initialized")
	return nil
}
