FROM prom/alertmanager:v0.32.1 AS alertmanager

FROM alpine:3.21

COPY --from=alertmanager /bin/alertmanager /bin/alertmanager

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

RUN mkdir -p /etc/alertmanager /alertmanager

EXPOSE 9093

ENTRYPOINT ["/entrypoint.sh"]
CMD ["--config.file=/etc/alertmanager/alertmanager.yml", "--storage.path=/alertmanager"]
