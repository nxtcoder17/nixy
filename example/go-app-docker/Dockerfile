FROM debian:bookworm-slim AS runtime
COPY .dist/app/nix /nix
COPY .dist/app/app-store-path /tmp/nixy-app-store-path


# INFO: SINCE, docker can't copy symlinks, while the `app` directory in a nix build artifact is always a symlink to some path in /nix store, we need to do this shenanigan
RUN ln -s "$(cat /tmp/nixy-app-store-path)" /app \
  && rm -f /tmp/nixy-app-store-path

RUN ls -al /app/bin

FROM gcr.io/distroless/static-debian12
COPY --from=runtime /nix /nix
COPY --from=runtime /app /app
ENV PATH="/app/bin:${PATH}"
CMD ["go-app"]
