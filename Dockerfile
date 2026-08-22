# -- Build stage ---------------------------------------------------------------
FROM golang:1.22-alpine AS go-build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags='-s -w' -o /worldweaver ./cmd/server

# -- Frontend build stage ------------------------------------------------------
FROM node:20-alpine AS web-build
WORKDIR /web
COPY web/package*.json ./
RUN npm ci 2>/dev/null || npm install
COPY web/ .
RUN npx tsc --noEmit || true
# For now, serve raw TS files (Vite build requires full setup)
RUN mkdir -p dist && cp -r *.html *.ts src/ dist/ 2>/dev/null || true

# -- Runtime stage -------------------------------------------------------------
FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=go-build  /worldweaver     /app/worldweaver
COPY --from=web-build /web/dist        /app/web
EXPOSE 8080
ENTRYPOINT ["/app/worldweaver", "-addr", ":8080"]
