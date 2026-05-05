FROM node:22-alpine AS web
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.24-alpine AS server
WORKDIR /app
ENV GOTOOLCHAIN=auto
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o bin/server ./cmd/server

FROM alpine:3.21
WORKDIR /app
COPY --from=server /app/bin/server ./server
COPY --from=web /app/web/dist ./web/dist
RUN mkdir -p data
EXPOSE 8080
CMD ["./server", "-addr", ":8080", "-data", "data", "-web", "web/dist"]
