.PHONY: check fmtcheck test vet site

# Everything CI runs. A page, a drawn file or a link that has drifted fails here.
check: fmtcheck test vet
	@echo "All checks passed!"

# Lists the files and fails if any is not gofmt-formatted.
fmtcheck:
	@echo "Checking format..."
	@test -z "$$(gofmt -l .)" || { echo "not formatted:"; gofmt -l .; exit 1; }

# The race detector needs cgo; run `make test RACE=` where there is no C compiler.
RACE ?= -race
test:
	@echo "Testing..."
	go test $(RACE) ./...

vet:
	@echo "Vetting..."
	go vet ./...

# Writes every made file again: the drawings (the sign, the names, the icons, the card) from
# internal/brand, and the pages from games.json and the templates in internal/sitepages. Run it
# after a change to either, then commit what it wrote.
site:
	@echo "Drawing and writing the site..."
	go test ./internal/brand -run TestTheFilesAreWhatTheSourceDraws -update
	go test ./internal/sitepages -run TestThePagesAreWhatTheSourceSays -update
