FROM squidfunk/mkdocs-material:9.7.7@sha256:51b87149d227691486b5f08993d28c65ca7e4990010664b697265b8e6fcd5287

WORKDIR /docs
COPY docs/requirements.txt /docs/requirements.txt
RUN pip install --no-cache-dir -r /docs/requirements.txt

CMD ["serve", "--dev-addr=0.0.0.0:8000", "--livereload"]
