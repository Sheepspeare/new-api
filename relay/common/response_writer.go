package common

import (
	"bytes"
	appcommon "github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

type LogResponseWriter struct {
	gin.ResponseWriter
	body        *bytes.Buffer
	maxCapacity int
}

func NewLogResponseWriter(c *gin.Context) *LogResponseWriter {
	return &LogResponseWriter{
		ResponseWriter: c.Writer,
		body:        bytes.NewBuffer(make([]byte, 0, 1024)),
		maxCapacity: appcommon.LogDetailMaxSize, // 限制最大容量，防 OOM
	}
}

func (w *LogResponseWriter) Write(b []byte) (int, error) {
	if w.body.Len() < w.maxCapacity {
		toWrite := len(b)
		if w.body.Len()+toWrite > w.maxCapacity {
			toWrite = w.maxCapacity - w.body.Len()
			w.body.Write(b[:toWrite])
		} else {
			w.body.Write(b)
		}
	}
	return w.ResponseWriter.Write(b)
}

func (w *LogResponseWriter) WriteString(s string) (int, error) {
	if w.body.Len() < w.maxCapacity {
		toWrite := len(s)
		if w.body.Len()+toWrite > w.maxCapacity {
			toWrite = w.maxCapacity - w.body.Len()
			w.body.WriteString(s[:toWrite])
		} else {
			w.body.WriteString(s)
		}
	}
	return w.ResponseWriter.WriteString(s)
}

func (w *LogResponseWriter) GetBody() string {
	return w.body.String()
}
