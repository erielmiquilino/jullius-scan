## Why

O backend ja possui API, worker, PostgreSQL e Redis como base funcional, mas ainda falta uma camada E2E dedicada para validar o fluxo real entre esses componentes sem depender apenas de testes unitarios ou de integracao isolados. Fazer isso agora cria uma base de confianca para evoluir scraping, filas e persistencia, ao mesmo tempo em que preserva uma separacao clara para uma futura suite E2E mobile em Flutter.

## What Changes

- Definir um projeto E2E de backend separado do backend principal, implementado com Playwright + TypeScript.
- Estabelecer uma arquitetura de testes que exercite a API HTTP, o processamento assincrono do worker, a persistencia em PostgreSQL e o fluxo de fila no Redis.
- Padronizar fixtures, ambiente, seed/reset de dados e estrategias de observabilidade para execucao local e futura execucao em CI.
- Organizar a raiz de testes automatizados para suportar, no futuro, uma camada independente de E2E mobile para Flutter sem misturar suites, dependencias ou comandos.

## Capabilities

### New Capabilities
- `backend-e2e-automation`: Plataforma E2E separada que valida o fluxo completo do backend entre API, worker, Redis e PostgreSQL usando Playwright + TypeScript.

### Modified Capabilities

- None.

## Impact

- Afeta a organizacao do repositorio para testes automatizados fora do backend principal.
- Introduz dependencias de Node.js, TypeScript e Playwright para a camada E2E de backend.
- Requer utilitarios de bootstrap, seed, reset e espera assincrona para validar estados persistidos e consumo de jobs.
- Prepara a base para futura adicao de uma suite E2E mobile em Flutter em estrutura paralela, sem compartilhar a mesma arvore de execucao.
