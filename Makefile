# Fenleap public images
#
#   make list                     show the catalogue
#   make build IMAGE=mysql-no-shell
#   make test  IMAGE=mysql-no-shell
#   make build-all / test-all
#   make push  IMAGE=mysql-no-shell VERSION=1.0.0
#
# Nothing here names a specific image: adding images/<name>/Dockerfile is all
# it takes for a new image to build, test, and release like the others.

REGISTRY  ?= ghcr.io/fenleap
VERSION   ?= dev
PLATFORMS ?= linux/amd64,linux/arm64
DOCKER    ?= docker

IMAGES := $(notdir $(patsubst %/,%,$(dir $(wildcard images/*/Dockerfile))))

export DOCKER_BUILDKIT := 1

.PHONY: list lint build test push build-all test-all clean check-image client-version

list:
	@echo "Images in this repository:"
	@$(foreach i,$(IMAGES),echo "  $(REGISTRY)/$(i)";)

# Which upstream client actually landed in a built image, as opposed to which
# one the build argument asked for.
client-version: check-image
	@./shared/scripts/client-version.sh $(REGISTRY)/$(IMAGE):$(VERSION) \
	  $(firstword $(subst -, ,$(IMAGE)))

check-image:
ifndef IMAGE
	$(error IMAGE is required, e.g. make $(MAKECMDGOALS) IMAGE=mysql-no-shell. Try `make list`)
endif
	@test -f images/$(IMAGE)/Dockerfile || \
	  { echo "No such image: $(IMAGE). Try 'make list'."; exit 1; }

# The launcher is shared by every client image, so it is linted once, centrally.
lint:
	cd shared/launcher && test -z "$$(gofmt -l .)" && go vet ./... && go test ./...

build: check-image
	$(DOCKER) build -f images/$(IMAGE)/Dockerfile . \
	  --build-arg VERSION=$(VERSION) \
	  -t $(REGISTRY)/$(IMAGE):$(VERSION)

# The flavour tells the smoke test which client binary to exercise. Derived from
# the image name so a new "<client>-no-shell" image needs no wiring.
test: check-image build
	./shared/test/smoke.sh $(REGISTRY)/$(IMAGE):$(VERSION) $(firstword $(subst -, ,$(IMAGE)))

build-all:
	@$(foreach i,$(IMAGES),$(MAKE) build IMAGE=$(i) || exit 1;)

test-all: lint
	@$(foreach i,$(IMAGES),$(MAKE) test IMAGE=$(i) || exit 1;)

# buildx cannot --load a multi-platform result, so `make test` (single arch)
# must pass before pushing.
# Releases normally go through CI, which also tags by client version. This is
# the escape hatch; it deliberately does not invent client-version tags, since
# those must come from a tested artifact.
push: check-image
	$(DOCKER) buildx build -f images/$(IMAGE)/Dockerfile . \
	  --platform $(PLATFORMS) --build-arg VERSION=$(VERSION) \
	  -t $(REGISTRY)/$(IMAGE):$(VERSION) -t $(REGISTRY)/$(IMAGE):latest --push

clean:
	@$(foreach i,$(IMAGES),-$(DOCKER) rmi $(REGISTRY)/$(i):$(VERSION) 2>/dev/null;)
