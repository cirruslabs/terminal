package server

import (
	"context"
	"github.com/blendle/zapdriver"
	"github.com/cirruslabs/terminal/internal/xcloudtracecontext"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

func (ts *TerminalServer) TraceContext(ctx context.Context) []zap.Field {
	var noContext []zap.Field

	if ts.gcpProjectID == "" {
		return noContext
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return noContext
	}

	headers := md.Get("X-Cloud-Trace-Context")
	if len(headers) != 1 {
		return noContext
	}

	traceID, spanID, traceSampled := xcloudtracecontext.DeconstructXCloudTraceContext(headers[0])

	return zapdriver.TraceContext(traceID, spanID, traceSampled, ts.gcpProjectID)
}
