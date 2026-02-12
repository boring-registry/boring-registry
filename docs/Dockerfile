FROM squidfunk/mkdocs-material:9.7.1@sha256:3bba0a99bc6e635bb8e53f379d32ab9cecb554adee9cc8f59a347f93ecf82f3b

WORKDIR /docs
COPY docs/requirements.txt /docs/requirements.txt
RUN pip install --no-cache-dir -r /docs/requirements.txt

CMD ["serve", "--dev-addr=0.0.0.0:8000", "--livereload"]
