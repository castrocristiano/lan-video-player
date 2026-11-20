# Feature Specification: Player de Vídeo em Rede Local via Navegador

**Feature Branch**: `001-lan-video-streaming`

**Created**: 2026-11-19

**Status**: Draft

**Input**: User description: "Vídeo player para transmitir na rede lan interna uma biblioteca de vídeos, onde pode ser acessado por um navegador web (com pastas e subpastas, busca/filtro, ordenação, miniaturas, player com seek, tela cheia, velocidade, próximo vídeo automático, legendas externas, troca de faixa de áudio, retomar reprodução onde parou, uso em desktop/celular/TV na LAN e configuração com uma ou mais pastas de mídia e porta)"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Navegar em Uma ou Mais Pastas de Mídia pela Rede Local (Priority: P1)

Um operador inicia o serviço apontando para uma ou mais pastas de vídeos no computador anfitrião e definindo a porta desejada na rede local. Qualquer pessoa conectada à mesma rede local abre o endereço do serviço no navegador web (em computador, celular, tablet ou TV compatível) e navega pelas pastas e subpastas da biblioteca para visualizar os vídeos disponíveis, sem instalar aplicativos extras e sem acesso à internet.

**Why this priority**: Sem configurar a biblioteca e navegar pelos vídeos pela rede local, o produto não entrega valor básico. É a porta de entrada obrigatória de toda a experiência.

**Independent Test**: Pode ser testada isoladamente iniciando o serviço com duas pastas de mídia contendo subpastas e vídeos de teste, acessando o endereço em um navegador na rede local e verificando que as raízes configuradas, subpastas, trilha de navegação (breadcrumbs) e arquivos de vídeo aparecem corretamente, ignorando arquivos que não são de vídeo.

**Acceptance Scenarios**:

1. **Given** o serviço está em execução apontando para uma única pasta contendo subpastas, vídeos e documentos diversos, **When** o usuário abre o endereço do serviço em um navegador na rede local, **Then** a interface exibe diretamente o conteúdo dessa pasta raiz mostrando apenas as subpastas e os arquivos de vídeo reconhecidos, com nome, tamanho e data de modificação visíveis.
2. **Given** o serviço está em execução apontando para duas ou mais pastas raiz de mídia (ex.: "Filmes" e "Séries"), **When** o usuário abre a tela inicial, **Then** a interface apresenta cada pasta configurada como ponto de entrada distinto, permitindo entrar em qualquer uma e navegar livremente por suas subpastas com trilha de navegação (breadcrumbs).
3. **Given** o serviço está rodando em uma rede local totalmente sem acesso à internet, **When** o usuário carrega a página no navegador, **Then** toda a interface visual, ícones e estilos carregam completamente sem erros nem dependências externas.
4. **Given** um cliente tenta solicitar manualmente um caminho fora das pastas da biblioteca configuradas (incluindo tentativas de subir níveis de diretório ou seguir atalhos externos), **When** a requisição é processada, **Then** o acesso é recusado e nenhum arquivo ou diretório fora das raízes configuradas é exposto.

---

### User Story 2 - Reproduzir Vídeos no Navegador com Controles Essenciais e Velocidade (Priority: P1)

Ao selecionar qualquer vídeo na biblioteca, o usuário assiste à reprodução diretamente na página do navegador com início rápido e controles completos de reprodução: reproduzir, pausar, ajustar volume, silenciar, tela cheia, controlar a velocidade de reprodução e avançar/retroceder para qualquer instante da linha do tempo sem precisar baixar o vídeo inteiro previamente.

**Why this priority**: É a função central do produto ("vídeo player para transmitir na rede LAN"). Combinada com a História 1, forma o MVP funcional mínimo completo.

**Independent Test**: Pode ser testada selecionando um vídeo de vários minutos na interface web, iniciando a reprodução, saltando diretamente para o meio e o final, alterando a velocidade de reprodução (ex.: 1,5x e 0,75x), pausando, ajustando volume e alternando tela cheia, confirmando reprodução fluida e garantia de que o arquivo original não sofreu nenhuma alteração.

**Acceptance Scenarios**:

