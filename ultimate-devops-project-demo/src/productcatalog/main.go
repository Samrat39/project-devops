// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/soheilhy/cmux"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	otelhooks "github.com/open-feature/go-sdk-contrib/hooks/open-telemetry/pkg"
	flagd "github.com/open-feature/go-sdk-contrib/providers/flagd/pkg"
	"github.com/open-feature/go-sdk/openfeature"

	pb "github.com/opentelemetry/opentelemetry-demo/src/product-catalog/genproto/oteldemo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	log               *logrus.Logger
	catalog           []*pb.Product
	resource          *sdkresource.Resource
	initResourcesOnce sync.Once
)

func init() {
	log = logrus.New()
	var err error
	catalog, err = readProductFiles()
	if err != nil {
		log.Fatalf("Reading Product Files: %v", err)
	}
}

/* ---------------- OTEL INIT ---------------- */

func initResource() *sdkresource.Resource {
	initResourcesOnce.Do(func() {
		extraResources, _ := sdkresource.New(
			context.Background(),
			sdkresource.WithOS(),
			sdkresource.WithProcess(),
			sdkresource.WithContainer(),
			sdkresource.WithHost(),
		)
		resource, _ = sdkresource.Merge(sdkresource.Default(), extraResources)
	})
	return resource
}

func initTracerProvider() *sdktrace.TracerProvider {
	exporter, _ := otlptracegrpc.New(context.Background())
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(initResource()),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)
	return tp
}

func initMeterProvider() *sdkmetric.MeterProvider {
	exporter, _ := otlpmetricgrpc.New(context.Background())
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		sdkmetric.WithResource(initResource()),
	)
	otel.SetMeterProvider(mp)
	return mp
}

/* ---------------- MAIN ---------------- */

func main() {
	tp := initTracerProvider()
	defer tp.Shutdown(context.Background())

	mp := initMeterProvider()
	defer mp.Shutdown(context.Background())

	openfeature.AddHooks(otelhooks.NewTracesHook())
	openfeature.SetProvider(flagd.NewProvider())

	runtime.Start()

	var port string
	mustMapEnv(&port, "PRODUCT_CATALOG_PORT")

	ln, _ := net.Listen("tcp", ":"+port)
	m := cmux.New(ln)

	grpcL := m.Match(cmux.HTTP2())
	httpL := m.Match(cmux.HTTP1Fast())

	grpcSrv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	svc := &productCatalog{}
	pb.RegisterProductCatalogServiceServer(grpcSrv, svc)
	healthpb.RegisterHealthServer(grpcSrv, svc)
	reflection.Register(grpcSrv)

	router := mux.NewRouter()
	router.HandleFunc("/api/products", handleListProducts).Methods(http.MethodGet)
	router.HandleFunc("/api/products/{id}", handleGetProduct).Methods(http.MethodGet)
	httpSrv := &http.Server{Handler: router}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go grpcSrv.Serve(grpcL)
	go httpSrv.Serve(httpL)
	go m.Serve()

	<-ctx.Done()
	httpSrv.Shutdown(context.Background())
	grpcSrv.GracefulStop()
}

/* ---------------- REST FIX (IMPORTANT) ---------------- */

type ProductListResponse struct {
	Products []*pb.Product `json:"products"`
}

func handleListProducts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(
		ProductListResponse{
			Products: catalog,
		},
	)
}

func handleGetProduct(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]

	for _, p := range catalog {
		if p.Id == id {
			json.NewEncoder(w).Encode(
				map[string]*pb.Product{
					"product": p,
				},
			)
			return
		}
	}
	http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
}

/* ---------------- GRPC ---------------- */

type productCatalog struct {
	pb.UnimplementedProductCatalogServiceServer
}

func (p *productCatalog) ListProducts(ctx context.Context, _ *pb.Empty) (*pb.ListProductsResponse, error) {
	trace.SpanFromContext(ctx).SetAttributes(
		attribute.Int("app.products.count", len(catalog)),
	)
	return &pb.ListProductsResponse{Products: catalog}, nil
}

func (p *productCatalog) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.Product, error) {
	for _, product := range catalog {
		if product.Id == req.Id {
			return product, nil
		}
	}
	return nil, status.Errorf(codes.NotFound, "product not found")
}

func (p *productCatalog) Check(context.Context, *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

/* ---------------- HELPERS ---------------- */

func mustMapEnv(target *string, key string) {
	if v, ok := os.LookupEnv(key); ok {
		*target = v
	} else {
		log.Fatalf("Missing env var: %s", key)
	}
}

func readProductFiles() ([]*pb.Product, error) {
	entries, err := os.ReadDir("./products")
	if err != nil {
		return nil, err
	}

	var products []*pb.Product
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			data, _ := os.ReadFile("./products/" + entry.Name())
			var res pb.ListProductsResponse
			protojson.Unmarshal(data, &res)
			products = append(products, res.Products...)
		}
	}
	log.Infof("Loaded %d products", len(products))
	return products, nil
}
