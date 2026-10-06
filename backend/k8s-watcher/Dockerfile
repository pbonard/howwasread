# both stages on debian 13, the binary needs at least the glibc it was built against
FROM rust:1.99-slim-trixie AS stage1
WORKDIR /app
COPY Cargo.toml Cargo.lock ./
COPY k8s-watcher/Cargo.toml k8s-watcher/
# dependencies are fetched from the manifests alone, so this layer is reused until they change
RUN mkdir k8s-watcher/src && echo 'fn main() {}' > k8s-watcher/src/main.rs && cargo fetch --locked
COPY k8s-watcher/ k8s-watcher/
RUN cargo build --release --locked -p k8s-watcher

# not scratch like the go services: aws-lc, the TLS crypto library, is C code that needs glibc
FROM gcr.io/distroless/cc-debian13
COPY --from=stage1 /app/target/release/k8s-watcher /k8s-watcher
USER 65534:65534
ENTRYPOINT ["/k8s-watcher"]
