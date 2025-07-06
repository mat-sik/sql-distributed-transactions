package logging

import (
	"io"
	"log/slog"
)

func LoggedClose(closer io.Closer) {
	if err := closer.Close(); err != nil {
		slog.Error("encountered error while trying to close a resource", "error", err)
	}
}