1. **Given** o usuário visualiza uma pasta com vídeos disponíveis, **When** seleciona um vídeo compatível com o navegador, **Then** o player abre exibindo o título do vídeo e inicia a transmissão permitindo assistir antes do carregamento completo do arquivo.
2. **Given** um vídeo está em reprodução, **When** o usuário arrasta a barra de progresso para um ponto avançado (ex.: 70% da duração) ou retrocede para um trecho anterior, **Then** a reprodução salta para o instante escolhido sem reiniciar do zero.
3. **Given** um vídeo está em reprodução, **When** o usuário seleciona uma velocidade diferente (ex.: 0,5x, 0,75x, 1x, 1,25x, 1,5x ou 2x), **Then** a velocidade de áudio e vídeo ajusta-se imediatamente e permanece visível nos controles.
4. **Given** o usuário seleciona um arquivo cujo formato não é reproduzível pelo navegador atual, **When** a reprodução falha por incompatibilidade do navegador, **Then** o sistema exibe uma mensagem clara informando que o formato não é suportado por aquele navegador, permitindo voltar à biblioteca sem travar a interface.

---

### User Story 3 - Buscar, Ordenar e Identificar Vídeos por Miniaturas (Priority: P2)

Em bibliotecas médias e grandes, o usuário visualiza miniaturas (thumbnails) nos cartões dos vídeos para reconhecê-los rapidamente, filtra vídeos pelo nome em qualquer pasta da biblioteca e escolhe o critério de ordenação (nome ou data de modificação).

**Why this priority**: Torna a descoberta visual e textual muito mais rápida em bibliotecas reais, podendo ser entregue de forma incremental sobre a navegação básica (P1).

**Independent Test**: Pode ser testada populando a biblioteca com múltiplos vídeos em subpastas distintas, verificando a exibição das miniaturas (ou imagem padrão de fallback quando indisponível), executando buscas parciais insensíveis a maiúsculas/minúsculas e alternando os critérios de ordenação.

**Acceptance Scenarios**:

1. **Given** uma pasta contém arquivos de vídeo, **When** o usuário abre a listagem, **Then** cada vídeo exibe uma miniatura visual de prévia (ou um ícone/miniatura padrão caso a prévia ainda não esteja disponível ou não possa ser gerada), sem criar ou alterar nenhum arquivo dentro das pastas de mídia do usuário.
2. **Given** a biblioteca contém vídeos distribuídos em uma ou mais pastas raiz e subpastas, **When** o usuário digita um termo no campo de busca, **Then** o sistema lista apenas os vídeos correspondentes ao termo (sem diferenciar maiúsculas de minúsculas), exibindo a localização de cada item na biblioteca e permitindo reproduzi-lo diretamente.
3. **Given** o usuário está navegando em uma pasta ou visualizando resultados, **When** altera o critério de ordenação entre nome (A–Z / Z–A) e data de modificação (mais recentes primeiro), **Then** a listagem é reorganizada imediatamente refletindo o critério escolhido.
4. **Given** o usuário busca por um termo que não corresponde a nenhum vídeo, **When** a busca é concluída, **Then** a interface exibe um aviso claro de "nenhum vídeo encontrado" e oferece ação simples para limpar o filtro.

---

### User Story 4 - Retomar Reprodução, Próximo Vídeo Automático, Legendas e Faixas de Áudio (Priority: P3)

Ao assistir conteúdos longos, filmes ou séries em sequência, o usuário pode fechar o navegador e retornar depois retomando exatamente de onde parou, avançar manualmente ou automaticamente para o próximo vídeo da pasta ao término do atual, ativar legendas locais associadas ao vídeo e alternar entre faixas de áudio quando disponíveis.

**Why this priority**: Completa a experiência de player de biblioteca doméstica (séries, aulas e filmes multilíngues) mantendo a simplicidade operacional e podendo ser validada independentemente do MVP.

**Independent Test**: Pode ser testada com uma pasta contendo três vídeos sequenciais, um arquivo de legenda correspondente e um vídeo com múltiplas faixas de áudio suportadas pelo navegador, verificando: retomada após recarregar a página, indicador de progresso na listagem, transição automática e manual para o próximo vídeo (com opção de desativar/cancelar), ativação de legenda e seleção de faixa de áudio.

**Acceptance Scenarios**:

