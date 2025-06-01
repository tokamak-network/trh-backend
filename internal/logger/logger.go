package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

func Init() {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.StacktraceKey = ""
	config.Encoding = "console"

	var err error
	Logger, err = config.Build()
	if err != nil {
		panic("failed to initialize logger: " + err.Error())
	}
}

func Info(msg string, fields ...zap.Field) {
	Logger.Info(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	Logger.Error(msg, fields...)
}

func Debug(msg string, fields ...zap.Field) {
	Logger.Debug(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	Logger.Warn(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	Logger.Fatal(msg, fields...)
}

func Infof(msg string, args ...interface{}) {
	Logger.Sugar().Infof(msg, args...)
}

func Errorf(msg string, args ...interface{}) {
	Logger.Sugar().Errorf(msg, args...)
}

func Debugf(msg string, args ...interface{}) {
	Logger.Sugar().Debugf(msg, args...)
}

func Warnf(msg string, args ...interface{}) {
	Logger.Sugar().Warnf(msg, args...)
}

func Fatalf(msg string, args ...interface{}) {
	Logger.Sugar().Fatalf(msg, args...)
}
