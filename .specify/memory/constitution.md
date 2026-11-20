# Constituição do lan-video-player

## Princípios Fundamentais

### I. Local-First e Zero Cloud (INEGOCIÁVEL)

O sistema DEVE operar 100% offline dentro da rede local (LAN), sem exigir acesso à internet em tempo de execução. O projeto NÃO DEVE incluir telemetria, rastreamento de uso, chamadas a APIs externas ou carregamento de scripts, fontes e estilos a partir de CDNs externos; todos os assets da interface Web DEVEM ser servidos localmente pela própria aplicação.

**Justificativa**: Um player de vídeo para LAN existe para privacidade, autonomia e baixa latência em rede doméstica ou corporativa fechada.

### II. Segurança e Acesso Somente-Leitura à Mídia (INEGOCIÁVEL)

Todos os diretórios de mídia configurados pelo usuário DEVEM ser acessados estritamente em modo somente-leitura. A aplicação NUNCA DEVE modificar, renomear, mover ou excluir arquivos de mídia do usuário. Toda rota ou parâmetro de arquivo recebido pela rede DEVE ser canonicalizado e validado para impedir *path traversal* e fuga via links simbólicos fora das raízes explicitamente autorizadas.

**Justificativa**: Expor uma biblioteca de mídia na rede local exige garantia absoluta de integridade dos arquivos do usuário e isolamento do restante do sistema de arquivos.

### III. Binário Único e Web UI Leve

O servidor DEVE ser distribuído como um binário único autossuficiente (implementado em Go ou Rust, a ser definido no plano arquitetural), com os assets estáticos da Web UI embutidos no executável, apto a rodar tanto diretamente quanto dentro de uma imagem de container minimalista. A aplicação NÃO DEVE exigir runtimes pesados, bancos de dados externos ou processos auxiliares obrigatórios para o funcionamento básico, mantendo baixo consumo de memória/CPU e interface leve compatível com navegadores modernos na LAN.

**Justificativa**: Facilita execução leve em container ou host direto em qualquer máquina da rede (incluindo hardware modesto como Raspberry Pi ou servidores domésticos) sem atrito operacional.

### IV. Simplicidade e YAGNI

O produto DEVE priorizar configuração mínima (funcionamento imediato com argumentos simples ou variáveis de ambiente) e escopo enxuto focado na reprodução de vídeos na LAN. Funcionalidades complexas de servidores de mídia pesados (ex.: gestão multi-tenant, ecossistemas de plugins, indexação complexa ou integrações externas) NÃO DEVEM ser introduzidas sem uma especificação aprovada que justifique claramente o custo de manutenção.

**Justificativa**: Complexidade acidental degrada confiabilidade, dificulta testes e afasta o projeto de sua proposta direta e leve.

### V. Conformidade Aberta (GNU AGPL v3.0)

O projeto é licenciado sob a GNU Affero General Public License v3.0 (`LICENSE`). Todo código-fonte, biblioteca de terceiros e asset embutido DEVE ser compatível com a GNU AGPL v3.0. Dependências proprietárias, binários fechados ou artefatos com restrições incompatíveis com software livre NÃO DEVEM ser incorporados ao repositório ou ao binário distribuído.

**Justificativa**: Garante transparência, auditabilidade e preservação das liberdades de software livre inclusive em uso via rede.

## Restrições Técnicas e de Distribuição

- **Stack alvo**: Binário único compilado em **Go** ou **Rust** (definição final documentada no `/speckit-plan` da primeira funcionalidade) acompanhado de Web UI leve embutida e livre de dependências de rede externa.
- **Execução em container (`docker-compose` com Podman)**: O projeto DEVE fornecer empacotamento em container minimalista (OCI/Docker) e arquivo `compose` compatível com **`podman compose` / `podman-compose`** (e `docker compose`). A configuração DEVE funcionar em **Podman rootless**, executar o processo como usuário não-root, montar os diretórios de mídia do host em modo somente-leitura (com suporte documentado a rótulos SELinux como `:ro,Z` ou `:ro,z` no Linux/Fedora) e expor a porta configurável para a rede local.
- **Escopo de rede**: A aplicação DEVE atender dispositivos na LAN via HTTP padrão (navegadores desktop, mobile e TVs compatíveis), sem exigir serviços de descoberta em nuvem ou túneis externos.
- **Portabilidade**: O código central DEVE priorizar portabilidade entre Linux e demais sistemas operacionais suportados pela linguagem escolhida, evitando acoplamento desnecessário a APIs exclusivas de uma única plataforma.
- **Conformidade de licença**: O arquivo `LICENSE` (GNU AGPL v3.0) DEVE permanecer na raiz do repositório e a interface/documentação DEVE respeitar a disponibilização do código-fonte correspondente prevista na Seção 13 da licença.

## Fluxo de Desenvolvimento e Portões de Qualidade

- **Política de testes pragmática**: Testes automatizados (unitários e de integração) SÃO OBRIGATÓRIOS para toda lógica central e contratos críticos, incluindo:
  - validação de caminhos, proteção contra *path traversal* e garantia de acesso somente-leitura;
  - varredura, listagem e metadados de arquivos de vídeo;
  - contratos HTTP, status codes e comportamento de streaming de mídia.
- **Portão de qualidade**: Nenhuma alteração DEVE ser mesclada com falhas nos testes automatizados, erros de compilação ou violações de qualquer princípio desta constituição.
- **Fluxo Spec-Driven**: Novas funcionalidades DEVEM seguir o fluxo Spec Kit (`/speckit-specify` → `/speckit-plan` → `/speckit-tasks` → `/speckit-implement`), com verificação explícita de conformidade constitucional antes do início da implementação.

## Governança

- **Supremacia**: Esta constituição prevalece sobre convenções ad-hoc e escolhas locais de implementação. Em caso de conflito entre um plano/código e esta constituição, a constituição vence até que seja formalmente emendada.
- **Procedimento de emenda**: Alterações nesta constituição DEVEM ser feitas por meio de `/speckit-constitution`, com justificativa explícita da mudança, atualização da versão semântica e revisão dos impactos em especificações ou planos ativos.
- **Política de versionamento**:
  - **MAJOR**: remoção ou redefinição incompatível de princípios fundamentais ou regras de governança.
  - **MINOR**: adição de novo princípio, nova seção ou expansão material de regras existentes.
  - **PATCH**: esclarecimentos, ajustes de redação ou correções sem mudança semântica.
- **Revisão de conformidade**: Todo plano técnico (`/speckit-plan`) e revisão de código DEVE validar aderência aos cinco princípios fundamentais e aos portões de qualidade. Qualquer desvio ou aumento de complexidade DEVE ser justificado por escrito no plano antes da implementação.

**Version**: 1.1.0 | **Ratified**: 2026-11-19 | **Last Amended**: 2026-11-19