1. **Given** o usuário assistiu parcialmente a um vídeo (ex.: 15 minutos de um vídeo de 40 minutos) e saiu da tela do player, **When** volta à listagem e abre o mesmo vídeo novamente no mesmo dispositivo, **Then** a listagem indica visualmente o progresso parcial e o player retoma a reprodução a partir do ponto salvo (oferecendo opção simples para reiniciar do início).
2. **Given** o usuário concluiu praticamente todo o vídeo (ex.: acima de 95% da duração), **When** o vídeo termina, **Then** o progresso salvo é redefinido como concluído para que futuras reproduções comecem do início.
3. **Given** a opção de reprodução automática do próximo vídeo está ativada e existe um vídeo seguinte na mesma pasta, **When** o vídeo atual chega ao fim, **Then** a interface exibe breve contagem regressiva (com opção de cancelar ou avançar imediatamente) e inicia o próximo vídeo automaticamente.
4. **Given** o usuário está assistindo a um vídeo em uma pasta com vários vídeos, **When** aciona "Próximo vídeo" ou "Vídeo anterior", **Then** o player carrega o vídeo adjacente seguindo a ordem da pasta (e desabilita claramente a ação inexistente no primeiro ou no último item).
5. **Given** existem um ou mais arquivos de legenda locais compatíveis na mesma pasta com o mesmo nome base do vídeo, **When** o usuário abre o vídeo no player, **Then** as legendas aparecem automaticamente nos controles do player permitindo escolher a faixa desejada ou desativar legendas.
6. **Given** um vídeo possui múltiplas faixas de áudio expostas pelo navegador do cliente, **When** o usuário abre o seletor de áudio nos controles do player, **Then** pode alternar entre as faixas disponíveis durante a reprodução; caso haja apenas uma faixa ou o navegador não suporte seleção de faixas de áudio, o controle não quebra a reprodução e mantém o áudio padrão.

---

### Edge Cases

