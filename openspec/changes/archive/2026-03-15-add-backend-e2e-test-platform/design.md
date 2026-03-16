## Context

O projeto possui um backend em Go separado entre API e worker, com PostgreSQL para persistencia e Redis para fila. A lacuna atual esta na verificacao do fluxo completo: submissao HTTP, criacao de job, consumo pelo worker, persistencia relacional e exposicao final do resultado.

Para que a suite seja deterministica, o ambiente E2E precisa simular a origem das paginas da SEFAZ com um mock server local simples servindo HTML estatico. Ao mesmo tempo, a autenticacao nao deve ser simplificada com bypass no produto: os testes precisam obter um JWT real em um projeto Firebase dedicado para testes e enviar esse token para a API exatamente como um cliente real faria.

Como a stack E2E proposta usa Playwright + TypeScript, ela introduz um ecossistema diferente do backend principal. Isso pede uma estrutura isolada para nao contaminar o modulo Go com dependencias Node, scripts de Playwright e convencoes de automacao que nao pertencem ao runtime de producao. Ao mesmo tempo, a estrutura precisa deixar um caminho claro para uma futura camada E2E mobile em Flutter, sem compartilhar a mesma arvore de specs, fixtures ou comandos.

Restricoes e interessados principais:
- backend/API Go que permanece a aplicacao sob teste
- worker Go que precisa ser exercitado de forma assincrona e observavel
- PostgreSQL e Redis que precisam ser inspecionados como parte da verificacao E2E
- pipeline futura de CI que deve conseguir subir o ambiente, rodar a suite e coletar artefatos
- futura suite mobile Flutter que deve coexistir em paralelo, mas sem acoplamento operacional agora
- projeto Firebase dedicado para testes E2E, acessado por credenciais injetadas via ambiente
- mock server local da SEFAZ para servir fixtures HTML estaticas ao worker durante os cenarios deterministas

## Goals / Non-Goals

**Goals:**
- criar uma plataforma E2E de backend separada do modulo principal, usando Playwright + TypeScript
- validar fluxos reais entre API, worker, Redis e PostgreSQL em ambiente controlado e reproduzivel
- definir convencoes de estrutura, fixtures, seeds, reset e polling para testes assincronos
- preparar uma raiz de automacao que permita adicionar uma suite E2E mobile no futuro sem misturar dependencias ou suites
- tornar a execucao local e futura execucao em CI previsiveis, com logs e artefatos de falha
- manter o contrato real de autenticacao Firebase da API em Go sem introduzir bypass de teste

**Non-Goals:**
- implementar agora a suite E2E mobile para Flutter
- usar Playwright para dirigir interfaces web do produto, ja que o foco desta mudanca eh backend
- validar a SEFAZ publica em ambiente externo como dependencia obrigatoria dos testes deterministas
- substituir testes unitarios ou de integracao ja existentes no backend
- redefinir contratos funcionais da API ou do worker alem do necessario para suportar testabilidade E2E
- modificar o fluxo de autenticacao do produto para acomodar bypass ou headers especiais de teste

## Decisions

### Projeto Node separado para automacao E2E de backend
Decisao: criar uma arvore dedicada para automacao E2E de backend fora de `backend/`, com `package.json`, `tsconfig`, configuracao do Playwright e utilitarios proprios.

Racional:
- evita misturar dependencias Node com o modulo Go principal
- deixa claro que se trata de infraestrutura de teste, nao de runtime da aplicacao
- facilita criar futuramente uma raiz paralela para `mobile-e2e` sem colisao de scripts ou fixtures

Alternativas consideradas:
- colocar Playwright dentro de `backend/`: descartado por aumentar acoplamento entre runtime Go e tooling Node.
- criar uma unica pasta `e2e/` com backend e mobile juntos desde ja: descartado para evitar mistura prematura de suites com stacks diferentes.

### Estrutura por dominio de suite, nao por tecnologia compartilhada
Decisao: organizar a automacao em camadas separadas por dominio de validacao, com um espaco dedicado para backend agora e uma reserva explicita para mobile no futuro, por exemplo sob uma raiz como `testing/e2e/backend` e `testing/e2e/mobile`.

Racional:
- torna o limite entre suites evidente no repositorio
- permite comandos, dependencias e pipelines independentes por suite
- evita que fixtures de backend virem dependencia incidental da futura automacao Flutter

