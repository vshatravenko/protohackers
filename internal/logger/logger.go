package logger

import (
	"log/slog"
	"os"
	"strings"
)

const (
	logLevelEnvVar  = "PROTO_LOG_LEVEL"
	defaultLogLevel = "info"
)

var levelMap map[string]slog.Level = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

func ConfigureDefaultLoggerFromEnv() {
	val, ok := os.LookupEnv(logLevelEnvVar)
	if !ok {
		val = defaultLogLevel
	}

	ConfigureDefaultLogger(strings.ToLower(val))
}

func ConfigureDefaultLogger(level string) {
	levelVar := new(slog.LevelVar)

	if newLevel, ok := levelMap[level]; ok {
		levelVar.Set(newLevel)
	} else {
		slog.Info("unsupported log level, setting to info", "level", level)
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: levelVar})
	slog.SetDefault(slog.New(handler))
}
