GOFUMPT_CMD := docker run --rm -it -v $(shell pwd):/work ghcr.io/hellt/gofumpt:0.3.1
GOFUMPT_FLAGS := -l -w .

GODOT_CMD := docker run --rm -it -v $(shell pwd):/work ghcr.io/hellt/godot:1.4.11
GODOT_FLAGS := -w .

GOLINES_CMD := docker run --rm -it -v $(shell pwd):/work ghcr.io/hellt/golines:0.10.0 golines
GOLINES_FLAGS := -w .

GOLANGCI_CMD := docker run -it --rm -v $$(pwd):/app -w /app golangci/golangci-lint:v1.47.2 golangci-lint
GOLANGCI_FLAGS := --config ./.github/workflows/linters/.golangci.yml run -v --fix


# when running in a CI env we use locally installed bind
ifdef CI
	GOFUMPT_CMD := gofumpt
endif


format: gofumpt godot golines # apply Go formatters

gofumpt:
	${GOFUMPT_CMD} ${GOFUMPT_FLAGS}

godot:
	${GODOT_CMD} ${GODOT_FLAGS}

golines:
	${GOLINES_CMD} ${GOLINES_FLAGS}

golangci: # linting with golang-ci lint container
	${GOLANGCI_CMD} ${GOLANGCI_FLAGS}