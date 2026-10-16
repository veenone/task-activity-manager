# Gate commands. AGENTS.project.md cites `make gates` for rule P3.
.PHONY: gates test typecheck vet instruction lint ratchet

gates: vet test typecheck lint ratchet
	@echo "all gates passed"

vet:
	cd core && go vet ./...
	cd tam && go vet ./...
	cd xtm && go vet ./...

test:
	cd core && go test ./... -count=1
	cd tam && go test ./... -count=1
	cd xtm && go test ./internal/... -count=1
	npm test --workspaces --if-present

typecheck:
	npm run typecheck --workspaces --if-present

# The instruction gate runs inside `npm test --workspaces` (frontend/core).
# Match the prefix: the store schema and purge contract lives in its own
# file beside the main one.
instruction:
	cd frontend/core && npx vitest run src/instruction-gate

lint:
	sh scripts/lint-report.sh

ratchet:
	sh scripts/ratchet.sh
