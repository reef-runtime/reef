package main

import (
	"fmt"
	"os"

	"github.com/rifflock/lfshook"
	"github.com/sirupsen/logrus"
)

const logLevelDefault = logrus.TraceLevel
const logLevelEnvVarKey = "REEF_LOG_LEVEL"

func newLogger() *logrus.Logger {
	logLevel := logLevelDefault

	if newLogLevel, newLogLevelOk := os.LookupEnv(logLevelEnvVarKey); newLogLevelOk {
		switch newLogLevel {
		case "TRACE":
			logLevel = logrus.TraceLevel
		case "DEBUG":
			logLevel = logrus.DebugLevel
		case "INFO":
			logLevel = logrus.InfoLevel
		case "WARN":
			logLevel = logrus.WarnLevel
		case "ERROR":
			logLevel = logrus.ErrorLevel
		case "FATAL":
			logLevel = logrus.FatalLevel
		default:
			fmt.Printf("[LOGGER] Invalid log level from environment variable: '%s'. Using TRACE\n", newLogLevel)
		}
	}

	// Create new logger.
	logger := logrus.New()
	logger.SetLevel(logLevel)

	// Add filesystem hook in order to log to files.
	pathMap := lfshook.PathMap{
		logrus.InfoLevel:  "./log/application.log",
		logrus.WarnLevel:  "./log/application.log",
		logrus.ErrorLevel: "./log/error.log",
		logrus.FatalLevel: "./log/error.log",
	}
	var hook *lfshook.LfsHook = lfshook.NewHook(
		pathMap,
		&logrus.JSONFormatter{
			TimestampFormat:   "",
			DisableTimestamp:  false,
			DisableHTMLEscape: false,
			DataKey:           "",
			FieldMap:          nil,
			CallerPrettyfier:  nil,
			PrettyPrint:       false,
		},
	)
	logger.Hooks.Add(hook)

	return logger
}
