## Context

Jullius Scan sera composto por um cliente mobile em Flutter e um backend em Go exposto via API REST. O principal caso de uso do MVP eh capturar um QR Code de NFC-e, enviar a URL publica da SEFAZ para o backend e acompanhar a extracao de dados sem bloquear a experiencia mobile.

Nesta fase, o projeto e estritamente para uso pessoal e aprendizado com Go e Flutter. Isso muda algumas prioridades arquiteturais: nao ha necessidade imediata de publicacao em lojas, polimento avancado de UX, escalabilidade sofisticada ou mitigacoes caras para todas as falhas externas. O foco e construir uma fundacao limpa, operavel e suficiente para uso real em ambiente controlado.

O dominio exige tratamento de paginas publicas da SEFAZ que podem depender de JavaScript, o que inviabiliza uma estrategia baseada apenas em requests HTTP simples. A arquitetura tambem precisa separar autenticacao, orquestracao de scraping, persistencia normalizada e deploy em VPS com operacao simples.

Stakeholders principais:
- usuarios mobile autenticados via Firebase Auth
- backend/API responsavel por autorizacao, ingestao e consulta
- worker de scraping responsavel por navegacao `chromedp`, extracao e normalizacao
- operacao em VPS com Docker Compose, Traefik e pipeline GitHub Actions

## Goals / Non-Goals

**Goals:**
- definir uma base arquitetural enxuta para um MVP pessoal entre app Flutter, API Go, jobs assincronos e PostgreSQL
- padronizar o fluxo de scraping assincrono desde o QR Code ate a persistencia do cupom
- estabelecer contratos iniciais de autenticacao, estados de processamento e ownership por Casa
- documentar os componentes de infraestrutura e entrega continua na VPS reaproveitando o registry ja existente

**Non-Goals:**
- detalhar UX final do app Flutter ou fluxos completos de onboarding
- publicar o aplicativo em lojas ou suportar distribuicao publica neste momento
- cobrir leitura de camera nativa neste momento; o foco atual eh receber e processar a URL do QR Code
- definir observabilidade completa, rate limits avancados, antifraude ou resiliencia enterprise
- contornar bloqueios de Captcha da SEFAZ com servicos de terceiros no MVP
- implementar suporte multi-tenant, multi-regiao ou orquestracao em Kubernetes

## Decisions

### Flutter como cliente de borda para captura e consulta
Decisao: o Flutter fica responsavel por autenticacao via Firebase, leitura futura de camera/QR Code, submissao da URL do documento e exibicao do status/resultado do recibo.

Racional:
- preserva experiencia mobile e reaproveita SDKs maduros de Firebase Auth
- reduz acoplamento do cliente com a logica de scraping
- prepara o app para futuras capacidades de camera sem alterar o backend

Alternativas consideradas:
- Web app responsivo: descartado por nao priorizar captura mobile nativa.
- Backend renderizando paginas para usuario final: descartado por misturar scraping com apresentacao.

### API Go separada de workers assincromos
Decisao: a API REST em Go recebe requisicoes autenticadas, cria um job de scraping e retorna imediatamente um identificador/status inicial. A execucao do scraping ocorre fora do ciclo HTTP, em worker dedicado ou processo backend separado, usando Redis como fila e compartilhando banco e contrato de estados.

Racional:
- evita timeouts causados por paginas lentas ou bloqueios da SEFAZ
- melhora resiliencia e permite retentativas controladas
- simplifica escalabilidade futura ao separar API e processamento pesado

Alternativas consideradas:
- scraping sincrono dentro do handler HTTP: descartado por latencia alta e fragilidade operacional.
- polling interno no PostgreSQL: descartado para o MVP apos decisao de usar Redis em container dedicado na VPS com rede interna compartilhada.

### Timeouts e encerramento forcado no worker `chromedp`
Decisao: cada execucao de scraping deve rodar com timeouts explicitos em Go, incluindo timeout total de 45 segundos por job e limites internos de navegacao/renderizacao. Ao exceder esses limites ou ocorrer falha, o contexto do `chromedp` deve ser cancelado e o processo do Chrome/Chromium encerrado para evitar processos zumbis e consumo excessivo de RAM.

