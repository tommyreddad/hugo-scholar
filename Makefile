GO_FILES := $(shell find cmd internal -type f -name '*.go')
PRETTIER_FILES := $(wildcard *.md .github/*.yaml .github/workflows/*.yaml)

.PHONY: check fmt fmt-check test vet build clean

check: fmt-check test vet build

fmt:
	gofmt -w $(GO_FILES)
	prettier --write $(PRETTIER_FILES)

fmt-check:
	@files="$$(gofmt -l $(GO_FILES))"; \
	if [ -n "$$files" ]; then \
		printf 'Unformatted Go files:\n%s\n' "$$files"; \
		exit 1; \
	fi

test:
	go test ./...

vet:
	go vet ./...

build:
	hugo --source example --minify

clean:
	rm -rf -- example/public example/resources example/.hugo_build.lock
