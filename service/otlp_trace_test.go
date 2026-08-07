package service

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

func findAttr(attrs []*commonpb.KeyValue, key string) (*commonpb.AnyValue, bool) {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value, true
		}
	}
	return nil, false
}

func firstSpan(t *testing.T, payload []byte) (*tracepb.TracesData, *tracepb.Span) {
	t.Helper()
	var td tracepb.TracesData
	require.NoError(t, proto.Unmarshal(payload, &td))
	require.Len(t, td.ResourceSpans, 1)
	require.Len(t, td.ResourceSpans[0].ScopeSpans, 1)
	require.Len(t, td.ResourceSpans[0].ScopeSpans[0].Spans, 1)
	return &td, td.ResourceSpans[0].ScopeSpans[0].Spans[0]
}

// TC1: success inputs produce a well-formed span with the expected attributes.
func TestBuildOTLPTracePayload_Success(t *testing.T) {
	requestBody := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	responseBody := []byte(`{"id":"resp-1","choices":[{"message":{"role":"assistant","content":"hello"}}]}`)

	payload, err := BuildOTLPTracePayload(TracePayloadParams{
		RequestBody:      requestBody,
		ResponseBody:     responseBody,
		Model:            "gpt-4o",
		TokenId:          7,
		TokenName:        "t1",
		ChannelId:        3,
		RelayMode:        relayconstant.RelayModeChatCompletions,
		StartTime:        time.Now(),
		EndTime:          time.Now(),
		StatusCode:       200,
		IsError:          false,
		IsStream:         false,
		PromptTokens:     10,
		CompletionTokens: 5,
		TotalTokens:      15,
		HasUsage:         true,
	})
	require.NoError(t, err)

	td, span := firstSpan(t, payload)

	svcName, ok := findAttr(td.ResourceSpans[0].Resource.Attributes, "service.name")
	require.True(t, ok)
	assert.Equal(t, "new-api", svcName.GetStringValue())

	assert.Len(t, span.TraceId, 16)
	assert.Len(t, span.SpanId, 8)

	kind, ok := findAttr(span.Attributes, "openinference.span.kind")
	require.True(t, ok)
	assert.Equal(t, "LLM", kind.GetStringValue())

	inputValue, ok := findAttr(span.Attributes, "input.value")
	require.True(t, ok)
	assert.Equal(t, string(requestBody), inputValue.GetStringValue())

	inputMime, ok := findAttr(span.Attributes, "input.mime_type")
	require.True(t, ok)
	assert.Equal(t, "application/json", inputMime.GetStringValue())

	outputValue, ok := findAttr(span.Attributes, "output.value")
	require.True(t, ok)
	assert.Equal(t, string(responseBody), outputValue.GetStringValue())

	modelName, ok := findAttr(span.Attributes, "llm.model_name")
	require.True(t, ok)
	assert.Equal(t, "gpt-4o", modelName.GetStringValue())

	promptCount, ok := findAttr(span.Attributes, "llm.token_count.prompt")
	require.True(t, ok)
	assert.Equal(t, int64(10), promptCount.GetIntValue())

	completionCount, ok := findAttr(span.Attributes, "llm.token_count.completion")
	require.True(t, ok)
	assert.Equal(t, int64(5), completionCount.GetIntValue())

	totalCount, ok := findAttr(span.Attributes, "llm.token_count.total")
	require.True(t, ok)
	assert.Equal(t, int64(15), totalCount.GetIntValue())

	tokenId, ok := findAttr(span.Attributes, "newapi.token.id")
	require.True(t, ok)
	assert.Equal(t, int64(7), tokenId.GetIntValue())

	tokenName, ok := findAttr(span.Attributes, "newapi.token.name")
	require.True(t, ok)
	assert.Equal(t, "t1", tokenName.GetStringValue())

	channelId, ok := findAttr(span.Attributes, "newapi.channel.id")
	require.True(t, ok)
	assert.Equal(t, int64(3), channelId.GetIntValue())

	statusCode, ok := findAttr(span.Attributes, "newapi.http.status_code")
	require.True(t, ok)
	assert.Equal(t, int64(200), statusCode.GetIntValue())

	assert.Equal(t, tracepb.Status_STATUS_CODE_OK, span.Status.Code)
}

