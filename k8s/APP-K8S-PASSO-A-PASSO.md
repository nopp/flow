# NoppFlow: Passo a passo para cadastrar uma App com deploy em Kubernetes

Este guia mostra o fluxo completo para sair do zero até o primeiro deploy funcionando.

## 1) Pré-requisitos

1. NoppFlow em execução e acessível no navegador.
2. Cluster Kubernetes acessível pelo contexto onde o NoppFlow roda.
3. Imagem de runner disponível e acessível pelo cluster (campo `k8s_runner_image`).
4. Repositório Git da app com:
   - `Dockerfile` (se houver build de imagem),
   - manifests (`k8s/*.yaml`) para modo `kubectl`, ou chart para modo `helm`.

## 2) Configurar o Kubernetes (obrigatório antes do cadastro da app)

Importante:
1. NoppFlow não usa kubeconfig de usuário final para deploy.
2. O deploy usa o contexto do cluster + ServiceAccount/RBAC.

### 2.1 Criar namespaces (exemplo)

```bash
kubectl create namespace noppflow --dry-run=client -o yaml | kubectl apply -f -
kubectl create namespace apps --dry-run=client -o yaml | kubectl apply -f -
```

### 2.2 Garantir RBAC do controlador NoppFlow

Isso permite que o pod do NoppFlow crie Jobs/Secrets temporários para `k8s_deploy`.

```bash
kubectl apply -f k8s/controller-serviceaccount.example.yaml
kubectl apply -f k8s/controller-rbac.namespace.example.yaml
```

Se necessário, ajuste no arquivo:
1. `metadata.namespace` (namespace de apps),
2. `subjects[].namespace` (namespace onde NoppFlow roda).

### 2.3 Criar ServiceAccount e RBAC do runner da app

Isso define as permissões de deploy que a app terá no namespace alvo.

```bash
kubectl apply -f k8s/runner-rbac.example.yaml
```

Por padrão o exemplo cria:
1. `ServiceAccount`: `noppflow-runner` no namespace `apps`,
2. `Role` + `RoleBinding`: permissões para deployments/services/configmaps/secrets/jobs etc.

### 2.4 Validar permissões

Valide pelo menos:

```bash
kubectl auth can-i create jobs -n apps --as=system:serviceaccount:noppflow:noppflow-controller
kubectl auth can-i create deployments -n apps --as=system:serviceaccount:apps:noppflow-runner
kubectl auth can-i patch deployments -n apps --as=system:serviceaccount:apps:noppflow-runner
```

Se retornar `no`, ajuste Role/RoleBinding antes de seguir.

## 3) Login e configurações iniciais no NoppFlow

1. Entre com um usuário `admin`.
2. Abra `Access`.
3. Cadastre uma SSH key:
   - `Name`: um nome claro (ex: `github-main`).
   - `Private key`: chave com acesso ao repo.
   - Para repo público HTTPS, pode usar uma chave “placeholder” se sua política permitir esse fluxo.
4. (Opcional) Cadastre variáveis globais em `Global Env Vars`:
   - Ex: `REGISTRY_ADDR`, `ENVIRONMENT`, `TEAM_NAME`.
   - Elas ficam disponíveis nos steps (`$NOME_DA_VARIAVEL`).

## 4) Criar a App (Apps -> Add app)

Preencha os campos:

1. **Name**: nome amigável da aplicação.
2. **Repository URL**: URL do git (`https://...` ou `git@...`).
3. **Branch**: branch de deploy (ex: `main`).
4. **SSH key**: selecione a chave criada no passo anterior.

### Deploy mode

Escolha conforme seu caso:

1. `kubectl`:
   - `Kubernetes namespace`: namespace alvo (ex: `apps`).
   - `Kubernetes service account`: SA usada no job runner (ex: `noppflow-runner`).
   - `Runner image`: imagem do runner (ex: `localhost:5001/noppflow-runner:latest`).
   - `Manifest path`: pasta/arquivo dos manifests (ex: `k8s/`).
2. `helm`:
   - Mesmo namespace/SA/runner.
   - `Helm chart`: caminho do chart no repo (ex: `charts/my-app`).
   - `Helm values path` (opcional): ex: `charts/my-app/values-prod.yaml`.

