# Gate commands. AGENTS.project.md cites `make gates` for rule P3.
.PHONY: gates test typecheck vet instruction ratchet

gates: vet test typecheck ratchet
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
instruction:
	cd frontend/core && npx vitest run src/instruction-gate.test.ts

ratchet:
	sh scripts/ratchet.sh
