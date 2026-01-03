# ---------- Build Stage ----------
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o productcatalog


# ---------- Run Stage ----------
FROM alpine:3.19

WORKDIR /app

# Copy products folder (not only file)
COPY products/ /app/products/

# Copy binary
COPY --from=builder /app/productcatalog /app/productcatalog

EXPOSE 8088
ENV PRODUCT_CATALOG_SERVICE_PORT=8088

ENTRYPOINT ["./productcatalog"]