Racional:
- protege a VPS contra vazamento de memoria e processos presos
- torna o comportamento do worker previsivel mesmo diante de paginas lentas ou quebradas
- reduz risco operacional sem depender de intervencao manual

Alternativas consideradas:
- deixar apenas timeout no request HTTP: descartado por nao garantir encerramento do browser.
- aceitar processos orfaos e reciclar container periodicamente: descartado por mascarar falhas e desperdiçar recursos.

### `chromedp` como motor principal de scraping
Decisao: o worker de scraping deve usar `chromedp` com Chrome/Chromium em container para navegar ate a pagina publica da SEFAZ, aguardar carregamento relevante, extrair HTML final e transformar o conteudo em entidades normalizadas.

Racional:
- suporta paginas JS-heavy e interacoes necessarias antes da extracao
- mantem o stack de scraping no mesmo ecossistema do backend Go
- reduz discrepancias entre ambiente local e deploy quando empacotado via Docker

Alternativas consideradas:
- requests HTTP + parser HTML: insuficiente para paginas com renderizacao dinamica.
- Playwright em servico separado: viavel, mas adiciona stack extra e custo operacional agora.

### Modelo de dados com Casa compartilhada
Decisao: usar PostgreSQL com tabelas normalizadas `users`, `houses`, `house_members`, `receipts`, `items`, `stores` e uma tabela de controle de jobs de scraping. O isolamento principal deixa de ser por usuario e passa a ser por Casa, permitindo que membros compartilhem notas e evitando duplicidade quando pessoas da mesma familia escaneiam a mesma nota fisica.

Racional:
- separa claramente dominio fiscal do pipeline operacional
- permite compartilhamento simples para uso familiar sem exigir um modelo multi-tenant complexo
- facilita deduplicacao por chave/URL fiscal dentro da mesma Casa
- permite integridade relacional entre cupom, estabelecimento e itens

Alternativas consideradas:
- recibo pertencer apenas ao usuario que escaneou: descartado por criar duplicidade e atrito no uso familiar.
- armazenar tudo em JSON unico: descartado por dificultar consulta e evolucao analitica.
- banco NoSQL como principal: descartado por menor aderencia ao modelo relacional do dominio inicial.

### Provisionamento manual do MVP para Casa e usuarios
Decisao: o MVP nao tera fluxo de criacao automatica de Casa nem convite de membros. Usuarios, Casa inicial e relacionamentos basicos serao cadastrados manualmente no Firebase e no banco de dados nesta fase.

Racional:
- reduz escopo para manter o foco em scraping, API e aprendizado com Go/Flutter
- evita investir tempo agora em fluxos administrativos que nao sao essenciais para uso pessoal
- permite validar o dominio compartilhado antes de construir UX de gerenciamento

Alternativas consideradas:
- criar Casa automaticamente no primeiro login: adiado para reduzir comportamento implicito no MVP.
- implementar convite por e-mail, codigo ou UID: adiado por nao ser essencial nesta fase.

### Firebase Auth validado no backend Go
Decisao: o app Flutter autentica usuarios com Firebase Auth e envia JWT Bearer para a API. O backend Go valida assinatura, issuer, audience e subject antes de autorizar a operacao e resolver o contexto de Casa do usuario autenticado.

Racional:
- reduz esforco de construir auth proprietaria
- permite identidade consistente entre app e API
- assegura que cada job seja executado no contexto de uma Casa valida e auditavel

Alternativas consideradas:
- sessao propria no backend: descartada por retrabalho e maior superficie de seguranca.
- backend confiando apenas no UID enviado pelo cliente: descartado por inseguranca.

### Deploy em VPS com Docker Compose, Traefik e registry existente
Decisao: a plataforma roda em uma VPS com servicos conteinerizados via Docker Compose. O Traefik atua como proxy reverso usando labels para roteamento. O GitHub Actions constroi e publica imagens para o registry ja existente em `https://registry.skadi.digital/`, e o deploy acessa a VPS via SSH com chave `.pem` para atualizar a stack.

