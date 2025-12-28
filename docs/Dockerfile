FROM squidfunk/mkdocs-material:9.6.21@sha256:00f9276315990b82f5af8c47bb2b71e2c69aef9f02a08f8dffd2515f42310753

WORKDIR /docs
COPY docs/requirements.txt /docs/requirements.txt
RUN pip install --no-cache-dir -r /docs/requirements.txt
