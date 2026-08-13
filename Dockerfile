FROM golang:1.23-alpine AS builder
RUN apk add --no-cache git
WORKDIR /build
COPY agentorgs-controller/go.mod agentorgs-controller/go.sum ./
RUN go mod download
COPY agentorgs-controller/ ./
RUN CGO_ENABLED=0 go build -o /agentorgs-controller ./cmd/controller/
RUN CGO_ENABLED=0 go build -o /ago ./cmd/ago/

FROM alpine:3.20
RUN apk add --no-cache ca-certificates kubectl
COPY --from=builder /agentorgs-controller /usr/local/bin/agentorgs-controller
COPY --from=builder /ago /usr/local/bin/ago
COPY config/crd/ /opt/agentorgs/config/crd/
ENV AGENTORGS_HTTP_ADDR=:8090
ENTRYPOINT ["agentorgs-controller"]
