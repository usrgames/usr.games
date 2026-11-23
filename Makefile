.PHONY: check fmtcheck test vet shellcheck site

# Everything CI runs. A page, a drawn file or a link that has drifted fails here.
check: fmtcheck test vet shellcheck
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

# The installer, and the scripts that pin releases and keep the player, have to pass shellcheck
# as plain sh. Where there is no shellcheck this says so and passes; CI runs it for real.
shellcheck:
	@echo "Checking the installer and the scripts..."
	@if command -v shellcheck >/dev/null; then shellcheck -s sh site/install.sh scripts/pin.sh scripts/player.sh; else echo "no shellcheck here: not checked"; fi

# Writes every made file again: the drawings (the sign, the names, the icons, the card) from
# internal/brand, and the pages from games.json and the templates in internal/sitepages. Run it
# after a change to either, then commit what it wrote.
site:
	@echo "Drawing and writing the site..."
	go test ./internal/brand -run TestTheFilesAreWhatTheSourceDraws -update
	go test ./internal/sitepages -run TestThePagesAreWhatTheSourceSays -update
