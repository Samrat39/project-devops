# ---------- Build Stage ----------
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o productcatalog main.go


# ---------- Run Stage ----------
FROM alpine:3.19

WORKDIR /app

# Copy products.json
COPY products/products.json /app/products.json

# Copy binary from builder stage
COPY --from=builder /app/productcatalog /app/productcatalog

EXPOSE 8088
ENV PRODUCT_CATALOG_SERVICE_PORT=8088

ENTRYPOINT ["./productcatalog"]
