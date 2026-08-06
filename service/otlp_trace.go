package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

// MaxTraceValueBytes bounds the size of a single input.value/output.value
// attribute written into an OTLP trace span, to avoid unbounded payload
// growth on large requests/responses.
const MaxTraceValueBytes = 131072

// maxTraceStatusMessageBytes bounds the short masked error summary put on
// the span status when the relay failed.
const maxTraceStatusMessageBytes = 300

// traceBuffer accumulates the per-request data needed to build one OTLP
// trace span. StartRelayTrace creates it on the gin.Context, capture helpers
// fill it in as the relay progresses, and ExportRelayTrace reads it once at
// the end of the request to build and export the span.
type traceBuffer struct {
	Endpoint          string
	ResponseBody      []byte
	ResponseText      string
	ResponseIsStream  bool
	UpstreamErrorBody string
	PromptTokens      int
	CompletionTokens  int
	TotalTokens       int
	HasUsage          bool
}

// StartRelayTrace decides whether per-token OTLP tracing applies to this
// request and, if so, creates a traceBuffer on the gin.Context. Returns false
// (and creates no buffer) for realtime relays, when the token has tracing
// disabled, or when no trace endpoint is configured.
func StartRelayTrace(c *gin.Context, info *relaycommon.RelayInfo) bool {
	if info != nil && info.RelayFormat == types.RelayFormatOpenAIRealtime {
		return false
	}
	if !common.GetContextKeyBool(c, constant.ContextKeyTokenTraceEnabled) {
		return false
	}
	endpoint := common.GetContextKeyString(c, constant.ContextKeyTokenTraceEndpoint)
	if endpoint == "" {
		return false
	}
	c.Set(string(constant.ContextKeyTraceBuffer), &traceBuffer{Endpoint: endpoint})
	return true
}

// getTraceBuffer returns the traceBuffer stored on c by StartRelayTrace, or
// nil when tracing is not active for this request.
func getTraceBuffer(c *gin.Context) *traceBuffer {
	value, exists := c.Get(string(constant.ContextKeyTraceBuffer))
	if !exists {
		return nil
	}
	buf, ok := value.(*traceBuffer)
	if !ok {
		return nil
	}
	return buf
}

// TraceCaptureResponseBody stores a copy of a non-streaming response body
// for tracing. No-op when tracing is not active.
func TraceCaptureResponseBody(c *gin.Context, data []byte) {
	buf := getTraceBuffer(c)
	if buf == nil {
		return
	}
	buf.ResponseBody = append([]byte(nil), data...)
	buf.ResponseIsStream = false
}

// TraceCaptureResponseText stores the assembled text of a streaming response
// for tracing. No-op when tracing is not active.
func TraceCaptureResponseText(c *gin.Context, text string) {
	buf := getTraceBuffer(c)
	if buf == nil {
		return
	}
	buf.ResponseText = text
	buf.ResponseIsStream = true
}

// TraceCaptureUpstreamError stores the raw upstream error body for tracing.
// No-op when tracing is not active.
func TraceCaptureUpstreamError(c *gin.Context, body string) {
	buf := getTraceBuffer(c)
	if buf == nil {
		return
	}
	buf.UpstreamErrorBody = body
}

// TraceCaptureUsage stores token usage counts for tracing. No-op when
// tracing is not active or usage is nil.
func TraceCaptureUsage(c *gin.Context, usage *dto.Usage) {
	buf := getTraceBuffer(c)
	if buf == nil || usage == nil {
		return
	}
	buf.PromptTokens = usage.PromptTokens
	buf.CompletionTokens = usage.CompletionTokens
	buf.TotalTokens = usage.TotalTokens
	buf.HasUsage = true
}

func stringAttr(key, value string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: value}}}
}

func intAttr(key string, value int) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: int64(value)}}}
}

func boolAttr(key string, value bool) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: value}}}
}