Alternativas consideradas:
- compartilhar a mesma pasta de `fixtures` entre backend e mobile desde o inicio: descartado por criar acoplamento antes de existir necessidade real.

### Playwright como orquestrador HTTP e de observabilidade, nao como browser da aplicacao
Decisao: usar Playwright principalmente com `APIRequestContext`, fixtures TypeScript, hooks de ambiente e artefatos de execucao. O browser do Playwright nao sera o foco principal da suite de backend, embora continue disponivel para paginas auxiliares ou diagnstico se necessario.

Racional:
- Playwright oferece runner robusto, retries, traces, reporters e boa ergonomia para testes assincronos
- `APIRequestContext` atende bem o papel de cliente HTTP autenticado para a API
- mantem a stack de automacao simples sem introduzir um framework adicional so para API testing

Alternativas consideradas:
- usar apenas Jest/Vitest com fetch: descartado por perder a integracao nativa com traces, fixtures e reporter do Playwright.
- usar Postman/Newman: descartado por menor flexibilidade para coordenar polling, reset de banco e verificacoes cruzadas.

### Ambiente E2E sobe a stack real com mock server local para a SEFAZ
Decisao: a suite deve subir API, worker, PostgreSQL, Redis e um mock server simples da SEFAZ em compose de teste, usando configuracao dedicada. O mock server pode ser baseado em Nginx ou em um script Node minimo, e deve expor o fixture `nfce-consulta-detalhada.html` a partir de um diretorio do novo workspace E2E, por exemplo `mocks/sefaz/`. O worker em Go acessara essa URL local durante os testes deterministas.

Racional:
- garante repetibilidade e estabilidade da suite
- mantem o foco no comportamento do sistema interno que queremos validar
- reduz falsos negativos causados por rede externa, bloqueios ou variacoes de terceiros
- permite validar o fluxo real de scraping do worker sem depender da SEFAZ publica

Alternativas consideradas:
- rodar contra servicos compartilhados ja existentes no ambiente de desenvolvimento: descartado por baixa isolacao e risco de dados sujos.
- chamar a SEFAZ real em cada execucao: descartado por nao ser confiavel para CI e por introduzir limite externo fora do controle do projeto.

### Autenticacao real via Firebase REST API sem alterar o produto
Decisao: a suite E2E nao deve introduzir bypass de autenticacao nem alterar o codigo da API em Go. Os testes devem autenticar contra um projeto Firebase real dedicado para testes usando a REST API do Firebase, obter um JWT real e enviar esse token para a API. As credenciais devem entrar via variaveis de ambiente, incluindo `FIREBASE_API_KEY`, `FIREBASE_TEST_USER_EMAIL`, `FIREBASE_TEST_USER_PASSWORD` e `FIREBASE_UUID`.

Racional:
- preserva o contrato real de autenticacao usado pela aplicacao em producao
- evita que a suite valide um caminho de auth artificial diferente do comportamento real
- mantem a responsabilidade de automacao no workspace E2E, sem introduzir codigo de teste no produto

Alternativas consideradas:
- bypass por header de teste ou flag na API: descartado por alterar o produto e reduzir a confianca do fluxo autenticado.
- emulador ou stub de identidade local: descartado porque a decisao do time eh validar contra Firebase real dedicado.

### Reset de dados por teste e seeds explicitos com acesso direto a banco e fila
Decisao: cada spec E2E deve depender de um mecanismo claro de bootstrap, seed e cleanup que restaure PostgreSQL e Redis para um estado conhecido antes da execucao ou por grupo de testes, com fixtures utilitarias para usuarios, casas, jobs e recibos. Para isso, o workspace E2E deve usar clientes diretos para PostgreSQL e Redis no TypeScript, viabilizando reset, seed e inspecao autonoma de estado.

Racional:
- evita acoplamento entre cenarios
- melhora depuracao ao tornar o estado inicial previsivel
- facilita paralelizacao futura ou execucao seletiva de suites
- reduz dependencia de utilitarios ad-hoc externos ao runner para preparar estado

Alternativas consideradas:
- reaproveitar um banco long-lived entre execucoes sem reset completo: descartado por gerar flakiness e dependencia entre testes.

