package telemetry

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/gophish/gophish/logger"
	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otellog "go.opentelemetry.io/otel/log"
)

// BridgeHook is a logrus.Hook that forwards every log entry to the OTel log
// provider, so logs can be exported as OTLP log records in addition to the
// normal logrus output.
type BridgeHook struct{}

var _ logrus.Hook = BridgeHook{}

// Levels returns all levels; every logrus entry is bridged to OTel.
func (BridgeHook) Levels() []logrus.Level { return logrus.AllLevels }

// Fire converts the logrus entry into an OTel log record and emits it.
func (BridgeHook) Fire(e *logrus.Entry) error {
	rec := otellog.Record{}
	rec.SetTimestamp(e.Time)
	rec.SetBody(attribute.StringValue(e.Message))
	rec.SetSeverity(severity(e.Level))
	rec.SetSeverityText(e.Level.String())
	for k, v := range e.Data {
		rec.AddAttributes(attribute.String(k, fmt.Sprintf("%v", v)))
	}
	otelapi.GetLoggerProvider().Logger("gophish").Emit(context.Background(), rec)
	return nil
}

// severity maps logrus levels onto OTel severities.
func severity(l logrus.Level) otellog.Severity {
	switch l {
	case logrus.PanicLevel, logrus.FatalLevel:
		return otellog.SeverityError4
	case logrus.ErrorLevel:
		return otellog.SeverityError1
	case logrus.WarnLevel:
		return otellog.SeverityWarn1
	case logrus.InfoLevel:
		return otellog.SeverityInfo1
	default:
		return otellog.SeverityDebug1
	}
}

// BridgeLogrus installs the logrus→OTel bridge hook on the global logger so
// that logrus entries are also exported as OTel log records.
func BridgeLogrus() {
	logger.Logger.AddHook(BridgeHook{})
}
