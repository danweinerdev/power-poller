FROM python:3.12-alpine

COPY ["requirements.txt", "/srv/"]
COPY ["kasa_monitor/", "/srv/kasa_monitor/"]

RUN set -ex; \
    apk update; \
    apk add --no-cache git; \
    \
    python3 -m pip install -r /srv/requirements.txt; \
    adduser --home=/srv --shell=/bin/false \
        --disabled-password --no-create-home monitor; \
    chmod 640 -R /srv/kasa_monitor/**/*.py /srv/*.txt; \
    chown monitor:monitor -R /srv; \
    \
    rm -rf /var/cache/apk/*;

USER monitor
WORKDIR /srv

ENV PYTHONUNBUFFERED=1
ENV PYTHONPATH=/srv

ENTRYPOINT ["python3", "-m", "kasa_monitor"]
CMD ["run", "-o", "--loglevel=INFO", "/etc/monitor.conf"]
