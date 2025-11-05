FROM golang:1.22-alpine AS build

WORKDIR /app

# Use Go build cache
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    mkdir -p /root/.cache/go-build

COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build ONLY the HTTP main file
RUN go build -o productcatalog main.go

####################################

FROM alpine:3.19

WORKDIR /app

# Copy products.json to working directory
COPY products/products.json /app/products.json

# Copy built binary
COPY --from=builder /app/productcatalog /app/productcatalog

EXPOSE 8088

ENV PRODUCT_CATALOG_SERVICE_PORT=8088

ENTRYPOINT ["./productcatalog"]
