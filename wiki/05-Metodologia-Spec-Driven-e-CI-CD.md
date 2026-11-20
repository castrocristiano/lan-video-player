# 05 — Metodologia Spec-Driven (SDD) e CI/CD

## Fluxo Spec-Driven Development (Spec Kit)

Todas as funcionalidades seguem o fluxo constitucional definido em `.specify/memory/constitution.md`:

1. **Constituição** (`.specify/memory/constitution.md` v1.1.0): Princípios inegociáveis (100% offline/LAN, somente-leitura, binário único, YAGNI, GNU AGPL v3.0, Podman rootless).
2. **Especificação** (`specs/001-lan-video-streaming/spec.md`): Histórias de usuário (`US1`–`US4`), requisitos (`FR-001`..`FR-019`) e critérios mensuráveis (`SC-001`..`SC-008`).
3. **Plano & Contratos** (`plan.md`, `research.md`, `data-model.md`, `contracts/http-api.md`, `quickstart.md`).
4. **Tarefas & Implementação** (`tasks.md`): 42 tarefas rastreadas e validadas com testes automatizados.

---

## Esteira de CI/CD (GitHub Actions + SonarQube)

O workflow `.github/workflows/ci.yml` executa automaticamente em push para `main`, `develop`, `feature/**`, `phase/**` e `hotfix*`:

1. **`build-and-test`**:
   - `go vet ./...`
   - `go test -v -race -coverprofile=coverage.out ./...`
   - Build estático (`CGO_ENABLED=0`) e verificação `--version`.
2. **`sonarqube`**:
   - Sobe um container `sonarqube:10.5-community`, executa o `sonar-scanner-cli` com `sonar-project.properties` e valida o Quality Gate.
3. **`auto-pr-to-develop` / `auto-pr-to-main`**:
   - Abre Pull Requests automaticamente entre branches de feature → `develop` → `main`.
4. **`release-on-main`**:
   - Ao fazer merge na `main`, compila o binário `lan-video-player-linux-amd64` e publica a **GitHub Release** com notas automáticas.
5. **`.github/workflows/deploy-wiki.yml`**:
   - Sincroniza automaticamente a pasta `wiki/` com a **GitHub Wiki** ao atualizar a `main`.
