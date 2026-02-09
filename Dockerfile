# --- Build Stage ---
FROM golang:1.25.7 AS builder
WORKDIR /app

# Using separate COPY for go.mod/sum to leverage Docker layer caching
COPY go.sum go.mod ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 is used because modernc.org/sqlite is a pure Go implementation.
# This ensures a static binary and simplifies the final image environment.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o go-todo cmd/go-todo/main.go 


# --- Final Image ---
FROM ubuntu:jammy
WORKDIR /app
RUN mkdir -p /app/data

# Copying only the binary to keep the final image size minimal and secure
COPY --from=builder /app/go-todo .

# Ensure the frontend assets are available in the same relative path
COPY web ./web

# Default environment variables for flexibility. 
ENV TODO_HOST="0.0.0.0"
ENV TODO_PORT=7540
ENV TODO_DBFILE=/app/data/scheduler.db
ENV TODO_PASSWORD="" 
ENV TODO_SECRETKEY=""

# Expose for documentation.
EXPOSE 7540

CMD ["./go-todo"]