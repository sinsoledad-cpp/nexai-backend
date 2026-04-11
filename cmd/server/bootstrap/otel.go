package bootstrap

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// InitOTEL 返回一个关闭函数，并且让调用者关闭的时候来决定这个 ctx
// InitOTEL 初始化 OpenTelemetry
func InitOTEL() func(context.Context) {
	ctx := context.Background()
	res, err := newResource("bedrock", "v0.0.1")
	if err != nil {
		panic(err)
	}

	prop := newPropagator()
	otel.SetTextMapPropagator(prop)

	// ---------------------------------------------------------
	// 1. 初始化 OTLP Exporter (用于发给 Jaeger 4318 端口)
	// ---------------------------------------------------------
	otlpExporter, err := otlptracehttp.New(ctx,
		// 如果你修改了 docker 映射端口，这里记得改成 localhost:14318
		otlptracehttp.WithEndpoint("localhost:4318"),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		panic(err)
	}

	// ---------------------------------------------------------
	// 2. 初始化 Zipkin Exporter (用于发给 Zipkin 9411 端口)
	// ---------------------------------------------------------
	// 注意：Jaeger 本身也兼容 Zipkin 协议，所以这个地址既可以是
	// 真正的 Zipkin Server，也可以是 Jaeger 的 9411 端口。
	zipkinExporter, err := zipkin.New("http://localhost:19411/api/v2/spans")
	if err != nil {
		panic(err)
	}

	// ---------------------------------------------------------
	// 3. 注册多个 Exporter 到 Provider
	// ---------------------------------------------------------
	// 关键点：多次调用 trace.WithBatcher 即可
	tp := trace.NewTracerProvider(
		trace.WithResource(res),

		// 第一个导出通道：发往 OTLP (Jaeger)
		trace.WithBatcher(otlpExporter, trace.WithBatchTimeout(time.Second)),

		// 第二个导出通道：发往 Zipkin
		trace.WithBatcher(zipkinExporter, trace.WithBatchTimeout(time.Second)),
	)

	otel.SetTracerProvider(tp)

	return func(ctx context.Context) {
		_ = tp.Shutdown(ctx)
	}
}

// 产生遥测数据的实体
func newResource(serviceName, serviceVersion string) (*resource.Resource, error) {
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		))
}

// 创建一个“传播器”
func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}