### Polling deterministico para fluxos assincronos
Decisao: os testes devem encapsular espera assincrona em helpers que consultam API e, quando necessario, banco de dados, com timeout total e mensagens de erro diagnsticas. A suite nao deve usar sleeps arbitrarios como mecanismo principal de sincronizacao.

Racional:
- o worker processa jobs fora do ciclo HTTP, entao a verificacao precisa respeitar o comportamento eventual do sistema
- helpers centralizados reduzem flakiness e deixam o motivo da falha mais claro

Alternativas consideradas:
- usar apenas `setTimeout` fixo entre passos: descartado por lentidao e baixa confiabilidade.

### Contrato de fronteira para futura suite mobile
Decisao: documentar desde agora que a futura suite mobile tera workspace, comandos, fixtures e pipeline proprios, reaproveitando no maximo utilitarios neutros de ambiente em uma camada compartilhada deliberada e minima.

Racional:
- preserva independencia da futura automacao Flutter
- evita transformar a suite de backend em dependencia estrutural do mobile
- permite evoluir cadencias diferentes para backend e mobile

Alternativas consideradas:
- compartilhar tudo sob uma unica configuracao de Playwright: descartado porque Flutter E2E provavelmente exigira runner, emuladores e artefatos diferentes.

## Risks / Trade-offs

- [Suite E2E ficar lenta por subir varios servicos] -> Mitigacao: separar smoke e full suite, reutilizar ambiente por job de teste quando seguro e manter seeds enxutas.
- [Flakiness em fluxos assincronos] -> Mitigacao: usar polling centralizado com timeouts claros, sinais de prontidao e asserts de diagnostico em API/banco.
- [Dependencia demais de doubles reduzir confianca no scraping real] -> Mitigacao: manter a suite E2E de backend focada no sistema interno e deixar verificacoes contra ambientes reais para testes complementares, nao para a suite deterministica principal.
- [Dependencia de um projeto Firebase real para autenticacao] -> Mitigacao: usar um projeto dedicado de testes, injetar credenciais por ambiente e manter usuarios/claims previsiveis para a suite.
- [Mistura futura entre backend e mobile por conveniencia] -> Mitigacao: codificar a separacao em estrutura, scripts e documentacao desde a primeira versao.
- [Playwright ser usado so para API parecer excesso] -> Mitigacao: aproveitar reporter, traces, fixtures e integracao CI para justificar a padronizacao numa unica stack de automacao.

## Migration Plan

1. Criar a raiz dedicada da automacao E2E de backend com configuracao Node, TypeScript e Playwright.
2. Adicionar ao ambiente de teste isolado os servicos de PostgreSQL, Redis, API, worker e um mock server simples da SEFAZ que exponha o fixture `mocks/sefaz/nfce-consulta-detalhada.html`.
3. Mover o fixture HTML ja existente na raiz do repositorio para o workspace E2E e referenciar sua URL local nos cenarios de scraping.
4. Instalar os clientes TypeScript para acesso direto a PostgreSQL e Redis e implementar fixtures de bootstrap, autenticacao Firebase real, seed/reset de banco e limpeza de Redis.
5. Criar os primeiros fluxos E2E cobrindo submissao de recibo, consumo do job, persistencia e consulta final do resultado.
6. Configurar reporters, traces e comandos de execucao local/CI.
7. Documentar a fronteira arquitetural reservando uma raiz paralela para futura automacao mobile.

Rollback:
- remover a raiz de automacao E2E se a stack escolhida nao se mostrar adequada
- desligar os artefatos de CI associados sem impactar runtime de producao
- manter qualquer ajuste de testabilidade no backend protegido por configuracao e compativel com execucao normal

## Resolved Questions

- A autenticacao E2E sera feita contra um projeto Firebase real dedicado para testes, usando a REST API do Firebase e variaveis de ambiente (`FIREBASE_API_KEY`, `FIREBASE_TEST_USER_EMAIL`, `FIREBASE_TEST_USER_PASSWORD`, `FIREBASE_UUID`). Nao havera bypass nem alteracao do codigo da API em Go.
- O ambiente E2E deve subir em containers para alinhar com a topologia real e incluir explicitamente um mock server local da SEFAZ servindo o fixture HTML estatico a partir do workspace E2E.