// truncateToBytes cuts s to at most limit bytes, backing off to the previous
// UTF-8 rune boundary so multi-byte characters are never split.
func truncateToBytes(s string, limit int) (string, bool) {
	if len(s) <= limit {
		return s, false
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut], true
}

// truncateTraceValue applies the MaxTraceValueBytes bound used for
// input.value/output.value attributes.
func truncateTraceValue(s string) (string, bool) {
	return truncateToBytes(s, MaxTraceValueBytes)
}

// maskedErrorSummary builds a short, masked, non-sensitive status message
// from the raw upstream error body.
func maskedErrorSummary(body string, statusCode int) string {
	masked, _ := truncateToBytes(kitutil.MaskSensitiveInfo(body), maxTraceStatusMessageBytes)
	if masked == "" {
		return "status_code=" + strconv.Itoa(statusCode)
	}
	return "status_code=" + strconv.Itoa(statusCode) + ": " + masked
}

// TracePayloadParams holds the explicit inputs needed to build one OTLP
// trace span. All fields are plain data — no gin.Context, no token/channel
// secrets — so BuildOTLPTracePayload stays a pure function.
type TracePayloadParams struct {
	RequestBody       []byte
	ResponseBody      []byte
	ResponseText      string
	IsStream          bool
	UpstreamErrorBody string
	Model             string
	TokenId           int
	TokenName         string
	ChannelId         int
	RelayMode         int
	StartTime         time.Time
	EndTime           time.Time
	StatusCode        int
	IsError           bool
	PromptTokens      int
	CompletionTokens  int
	TotalTokens       int
	HasUsage          bool
}

// BuildOTLPTracePayload builds one OTLP/OpenInference trace span (wrapped in
// the standard ResourceSpans/ScopeSpans envelope) as OTLP protobuf-encoded
// bytes. Pure function: no I/O, no gin.Context. Never includes a token key,
// channel key, Authorization header, or DSN.
func BuildOTLPTracePayload(p TracePayloadParams) ([]byte, error) {
	traceIdBytes := make([]byte, 16)
	if _, err := rand.Read(traceIdBytes); err != nil {
		return nil, err
	}
	spanIdBytes := make([]byte, 8)
	if _, err := rand.Read(spanIdBytes); err != nil {
		return nil, err
	}

	name := p.Model
	if name == "" {
		name = "llm"
	}

	inputValue, truncated := truncateTraceValue(string(p.RequestBody))
	attrs := []*commonpb.KeyValue{
		stringAttr("openinference.span.kind", "LLM"),
		stringAttr("input.value", inputValue),
		stringAttr("input.mime_type", "application/json"),
	}

	switch {
	case p.IsError:
		outputValue, outputTruncated := truncateTraceValue(p.UpstreamErrorBody)
		truncated = truncated || outputTruncated
		attrs = append(attrs, stringAttr("output.value", outputValue), stringAttr("output.mime_type", "application/json"))
	case p.RelayMode == relayconstant.RelayModeAudioSpeech:
		attrs = append(attrs, stringAttr("newapi.output.omitted", "binary"))
	case p.IsStream:
		outputValue, outputTruncated := truncateTraceValue(p.ResponseText)
		truncated = truncated || outputTruncated
		attrs = append(attrs, stringAttr("output.value", outputValue), stringAttr("output.mime_type", "text/plain"))
	default:
		outputValue, outputTruncated := truncateTraceValue(string(p.ResponseBody))
		truncated = truncated || outputTruncated
		attrs = append(attrs, stringAttr("output.value", outputValue), stringAttr("output.mime_type", "application/json"))
	}

	attrs = append(attrs, stringAttr("llm.model_name", p.Model))
	if p.HasUsage {
		attrs = append(attrs,
			intAttr("llm.token_count.prompt", p.PromptTokens),
			intAttr("llm.token_count.completion", p.CompletionTokens),
			intAttr("llm.token_count.total", p.TotalTokens),
		)
	}
	attrs = append(attrs,
		intAttr("newapi.token.id", p.TokenId),
		stringAttr("newapi.token.name", p.TokenName),
		intAttr("newapi.channel.id", p.ChannelId),
		intAttr("newapi.http.status_code", p.StatusCode),
	)
	if truncated {
		attrs = append(attrs, boolAttr("newapi.truncated", true))
	}

	status := &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK}
	if p.IsError {
		status.Code = tracepb.Status_STATUS_CODE_ERROR
		status.Message = maskedErrorSummary(p.UpstreamErrorBody, p.StatusCode)
	}

	span := &tracepb.Span{
		TraceId:           traceIdBytes,
		SpanId:            spanIdBytes,
		Name:              name,
		Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
		StartTimeUnixNano: uint64(p.StartTime.UnixNano()),
		EndTimeUnixNano:   uint64(p.EndTime.UnixNano()),
		Attributes:        attrs,
		Status:            status,
	}

	ss := &tracepb.ScopeSpans{
		Scope: &commonpb.InstrumentationScope{Name: "new-api"},
		Spans: []*tracepb.Span{span},
	}
	rs := &tracepb.ResourceSpans{
		Resource:   &resourcepb.Resource{Attributes: []*commonpb.KeyValue{stringAttr("service.name", "new-api")}},
		ScopeSpans: []*tracepb.ScopeSpans{ss},
	}

	return proto.Marshal(&tracepb.TracesData{ResourceSpans: []*tracepb.ResourceSpans{rs}})
}