- **Biblioteca vazia ou pasta sem vídeos**: Quando uma pasta configurada (ou subpasta) não contém vídeos suportados, o sistema exibe um estado vazio informativo em vez de tela em branco ou erro técnico.
- **Pasta configurada inválida ou duplicada na inicialização**: Se qualquer pasta informada ao iniciar o serviço não existir, não for um diretório ou não tiver permissão de leitura, o serviço recusa iniciar e informa claramente qual caminho falhou; se duas raízes distintas tiverem o mesmo nome de diretório final, o sistema apresenta rótulos diferenciados sem conflito.
- **Tentativa de acesso fora das raízes da biblioteca**: Caminhos manipulados com segmentos de retorno ao diretório pai, caracteres codificados ou links simbólicos apontando para fora de qualquer raiz configurada são sempre bloqueados com resposta de acesso negado ou não encontrado, sem revelar detalhes internos do sistema de arquivos.
- **Alteração externa dos arquivos no disco**: Se um vídeo listado for removido ou renomeado antes de o usuário clicar nele, o sistema exibe uma mensagem amigável de vídeo indisponível e permite atualizar a listagem sem queda do serviço.
- **Falha ao obter miniatura**: Se um vídeo estiver corrompido ou em formato sem prévia visual disponível, o cartão exibe uma miniatura padrão limpa sem atrasar ou bloquear o carregamento da listagem.
- **Múltiplos clientes simultâneos na LAN**: Vários dispositivos na rede local (ex.: TV, celular e computador) assistindo vídeos diferentes ou trechos distintos do mesmo vídeo simultaneamente não devem interferir entre si nem sobrescrever o progresso de reprodução uns dos outros.
- **Nomes com caracteres especiais**: Pastas, vídeos e arquivos de legenda com espaços, acentos, parênteses, colchetes, símbolos e Unicode são exibidos, associados e reproduzidos corretamente.
- **Interrupção de conexão ou fechamento do player**: Se o usuário fechar a aba, avançar rapidamente várias vezes ou perder sinal Wi-Fi momentaneamente, o serviço libera recursos normalmente sem travar para os demais usuários da rede.
- **Garantia estrita de somente-leitura**: Nenhuma operação da interface ou do serviço (incluindo miniaturas ou progresso de reprodução) cria arquivos temporários dentro das pastas de mídia, nem altera permissões, metadados, nomes ou conteúdo dos vídeos do usuário.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O sistema DEVE permitir ao operador iniciar o serviço configurando uma ou mais pastas raiz de mídia e o endereço/porta de escuta na rede local, oferecendo valores padrão simples quando omitidos.
- **FR-002**: O sistema DEVE validar na inicialização que todas as pastas raiz configuradas existem e podem ser lidas, encerrando com mensagem clara indicando o caminho inválido caso contrário.
- **FR-003**: O sistema DEVE listar as raízes configuradas (quando houver mais de uma), subpastas e arquivos de vídeo dentro da pasta solicitada, ocultando arquivos não relacionados a vídeo e arquivos ocultos do sistema por padrão.
- **FR-004**: O sistema DEVE permitir navegação hierárquica entre pastas raiz e subpastas com trilha de navegação (breadcrumbs) e retorno rápido a níveis anteriores.
- **FR-005**: O sistema DEVE exibir metadados básicos de cada item listado (nome legível, tamanho do arquivo, data de modificação, distinção visual clara entre pastas e vídeos) e miniatura visual de prévia para vídeos com fallback padrão quando indisponível.
- **FR-006**: O sistema DEVE transmitir o conteúdo do vídeo selecionado diretamente para reprodução no navegador web com suporte a início progressivo e salto imediato para qualquer ponto da linha do tempo (seek).
- **FR-007**: A interface do player web DEVE oferecer controles essenciais de reprodução (reproduzir, pausar, barra de progresso com tempo atual e total, volume, mudo, tela cheia e seletor de velocidade de reprodução) utilizáveis em navegadores de desktop, celular e TV.
- **FR-008**: O sistema DEVE operar estritamente em modo somente-leitura sobre todas as pastas de mídia configuradas, garantindo que nenhum arquivo ou diretório do usuário seja criado, modificado, renomeado, movido ou excluído (inclusive durante geração de miniaturas ou registro de progresso).
- **FR-009**: O sistema DEVE isolar todo acesso estritamente às pastas raiz configuradas, bloqueando qualquer tentativa de acesso a arquivos ou pastas externos (inclusive por manipulação de caminhos ou links simbólicos que apontem para fora das raízes autorizadas).
- **FR-010**: O sistema DEVE funcionar integralmente offline na rede local interna, servindo toda a interface web localmente sem realizar chamadas externas, carregar recursos de terceiros pela internet ou coletar telemetria.
- **FR-011**: O sistema DEVE suportar busca por nome de vídeo (insensível a maiúsculas/minúsculas) em toda a biblioteca configurada e ordenação de itens por nome ou data de modificação.
- **FR-012**: O sistema DEVE salvar localmente no dispositivo do espectador o ponto onde cada vídeo foi interrompido, exibir indicador de progresso na listagem e retomar a reprodução a partir desse ponto ao reabrir o vídeo (com opção de reiniciar do início e reinício automático após conclusão).
- **FR-013**: O sistema DEVE permitir avançar manualmente para o vídeo anterior ou próximo da mesma pasta a partir da tela do player e oferecer opção ativável/desativável de reprodução automática do próximo vídeo ao término do atual (com breve contagem regressiva cancelável).
- **FR-014**: O sistema DEVE detectar e disponibilizar legendas locais compatíveis presentes na mesma pasta com o mesmo nome base do vídeo (suportando múltiplos idiomas/rótulos quando presentes no nome do arquivo), permitindo ativá-las, alterná-las ou desativá-las no player.
- **FR-015**: O sistema DEVE oferecer seleção de faixa de áudio nos controles do player quando o vídeo e o navegador disponibilizarem múltiplas faixas de áudio, mantendo reprodução estável com a faixa padrão nos demais casos.
- **FR-016**: A interface web DEVE disponibilizar aviso de licença (GNU AGPL v3.0) e referência ao código-fonte correspondente de forma discreta e acessível aos usuários da rede.
- **FR-017**: O sistema DEVE exibir mensagens de erro amigáveis e recuperáveis quando um arquivo não for encontrado, uma pasta estiver vazia ou o formato do vídeo não for suportado pelo navegador do usuário.

### Key Entities *(include if feature involves data)*

