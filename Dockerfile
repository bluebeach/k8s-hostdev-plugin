FROM scratch

COPY k8s-hostdev-plugin /

ENTRYPOINT ["/k8s-hostdev-plugin"]