// TC2: ExportOTLPTrace POSTs the payload to the configured endpoint exactly once.
func TestExportOTLPTrace_PostsPayload(t *testing.T) {
	initDefaultHTTPClientFixture(t)

	type captured struct {
		method      string
		path        string
		contentType string
		body        []byte
	}
	received := make(chan captured, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- captured{method: r.Method, path: r.URL.Path, contentType: r.Header.Get("Content-Type"), body: body}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	payload, err := BuildOTLPTracePayload(TracePayloadParams{
		RequestBody:  []byte(`{"model":"gpt-4o"}`),
		ResponseBody: []byte(`{"ok":true}`),
		Model:        "gpt-4o",
		TokenId:      1,
		TokenName:    "t",
		ChannelId:    1,
		RelayMode:    relayconstant.RelayModeChatCompletions,
		StartTime:    time.Now(),
		EndTime:      time.Now(),
		StatusCode:   200,
	})
	require.NoError(t, err)

	ExportOTLPTrace(server.URL+"/v1/traces", payload)

	select {
	case got := <-received:
		assert.Equal(t, http.MethodPost, got.method)
		assert.Equal(t, "/v1/traces", got.path)
		assert.Equal(t, "application/x-protobuf", got.contentType)
		var td tracepb.TracesData
		require.NoError(t, proto.Unmarshal(got.body, &td))
		assert.Len(t, td.ResourceSpans, 1)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OTLP export request")
	}

	select {
	case <-received:
		t.Fatal("expected exactly one export request")
	default:
	}
}

// TC3: failure inputs mark the span as errored and omit token count attributes.
func TestBuildOTLPTracePayload_Failure(t *testing.T) {
	payload, err := BuildOTLPTracePayload(TracePayloadParams{
		RequestBody:       []byte(`{"model":"gpt-4o"}`),
		UpstreamErrorBody: `{"error":"upstream boom"}`,
		Model:             "gpt-4o",
		TokenId:           1,
		TokenName:         "t",
		ChannelId:         1,
		RelayMode:         relayconstant.RelayModeChatCompletions,
		StartTime:         time.Now(),
		EndTime:           time.Now(),
		StatusCode:        502,
		IsError:           true,
		HasUsage:          false,
	})
	require.NoError(t, err)

	_, span := firstSpan(t, payload)

	assert.Equal(t, tracepb.Status_STATUS_CODE_ERROR, span.Status.Code)
	assert.NotEmpty(t, span.Status.Message)

	outputValue, ok := findAttr(span.Attributes, "output.value")
	require.True(t, ok)
	assert.Contains(t, outputValue.GetStringValue(), "upstream boom")

	statusCode, ok := findAttr(span.Attributes, "newapi.http.status_code")
	require.True(t, ok)
	assert.Equal(t, int64(502), statusCode.GetIntValue())

	_, ok = findAttr(span.Attributes, "llm.token_count.prompt")
	assert.False(t, ok)
	_, ok = findAttr(span.Attributes, "llm.token_count.completion")
	assert.False(t, ok)
	_, ok = findAttr(span.Attributes, "llm.token_count.total")
	assert.False(t, ok)
}

// TC4: StartRelayTrace returns false when tracing is disabled, or when enabled
// but no endpoint is configured; no export request must ever be sent.
func TestStartRelayTrace_DisabledOrMissingEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	received := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI}

	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	common.SetContextKey(c1, constant.ContextKeyTokenTraceEnabled, false)
	common.SetContextKey(c1, constant.ContextKeyTokenTraceEndpoint, server.URL+"/v1/traces")
	assert.False(t, StartRelayTrace(c1, info))

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	common.SetContextKey(c2, constant.ContextKeyTokenTraceEnabled, true)
	common.SetContextKey(c2, constant.ContextKeyTokenTraceEndpoint, "")
	assert.False(t, StartRelayTrace(c2, info))

	select {
	case <-received:
		t.Fatal("expected zero export requests when tracing is not active")
	default:
	}

	// Neither call to StartRelayTrace created a traceBuffer on c1/c2, so
	// getTraceBuffer returns nil and ExportRelayTrace must early-return
	// without ever posting to the collector.
	ExportRelayTrace(c1, info, nil)
	ExportRelayTrace(c2, info, nil)

	select {
	case <-received:
		t.Fatal("expected zero export requests when no trace buffer was started")
	case <-time.After(200 * time.Millisecond):
	}
}

