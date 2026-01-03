# ---------- Build Stage ----------
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build tools (important for some deps)
RUN apk add --no-cache git build-base

# Copy go mod files first
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# 🔥 IMPORTANT FIXES
RUN go mod tidy
RUN go build -o productcatalog

# ---------- Run Stage ----------
FROM alpine:3.19

WORKDIR /app

# Copy products folder
COPY products/ /app/products/

# Copy binary
COPY --from=builder /app/productcatalog /app/productcatalog

EXPOSE 8088
ENV PRODUCT_CATALOG_SERVICE_PORT=8088

ENTRYPOINT ["./productcatalog"]
