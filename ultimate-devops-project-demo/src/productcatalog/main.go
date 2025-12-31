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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	pb "github.com/opentelemetry/opentelemetry-demo/src/product-catalog/genproto/oteldemo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

/* ---------------- GLOBALS ---------------- */

var (
	log               = logrus.New()
	catalog           []*pb.Product
	resource          *sdkresource.Resource
	initResourcesOnce sync.Once
)

/* ---------------- INIT ---------------- */

func init() {
	var err error
	catalog, err = readProductFiles()
	if err != nil {
		log.Fatalf("Failed to read products: %v", err)
	}
}

/* ---------------- OTEL ---------------- */

func initResource() *sdkresource.Resource {
	initResourcesOnce.Do(func() {
		resource, _ = sdkresource.Merge(
			sdkresource.Default(),
			sdkresource.NewWithAttributes(),
		)
	})
	return resource
}

func initTracer() *sdktrace.TracerProvider {
	tp := sdktrace.NewTracerProvider(
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

/* ---------------- MAIN ---------------- */

func main() {
	tp := initTracer()
	defer tp.Shutdown(context.Background())

	var port string
	mustMapEnv(&port, "PRODUCT_CATALOG_PORT")

	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatal(err)
	}

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

/* ---------------- REST (FIXED CONTRACT) ---------------- */

type ProductListResponse struct {
	Products []*pb.Product `json:"products"`
}

func handleListProducts(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ProductListResponse{Products: catalog})
}

func handleGetProduct(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]

	for _, p := range catalog {
		if p.Id == id {
			json.NewEncoder(w).Encode(map[string]*pb.Product{
				"product": p,
			})
			return
		}
	}
	http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
}

/* ---------------- GRPC ---------------- */

type productCatalog struct {
	pb.UnimplementedProductCatalogServiceServer
}

func (p *productCatalog) ListProducts(
	ctx context.Context,
	_ *pb.Empty,
) (*pb.ListProductsResponse, error) {
	trace.SpanFromContext(ctx).SetAttributes(
		attribute.Int("products.count", len(catalog)),
	)
	return &pb.ListProductsResponse{Products: catalog}, nil
}

func (p *productCatalog) GetProduct(
	ctx context.Context,
	req *pb.GetProductRequest,
) (*pb.Product, error) {
	for _, product := range catalog {
		if product.Id == req.Id {
			return product, nil
		}
	}
	return nil, status.Errorf(codes.NotFound, "product not found")
}

/* ---------------- HEALTH (FIX) ---------------- */

func (p *productCatalog) Check(
	context.Context,
	*healthpb.HealthCheckRequest,
) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{
		Status: healthpb.HealthCheckResponse_SERVING,
	}, nil
}

func (p *productCatalog) Watch(
	*healthpb.HealthCheckRequest,
	healthpb.Health_WatchServer,
) error {
	return status.Errorf(codes.Unimplemented, "watch not implemented")
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