// TC5: StartRelayTrace activates tracing when enabled with an endpoint;
// TraceCaptureResponseText fills the buffer and feeds a stream payload.
func TestStartRelayTrace_EnabledCapturesStreamText(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	common.SetContextKey(c, constant.ContextKeyTokenTraceEnabled, true)
	common.SetContextKey(c, constant.ContextKeyTokenTraceEndpoint, "http://collector.local/v1/traces")

	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI}
	require.True(t, StartRelayTrace(c, info))

	TraceCaptureResponseText(c, "Hello world")

	buf := getTraceBuffer(c)
	require.NotNil(t, buf)
	assert.Equal(t, "Hello world", buf.ResponseText)
	assert.True(t, buf.ResponseIsStream)

	payload, err := BuildOTLPTracePayload(TracePayloadParams{
		RequestBody:  []byte(`{"model":"gpt-4o"}`),
		ResponseText: "Hello world",
		IsStream:     true,
		Model:        "gpt-4o",
		TokenId:      1,
		TokenName:    "t",
		ChannelId:    1,
		RelayMode:    relayconstant.RelayModeChatCompletions,
		StartTime:    time.Now(),
		EndTime:      time.Now(),
		StatusCode:   200,
	})
	require.NoError(t, err)

	_, span := firstSpan(t, payload)
	outputValue, ok := findAttr(span.Attributes, "output.value")
	require.True(t, ok)
	assert.Equal(t, "Hello world", outputValue.GetStringValue())

	outputMime, ok := findAttr(span.Attributes, "output.mime_type")
	require.True(t, ok)
	assert.Equal(t, "text/plain", outputMime.GetStringValue())
}

// TC6: with the value-size cap removed, input.value and output.value are
// exported VERBATIM at any size. A request/response larger than the old
// 131072-byte cap is emitted intact (full length, tail preserved) and no
// newapi.truncated attribute is present.
func TestBuildOTLPTracePayload_FullValueNoTruncation(t *testing.T) {
	const oversized = 131072 + 5000
	inputTail := "INPUT_TAIL_MARKER_END"
	outputTail := "OUTPUT_TAIL_MARKER_END"
	longInput := strings.Repeat("a", oversized) + inputTail
	longOutput := strings.Repeat("b", oversized) + outputTail

	payload, err := BuildOTLPTracePayload(TracePayloadParams{
		RequestBody:  []byte(longInput),
		ResponseBody: []byte(longOutput),
		Model:        "gpt-4o",
		TokenId:      1,
		TokenName:    "t",
		ChannelId:    1,
		RelayMode:    relayconstant.RelayModeChatCompletions,
		StartTime:    time.Now(),
		EndTime:      time.Now(),
		StatusCode:   200,
	})
	require.NoError(t, err)

	_, span := firstSpan(t, payload)

	// input.value is intact: full length, over the old cap, tail preserved.
	inputValue, ok := findAttr(span.Attributes, "input.value")
	require.True(t, ok)
	assert.Equal(t, longInput, inputValue.GetStringValue())
	assert.Greater(t, len(inputValue.GetStringValue()), 131072)
	assert.True(t, strings.HasSuffix(inputValue.GetStringValue(), inputTail))

	// output.value is intact too.
	outputValue, ok := findAttr(span.Attributes, "output.value")
	require.True(t, ok)
	assert.Equal(t, longOutput, outputValue.GetStringValue())
	assert.Greater(t, len(outputValue.GetStringValue()), 131072)
	assert.True(t, strings.HasSuffix(outputValue.GetStringValue(), outputTail))

	// The truncation flag is gone entirely.
	_, ok = findAttr(span.Attributes, "newapi.truncated")
	assert.False(t, ok)
}