- **Raiz de Biblioteca (Media Root)**: Diretório somente-leitura configurado pelo operador na inicialização, identificado por um rótulo amigável e identificador único, isolado do restante do sistema de arquivos.
- **Pasta (Folder)**: Diretório relativo a uma raiz de biblioteca, identificado por nome e caminho relativo seguro, agrupando subpastas e vídeos.
- **Item de Vídeo (Video Item)**: Arquivo de vídeo acessível em uma raiz, representado por identificador/caminho relativo seguro, título de exibição, tamanho, data de modificação, prévia visual (miniatura), tipo de mídia e legendas associadas.
- **Legenda Local (Local Subtitle)**: Arquivo de legenda localizado na mesma pasta de um vídeo com o mesmo nome base, podendo carregar rótulo/idioma derivado do sufixo do nome.
- **Progresso de Reprodução (Playback Progress)**: Registro local no dispositivo do espectador associando um vídeo ao último instante assistido, duração conhecida e estado de conclusão.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Um operador consegue iniciar o serviço apontando para uma ou mais pastas de vídeos e começar a assistir um vídeo em outro dispositivo na mesma rede local em menos de 1 minuto, sem etapas de cadastro ou configuração externa.
- **SC-002**: A navegação entre pastas e a abertura da tela inicial carregam em menos de 1 segundo em uma biblioteca local com até 1.000 arquivos de vídeo (sem que a preparação de miniaturas bloqueie a listagem).
- **SC-003**: A reprodução de um vídeo compatível inicia em menos de 2 segundos na rede local, e operações de avanço/retrocesso (seek) na linha do tempo retomam a reprodução em menos de 2 segundos sem baixar o arquivo inteiro antecipadamente.
- **SC-004**: Pelo menos 5 dispositivos simultâneos na mesma rede local conseguem navegar na biblioteca e reproduzir vídeos distintos (ou trechos distintos do mesmo vídeo) ao mesmo tempo sem quedas, travamentos ou conflito de progresso salvo entre dispositivos.
- **SC-005**: 100% das tentativas de acesso fora das pastas raiz configuradas (caminhos ascendentes ou links simbólicos externos) são bloqueadas, e 0 arquivos dentro das pastas de mídia sofrem qualquer escrita ou alteração durante o uso.
- **SC-006**: 100% das telas e funcionalidades da interface web operam corretamente em uma rede local totalmente desconectada da internet (0 requisições externas).
- **SC-007**: Ao fechar e reabrir um vídeo assistido parcialmente no mesmo navegador, o player retoma com precisão de até 5 segundos em relação ao último ponto reproduzido em 100% dos casos testados.
- **SC-008**: A interface é operável sem rolagem horizontal quebrada ou controles inacessíveis em telas de computador, smartphone e navegadores de TV na rede local.

## Assumptions

- **Acesso livre na rede local interna (sem PIN/senha na v1)**: Por definição explícita para a v1, o acesso é aberto a todos os dispositivos conectados à rede local interna onde o serviço está publicado; proteção opcional por PIN/senha ou contas de usuário fica fora do escopo da v1.
- **Reprodução direta sem transcodificação obrigatória**: A v1 prioriza transmissão direta dos vídeos compatíveis com o navegador do cliente para manter instalação enxuta em binário único sem dependências externas obrigatórias; formatos não suportados pelo navegador exibem orientação clara ao usuário.
- **Progresso por dispositivo**: Como não há contas de usuário na v1, o ponto de retomada de cada vídeo é armazenado localmente no navegador de cada dispositivo, mantendo privacidade e evitando conflitos entre pessoas diferentes da mesma casa assistindo na mesma LAN.
- **Miniaturas sem modificar a biblioteca**: Miniaturas são apresentadas sem nunca gravar arquivos nas pastas de mídia do usuário e degradam graciosamente para um cartão padrão quando não puderem ser geradas ou carregadas.
- **Troca de faixa de áudio condicionada ao suporte do navegador**: Em reprodução direta no navegador, a troca de faixa de áudio atua sobre as faixas que o navegador do cliente reconhece e expõe para o arquivo reproduzido.
- **Escopo enxuto (fora de escopo na v1)**: Não estão incluídos na v1 autenticação/PIN, perfis de usuário, download automático de capas/sinopses da internet, edição/exclusão de arquivos pela interface web, transcodificação em tempo real ou exposição pública fora da rede local.
