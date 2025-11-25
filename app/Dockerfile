FROM nginx:alpine AS base
RUN addgroup -g 1001 appgroup && adduser -u 1001 -G appgroup -D appuser \
    && chown -R appuser:appgroup /usr/share/nginx/html /var/cache/nginx /var/log/nginx \
    && touch /var/run/nginx.pid && chown appuser:appgroup /var/run/nginx.pid

FROM base AS final
COPY --chown=appuser:appgroup index.html /usr/share/nginx/html/
COPY --chown=appuser:appgroup nginx.conf /etc/nginx/nginx.conf
USER appuser
EXPOSE 8080
