# ADR-0006: Gestão de segredos em Kubernetes (decisão pendente)

## Status
Proposta — decisão ainda não tomada

## Contexto
`k8s/secret.yaml` é um manifest `Secret` do tipo `Opaque` com `stringData`
versionado em texto plano no Git (hoje com valores placeholder
`seu_token_aqui` / `seu_pipe_id_aqui`, mas no formato exato que seria usado
com valores reais). Não há Sealed Secrets, External Secrets Operator, Vault,
nem qualquer mecanismo de secret management externo referenciado no repositório.

## Opções em avaliação
1. **Manter `Secret` nativo do k8s, mas nunca commitado** — mover
   `secret.yaml` para fora do controle de versão (gerado via CI/CD ou
   `kubectl create secret` manual), documentando o processo no README.
2. **Sealed Secrets (Bitnami)** — permite commitar o `SealedSecret`
   criptografado no Git; simples de adotar em cluster único.
3. **External Secrets Operator + AWS Secrets Manager** — alinhado com a
   "Visão de Produção (AWS)" já descrita no README (`PIPEFY_API_TOKEN` no
   Secrets Manager); maior consistência entre ambiente de produção real e
   manifests do repositório.

## Decisão
Não tomada nesta auditoria. Recomendação: opção 3, por já estar alinhada à
visão de produção documentada no README — mas isso é uma escolha do time, não
uma correção técnica objetiva como os demais achados do relatório de
auditoria.

## Consequências
Enquanto esta ADR permanecer "Proposta", `k8s/secret.yaml` deve ser tratado
como **documentação de formato**, nunca como fonte de segredos reais — o
risco concreto é um desenvolvedor substituir os placeholders por valores reais
e commitar por hábito, já que hoje nada no repositório (nem `.gitignore`, nem
CI) impede isso.

## Atualização (2026-09-07)
O risco imediato foi mitigado, mas a decisão de longo prazo (opção 1, 2 ou 3
acima) continua em aberto: o arquivo foi renomeado para
`k8s/secret.example.yaml` (documentação de formato, com instruções de
`kubectl create secret` no próprio comentário) e `k8s/secret.yaml` — o nome
que conteria valores reais — foi adicionado ao `.gitignore`. Isso resolve "um
segredo real pode ser commitado sem querer" sem resolver "como o time
efetivamente entrega segredos ao cluster em produção", que segue sendo o
escopo desta ADR.
