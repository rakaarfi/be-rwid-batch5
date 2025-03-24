package logger

import (
	"log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LoggerInstance menyimpan logger yang sudah diinisialisasi.
var (
	ErrorLogger    *zap.Logger
	AuditLogger    *zap.Logger
	RequestLogger  *zap.Logger
	SecurityLogger *zap.Logger
	SystemLogger   *zap.Logger
	ContextLogger  *zap.Logger
)

func newLogger(filePath string, level zapcore.Level) (*zap.Logger, error) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	ws := zapcore.AddSync(file)

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		ws,
		level,
	)
	return zap.New(core), nil
}

func InitLoggers() {
	var err error
	ErrorLogger, err = newLogger("logs/errors.log", zapcore.ErrorLevel)
	if err != nil {
		log.Fatalf("Cannot create error logger: %v", err)
	}
	AuditLogger, err = newLogger("logs/audit.log", zapcore.InfoLevel)
	if err != nil {
		log.Fatalf("Cannot create audit logger: %v", err)
	}
	RequestLogger, err = newLogger("logs/request.log", zapcore.InfoLevel)
	if err != nil {
		log.Fatalf("Cannot create request logger: %v", err)
	}
	SecurityLogger, err = newLogger("logs/security.log", zapcore.WarnLevel)
	if err != nil {
		log.Fatalf("Cannot create security logger: %v", err)
	}
	SystemLogger, err = newLogger("logs/system.log", zapcore.InfoLevel)
	if err != nil {
		log.Fatalf("Cannot create system logger: %v", err)
	}
	ContextLogger, err = newLogger("logs/context.log", zapcore.DebugLevel)
	if err != nil {
		log.Fatalf("Cannot create context logger: %v", err)
	}
}

func SyncLoggers() {
	_ = ErrorLogger.Sync()
	_ = AuditLogger.Sync()
	_ = RequestLogger.Sync()
	_ = SecurityLogger.Sync()
	_ = SystemLogger.Sync()
	_ = ContextLogger.Sync()
}