Racional:
- combina baixo custo, controle operacional e simplicidade de operacao inicial
- centraliza terminacao HTTP/HTTPS e roteamento em um unico componente
- reutiliza uma infraestrutura ja disponivel e com SSL ativo

Alternativas consideradas:
- Kubernetes: descartado por complexidade desnecessaria nesta fase.
- deploy manual com binarios soltos: descartado por baixa repetibilidade e rollback ruim.

### Persistencia enxuta de dados fiscais e diagnostico
Decisao: na primeira versao, o sistema persiste apenas loja, itens, totais, metadados do recibo, relacoes de Casa e logs estruturados. O HTML bruto da SEFAZ nao sera armazenado.

Racional:
- reduz volume de armazenamento e complexidade de sanitizacao
- mantem o foco nos dados realmente usados no MVP
- simplifica requisitos de privacidade, debug e manutencao operacional

Alternativas consideradas:
- armazenar HTML bruto para auditoria: adiado para evitar custo operacional adicional sem necessidade imediata.

## Risks / Trade-offs

- [Paginas da SEFAZ mudarem estrutura ou fluxo JS] -> Mitigacao: isolar seletores/regras de extracao, registrar logs estruturados e suportar ajuste/versionamento do parser.
- [`chromedp` consumir muita memoria ou CPU na VPS] -> Mitigacao: limitar concorrencia de workers, aplicar timeouts explicitos por job/navegacao e garantir cancelamento do contexto com encerramento do browser.
- [Fila Redis indisponivel ou mal configurada] -> Mitigacao: executar Redis em container interno dedicado, monitorar conectividade entre servicos e manter configuracao por ambiente.
- [Duplicacao de recibos por reenvio do mesmo QR Code] -> Mitigacao: aplicar chave de idempotencia por URL/chave fiscal no escopo da Casa e reaproveitar o mesmo recibo para todos os membros.
- [Falhas na validacao de JWT bloquearem uso legitimo] -> Mitigacao: padronizar configuracao de issuer/audience por ambiente e registrar motivos de rejeicao.
- [Captcha da SEFAZ bloquear a extracao] -> Mitigacao: risco aceito no MVP; registrar o motivo da falha e permitir nova tentativa manual sem integrar servicos de terceiros agora.
- [Acoplamento ao registry e acesso SSH atuais] -> Mitigacao: manter imagens versionadas, estrategia clara de rollback e parametrizacao do pipeline para futura troca de endpoint ou credenciais.

## Migration Plan

1. Provisionar a estrutura base da VPS com Docker, Docker Compose, Traefik e acesso autenticado ao registry existente.
2. Criar servicos separados para API Go, worker de scraping, PostgreSQL e Redis na composicao inicial, sendo o Redis criado manualmente na VPS com rede interna compartilhada.
3. Inserir manualmente no Firebase e no banco os usuarios, a Casa inicial e os dados basicos necessarios para uso do MVP.
4. Implementar validacao de JWT do Firebase, resolucao da Casa do usuario e endpoints base de submissao/consulta de jobs.
5. Integrar API e worker com Redis para enfileiramento e consumo dos jobs de scraping.
6. Configurar pipeline GitHub Actions para build, push em `https://registry.skadi.digital/` e deploy remoto por SSH com chave `.pem`.
7. Liberar o fluxo inicial para QR Code -> job -> scraping -> persistencia -> consulta de resultado.

Rollback:
- reverter imagens para a tag anterior no Docker Compose
- manter migrations compatveis com rollback ou expansao segura nas primeiras entregas
- pausar workers caso haja problema em scraping sem derrubar a API de consulta

## Open Questions

- Nao ha open questions bloqueantes para o MVP neste artefato; as decisoes atuais sao: Redis como fila, provisionamento manual de Casa/usuarios, persistencia apenas de campos extraidos e logs, timeout de 45 segundos por job e no maximo 3 retentativas.
