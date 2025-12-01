# Build the manager binary
FROM golang:1.25 AS builder
ARG TARGETOS
Run golangci/golangci-lint-action@v8
prepare environment
run golangci-lint
  Running [/home/runner/golangci-lint-2.6.2-linux-amd64/golangci-lint config path] in [/home/runner/work/krkn-operator-acm/krkn-operator-acm] ...
  Running [/home/runner/golangci-lint-2.6.2-linux-amd64/golangci-lint config verify] in [/home/runner/work/krkn-operator-acm/krkn-operator-acm] ...
  Running [/home/runner/golangci-lint-2.6.2-linux-amd64/golangci-lint run] in [/home/runner/work/krkn-operator-acm/krkn-operator-acm] ...
  Error: internal/controller/krkntargetrequest_controller.go:149:34: string `Completed` has 5 occurrences, make it a constant (goconst)
  	if krknRequest.Status.Status == "Completed" {
  	                                ^
  Error: internal/controller/krkntargetrequest_controller.go:173:2: Consider pre-allocating `targetData` (prealloc)
  	var targetData []krknv1alpha1.ClusterTarget
  	^
  Error: internal/controller/krkntargetrequest_controller.go:346:100: (*KrknTargetRequestReconciler).generateKubeconfig - caCrt is unused (unparam)
  func (r *KrknTargetRequestReconciler) generateKubeconfig(clusterName, clusterURL, clusterCABundle, caCrt, token string) (string, error) {
                                                                                                     ^
  3 issues:
  * goconst: 1
  * prealloc: 1
  * unparam: 1
  Error: issues found
  Ran golangci-lint in 77261msù
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY internal/ internal/

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager cmd/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]