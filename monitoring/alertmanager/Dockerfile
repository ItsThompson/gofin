FROM prom/alertmanager:v0.32.1 AS alertmanager

FROM alpine:3.19

RUN apk add --no-cache gettext

COPY --from=alertmanager /bin/alertmanager /bin/alertmanager
COPY --from=alertmanager /bin/amtool /bin/amtool

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

RUN mkdir -p /etc/alertmanager /alertmanager

EXPOSE 9093

ENTRYPOINT ["/entrypoint.sh"]
CMD ["--config.file=/etc/alertmanager/alertmanager.yml", "--storage.path=/alertmanager"]