## 5) Definir os steps do pipeline

Recomendação de sequência:

1. `precheck` (`cmd`):
   - valida arquivos e contexto.
   - exemplo: `echo precheck && test -f Dockerfile`
2. `build_push` (`cmd`):
   - build e push da imagem.
   - exemplo com tag por run:
     - `IMAGE_TAG=run-$NOPPFLOW_RUN_ID`
     - `IMAGE_REF=<registry>/<app>:$IMAGE_TAG`
     - build/push com sua ferramenta (kaniko/buildah/docker).
3. `deploy_apply` (`k8s_deploy`):
   - step que executa deploy no cluster conforme `deploy_mode`.
4. `deploy_set_image` (`cmd`, opcional mas comum):
   - atualiza a imagem no deployment.
   - exemplo:
     - `kubectl -n apps set image deploy/minha-app minha-app=<registry>/minha-app:run-$NOPPFLOW_RUN_ID`
5. `deploy_rollout` (`cmd`):
   - espera rollout concluir.
   - exemplo:
     - `kubectl -n apps rollout status deploy/minha-app --timeout=180s`

Observações:

1. Cada step deve ter **apenas um tipo** (`cmd`, `file`, `script` ou `k8s_deploy`).
2. `sleep_sec` é opcional (0..3600).
3. Salve a app ao final.

## 6) Executar o primeiro deploy

1. Vá em `Apps`.
2. Clique em `Run` na app.
3. Vá em `Recent runs`.
4. Expanda a execução (seta na linha).
5. Acompanhe:
   - log ao vivo (auto-follow),
   - trilha de steps com estado (`waiting`, `running`, `success`, `failed`).

## 7) Validar no cluster

Comandos úteis:

```bash
kubectl -n <namespace> get deploy,po
kubectl -n <namespace> rollout status deploy/<nome-deploy>
kubectl -n <namespace> get events --sort-by=.metadata.creationTimestamp | tail -n 30
```

Se houve `set image`, valide a imagem final:

```bash
kubectl -n <namespace> get deploy <nome-deploy> -o jsonpath='{.spec.template.spec.containers[*].image}{"\n"}'
```

## 8) Troubleshooting rápido

### 1. `kubectl: not found` no runner

1. Confirme que a imagem `k8s_runner_image` é a correta.
2. Atualize para runner image mais nova do ambiente.
3. Verifique se o cluster consegue puxar essa imagem.

### 2. `selector does not match template labels`

1. Corrija manifest de Deployment.
2. `spec.selector.matchLabels` deve casar exatamente com `spec.template.metadata.labels`.

### 3. `ErrImagePull`

1. Verifique endereço de registry (push vs pull).
2. Confirme tag publicada.
3. Confirme acesso de rede do node ao registry.

### 4. Step finaliza e app não sobe

1. Rode `kubectl describe pod`.
2. Valide probes, env vars e imagem.
3. Valide namespace e service account.

### 5. Repo privado não clona

1. Confirme `ssh_key_name` da app.
2. Confirme chave com acesso ao repo/branch.
3. Teste URL de repo e branch.

## 9) Template mínimo de steps (kubectl)

```text
1) precheck (cmd)
echo precheck && test -f k8s/deployment.yaml

2) build_push (cmd)
IMAGE_TAG=run-$NOPPFLOW_RUN_ID
IMAGE_REF=<registry>/minha-app:$IMAGE_TAG
echo "build/push $IMAGE_REF"

3) deploy_apply (k8s_deploy)

4) deploy_set_image (cmd)
kubectl -n apps set image deploy/minha-app minha-app=<registry>/minha-app:run-$NOPPFLOW_RUN_ID

5) deploy_rollout (cmd)
kubectl -n apps rollout status deploy/minha-app --timeout=180s
```

## 10) Dica prática de adoção

1. Comece com repo de exemplo simples.
2. Feche pipeline base (`precheck` + `deploy_apply` + `rollout`).
3. Depois adicione build/push com versionamento por `NOPPFLOW_RUN_ID`.
