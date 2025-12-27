FROM squidfunk/mkdocs-material:latest

WORKDIR /docs
COPY docs/requirements.txt /docs/requirements.txt
RUN pip install --no-cache-dir -r /docs/requirements.txt
