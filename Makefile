GOFMT_FILES?=$$(find . -name '*.go' |grep -v vendor)
PKG_NAME=boring-registry

default: build

build:
	go install github.com/TierMobility/boring-registry/cmd/boring-registry/...

test:
	go test -i $(TEST) || exit 1
	echo $(TEST) | \
	
testacc:
	TF_ACC=1 go test $(TEST) -v $(TESTARGS) -timeout 120m

vet:
	@echo "go vet ."
	@go vet $$(go list ./... | grep -v vendor/) ; if [ $$? -eq 1 ]; then \
		echo ""; \
		echo "Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for review."; \
		exit 1; \
	fi

fmt:
	gofmt -w $(GOFMT_FILES)	xargs -t -n4 go test $(TESTARGS) -timeout=30s -parallel=4

.PHONY: build test testacc vet fmt
