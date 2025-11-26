FROM nginxinc/nginx-unprivileged:alpine
COPY --chown=nginx:nginx index.html /usr/share/nginx/html/
COPY --chown=nginx:nginx nginx.conf /etc/nginx/nginx.conf
EXPOSE 8080