// ExportRelayTrace is the deferred entry point for per-token OTLP tracing.
// It must be called synchronously while c is still valid: it reads the
// request/response data captured on c's traceBuffer, builds the span
// payload, then hands the actual HTTP export off to a background goroutine
// that never touches c.
func ExportRelayTrace(c *gin.Context, info *relaycommon.RelayInfo, apiErr *types.NewAPIError) {
	buf := getTraceBuffer(c)
	if buf == nil {
		return
	}

	var requestBody []byte
	if storage, err := common.GetBodyStorage(c); err == nil {
		if body, err := storage.Bytes(); err == nil {
			requestBody = body
		}
	}

	statusCode := http.StatusOK
	isError := false
	if apiErr != nil {
		statusCode = apiErr.StatusCode
		isError = true
	}

	payload, err := BuildOTLPTracePayload(TracePayloadParams{
		RequestBody:       requestBody,
		ResponseBody:      buf.ResponseBody,
		ResponseText:      buf.ResponseText,
		IsStream:          buf.ResponseIsStream,
		UpstreamErrorBody: buf.UpstreamErrorBody,
		Model:             info.OriginModelName,
		TokenId:           info.TokenId,
		TokenName:         c.GetString("token_name"),
		ChannelId:         info.GetChannelID(),
		RelayMode:         info.RelayMode,
		StartTime:         info.StartTime,
		EndTime:           time.Now(),
		StatusCode:        statusCode,
		IsError:           isError,
		PromptTokens:      buf.PromptTokens,
		CompletionTokens:  buf.CompletionTokens,
		TotalTokens:       buf.TotalTokens,
		HasUsage:          buf.HasUsage,
	})
	if err != nil {
		common.SysError("otlp trace build failed: " + err.Error())
		return
	}

	endpoint := buf.Endpoint
	gopool.Go(func() {
		ExportOTLPTrace(endpoint, payload)
	})
}

// ExportOTLPTrace POSTs a pre-built OTLP trace payload to endpoint. Runs on a
// background goroutine (see ExportRelayTrace), so it must never reference a
// gin.Context. Never returns an error: failures are logged and dropped,
// since trace export must never affect the relay response.
func ExportOTLPTrace(endpoint string, payload []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		common.SysError("otlp trace export failed: " + err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/x-protobuf")

	resp, err := GetHttpClient().Do(req)
	if err != nil {
		common.SysError("otlp trace export failed: " + err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		preview, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		common.SysError(fmt.Sprintf("otlp trace export failed: status %d, body: %s", resp.StatusCode, preview))
	}
}
