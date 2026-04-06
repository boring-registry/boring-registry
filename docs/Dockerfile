FROM squidfunk/mkdocs-material:9.7.6@sha256:868ad4d39fb5865b72d00173ade00f4eae2b38dde7ff790a011cc44ce4a8ff8e

WORKDIR /docs
COPY docs/requirements.txt /docs/requirements.txt
RUN pip install --no-cache-dir -r /docs/requirements.txt

CMD ["serve", "--dev-addr=0.0.0.0:8000", "--livereload"]
