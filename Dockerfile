# Stage 1: Build the React frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2: Build the Go backend
FROM golang:1.22-alpine AS backend-builder
WORKDIR /app/backend
# Cache dependencies separately from source
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /app/aiexplains .

# Stage 3: Minimal runtime image
FROM alpine:3.20
RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=backend-builder /app/aiexplains ./
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

EXPOSE 3000

ENV FRONTEND_DIR=/app/frontend/dist \
    HOST=0.0.0.0

CMD ["./aiexplains", "serve"]