// TC7: end-to-end ExportRelayTrace run with a token key and channel key held
// on the *relaycommon.RelayInfo — the exact fields ExportRelayTrace actually
// reads (info.TokenKey, info.ChannelMeta.ApiKey) — proves those secrets never
// reach the exported OTLP payload actually POSTed to the collector, while the
// token identity (name) is still emitted.
func TestExportRelayTrace_NoSecretsLeak(t *testing.T) {
	initDefaultHTTPClientFixture(t)
	gin.SetMode(gin.TestMode)

	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`))
	c.Set("token_name", "safe-token-name")
	common.SetContextKey(c, constant.ContextKeyTokenTraceEnabled, true)
	common.SetContextKey(c, constant.ContextKeyTokenTraceEndpoint, server.URL)

	info := &relaycommon.RelayInfo{
		TokenId:         1,
		TokenKey:        "sk-TOKENSECRET",
		OriginModelName: "gpt-4o",
		StartTime:       time.Now(),
		RelayFormat:     types.RelayFormatOpenAI,
	}
	info.ChannelMeta = &relaycommon.ChannelMeta{ChannelId: 3, ApiKey: "ck-CHANNELSECRET"}

	require.True(t, StartRelayTrace(c, info))

	TraceCaptureResponseBody(c, []byte(`{"choices":[{"message":{"content":"hi there"}}]}`))

	ExportRelayTrace(c, info, nil)

	select {
	case body := <-received:
		require.NotEmpty(t, body)
		bodyStr := string(body)

		// Proves the token IDENTITY (its name) is emitted, and a real span was built.
		// Proto encodes strings as raw UTF-8 bytes, so substring checks on the raw
		// wire bytes still work.
		assert.Contains(t, bodyStr, "safe-token-name")
		assert.Contains(t, bodyStr, "gpt-4o")

		// Non-vacuous core: the token KEY (info.TokenKey) and the channel KEY
		// (info.ChannelMeta.ApiKey) must never appear in the exported payload.
		assert.NotContains(t, bodyStr, "sk-TOKENSECRET")
		assert.NotContains(t, bodyStr, "ck-CHANNELSECRET")

		// Belt-and-suspenders: every attribute key on the span must be within the
		// known whitelist, so any future attribute leaking an unexpected value is
		// caught even if its value happens not to match a hardcoded secret string.
		_, span := firstSpan(t, body)
		allowedAttrKeys := map[string]bool{
			"openinference.span.kind":    true,
			"input.value":                true,
			"input.mime_type":            true,
			"output.value":               true,
			"output.mime_type":           true,
			"llm.model_name":             true,
			"llm.token_count.prompt":     true,
			"llm.token_count.completion": true,
			"llm.token_count.total":      true,
			"newapi.token.id":            true,
			"newapi.token.name":          true,
			"newapi.channel.id":          true,
			"newapi.http.status_code":    true,
			"newapi.output.omitted":      true,
		}
		for _, attr := range span.Attributes {
			assert.True(t, allowedAttrKeys[attr.Key], "unexpected span attribute key: %s", attr.Key)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OTLP export request")
	}
}

// TC11(a): audio speech responses are binary, so the span omits output.value
// and records newapi.output.omitted instead.
func TestBuildOTLPTracePayload_AudioSpeechOmitsOutput(t *testing.T) {
	payload, err := BuildOTLPTracePayload(TracePayloadParams{
		RequestBody: []byte(`{"model":"tts-1","input":"hello"}`),
		Model:       "tts-1",
		TokenId:     1,
		TokenName:   "t",
		ChannelId:   1,
		RelayMode:   relayconstant.RelayModeAudioSpeech,
		StartTime:   time.Now(),
		EndTime:     time.Now(),
		StatusCode:  200,
		IsError:     false,
	})
	require.NoError(t, err)

	_, span := firstSpan(t, payload)

	_, ok := findAttr(span.Attributes, "output.value")
	assert.False(t, ok)

	omitted, ok := findAttr(span.Attributes, "newapi.output.omitted")
	require.True(t, ok)
	assert.Equal(t, "binary", omitted.GetStringValue())
}

// TC11(b): TraceCaptureUsage fills the buffer, and those counts flow through
// to the llm.token_count.* attributes.
func TestTraceCaptureUsage_SetsBufferAndPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	common.SetContextKey(c, constant.ContextKeyTokenTraceEnabled, true)
	common.SetContextKey(c, constant.ContextKeyTokenTraceEndpoint, "http://collector.local/v1/traces")

	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI}
	require.True(t, StartRelayTrace(c, info))

	TraceCaptureUsage(c, &dto.Usage{PromptTokens: 3, CompletionTokens: 4, TotalTokens: 7})

	buf := getTraceBuffer(c)
	require.NotNil(t, buf)
	assert.True(t, buf.HasUsage)
	assert.Equal(t, 3, buf.PromptTokens)
	assert.Equal(t, 4, buf.CompletionTokens)
	assert.Equal(t, 7, buf.TotalTokens)

	payload, err := BuildOTLPTracePayload(TracePayloadParams{
		RequestBody:      []byte(`{"model":"gpt-4o"}`),
		ResponseBody:     []byte(`{"ok":true}`),
		Model:            "gpt-4o",
		TokenId:          1,
		TokenName:        "t",
		ChannelId:        1,
		RelayMode:        relayconstant.RelayModeChatCompletions,
		StartTime:        time.Now(),
		EndTime:          time.Now(),
		StatusCode:       200,
		HasUsage:         true,
		PromptTokens:     buf.PromptTokens,
		CompletionTokens: buf.CompletionTokens,
		TotalTokens:      buf.TotalTokens,
	})
	require.NoError(t, err)

	_, span := firstSpan(t, payload)

	promptCount, ok := findAttr(span.Attributes, "llm.token_count.prompt")
	require.True(t, ok)
	assert.Equal(t, int64(3), promptCount.GetIntValue())

	completionCount, ok := findAttr(span.Attributes, "llm.token_count.completion")
	require.True(t, ok)
	assert.Equal(t, int64(4), completionCount.GetIntValue())

	totalCount, ok := findAttr(span.Attributes, "llm.token_count.total")
	require.True(t, ok)
	assert.Equal(t, int64(7), totalCount.GetIntValue())
}

// TC12: a non-empty Project routes the span to a named backend project via the
// openinference.project.name RESOURCE attribute; an empty Project omits it so
// spans fall back to the backend's default project. The attribute must live on
// the Resource, never on the span, because that is where Phoenix/OpenInference
// reads project routing.
func TestBuildOTLPTracePayload_ProjectRouting(t *testing.T) {
	withProject, err := BuildOTLPTracePayload(TracePayloadParams{
		RequestBody:  []byte(`{"model":"gpt-4o"}`),
		ResponseBody: []byte(`{"ok":true}`),
		Model:        "gpt-4o",
		Project:      "team-alpha",
		TokenId:      1,
		TokenName:    "t",
		ChannelId:    1,
		RelayMode:    relayconstant.RelayModeChatCompletions,
		StartTime:    time.Now(),
		EndTime:      time.Now(),
		StatusCode:   200,
	})
	require.NoError(t, err)

	td, span := firstSpan(t, withProject)

	project, ok := findAttr(td.ResourceSpans[0].Resource.Attributes, "openinference.project.name")
	require.True(t, ok)
	assert.Equal(t, "team-alpha", project.GetStringValue())

	// service.name stays alongside the project routing attribute.
	svcName, ok := findAttr(td.ResourceSpans[0].Resource.Attributes, "service.name")
	require.True(t, ok)
	assert.Equal(t, "new-api", svcName.GetStringValue())

	// Routing is a RESOURCE attribute, never a span attribute.
	_, ok = findAttr(span.Attributes, "openinference.project.name")
	assert.False(t, ok)

	withoutProject, err := BuildOTLPTracePayload(TracePayloadParams{
		RequestBody:  []byte(`{"model":"gpt-4o"}`),
		ResponseBody: []byte(`{"ok":true}`),
		Model:        "gpt-4o",
		Project:      "",
		TokenId:      1,
		TokenName:    "t",
		ChannelId:    1,
		RelayMode:    relayconstant.RelayModeChatCompletions,
		StartTime:    time.Now(),
		EndTime:      time.Now(),
		StatusCode:   200,
	})
	require.NoError(t, err)

	td2, _ := firstSpan(t, withoutProject)
	_, ok = findAttr(td2.ResourceSpans[0].Resource.Attributes, "openinference.project.name")
	assert.False(t, ok)
}

// TC13: StartRelayTrace copies the per-token trace project from the request
// context onto the trace buffer, so ExportRelayTrace can route the span.
func TestStartRelayTrace_CapturesProject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	common.SetContextKey(c, constant.ContextKeyTokenTraceEnabled, true)
	common.SetContextKey(c, constant.ContextKeyTokenTraceEndpoint, "http://collector.local/v1/traces")
	common.SetContextKey(c, constant.ContextKeyTokenTraceProject, "team-alpha")

	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI}
	require.True(t, StartRelayTrace(c, info))

	buf := getTraceBuffer(c)
	require.NotNil(t, buf)
	assert.Equal(t, "team-alpha", buf.Project)
}

// TestExportOTLPTrace_LogsNon2xx verifies a non-2xx response from the collector
// is logged via common.SysError with the status code, and never panics.
func TestExportOTLPTrace_LogsNon2xx(t *testing.T) {
	initDefaultHTTPClientFixture(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnsupportedMediaType)
		_, _ = w.Write([]byte("unsupported media type"))
	}))
	defer server.Close()

	var logBuffer bytes.Buffer
	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = oldWriter
		common.LogWriterMu.Unlock()
	})

	payload, err := BuildOTLPTracePayload(TracePayloadParams{
		RequestBody:  []byte(`{"model":"gpt-4o"}`),
		ResponseBody: []byte(`{"ok":true}`),
		Model:        "gpt-4o",
		TokenId:      1,
		TokenName:    "t",
		ChannelId:    1,
		RelayMode:    relayconstant.RelayModeChatCompletions,
		StartTime:    time.Now(),
		EndTime:      time.Now(),
		StatusCode:   200,
	})
	require.NoError(t, err)

	require.NotPanics(t, func() { ExportOTLPTrace(server.URL, payload) })

	assert.Contains(t, logBuffer.String(), "status 415")
}
