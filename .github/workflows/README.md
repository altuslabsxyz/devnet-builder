# GitHub Workflows

## Deploy Devnet Workflow

전문적인 DevOps 워크플로우로 stable 체인의 genesis를 export하고 devnet을 자동으로 구축 및 배포합니다.

### 핵심 동작 방식

1. **Stable Repository 연결**
   - `https://github.com/stablelabs/stable` 레포지토리를 자동으로 클론하거나 기존 클론을 찾습니다
   - stable-devnet의 태그와 동일한 버전을 체크아웃합니다
   - `stable-devnet/stable/` 경로에 심볼릭 링크로 연결합니다

2. **Genesis Export**
   - 연결된 stable 레포지토리에서 `stabled` 바이너리를 빌드합니다
   - 실행 중인 체인에서 genesis 상태를 export합니다

3. **Devnet 생성 및 배포**
   - Export된 genesis로 로컬 개발 네트워크를 생성합니다
   - 시스템에 배포하고 screen 세션으로 노드들을 실행합니다

### 워크플로우 개요

`deploy-devnet.yml`은 다음과 같은 작업을 수행합니다:

1. **시스템 검증**
   - stable.service systemd daemon 실행 확인
   - 노드 동기화 상태 확인 (catching_up == false)

2. **Stable Repository 설정**
   - stable-devnet 태그와 동일한 버전의 stable repository를 symbolic link로 설정
   - 자동으로 올바른 버전 검색 및 링크

3. **Genesis Export**
   - 실행 중인 체인에서 genesis 상태 export
   - stabled binary 빌드 후 export 실행

4. **Devnet 구축**
   - devnet-builder를 사용하여 로컬 개발 네트워크 생성
   - 다중 validator 및 account 설정

5. **배포 및 실행**
   - Artifact로 devnet 업로드
   - 시스템 디렉터리로 devnet 배포
   - Screen 세션으로 각 validator 노드 실행
   - 로그 파일 자동 생성

### 사용 방법

#### GitHub UI에서 실행

1. GitHub repository의 **Actions** 탭으로 이동
2. **Deploy Devnet** workflow 선택
3. **Run workflow** 버튼 클릭
4. 필요한 파라미터 입력
5. **Run workflow** 실행

#### GitHub CLI로 실행

```bash
gh workflow run deploy-devnet.yml \
  -f v_home=/data/.stable \
  -f daemon_name=stable.service \
  -f validators=4 \
  -f accounts=10
```

### 워크플로우 파라미터

| 파라미터 | 설명 | 필수 | 기본값 |
|---------|------|------|--------|
| `stable_tag` | Stable repository 태그 버전 | No | (현재 태그) |
| `v_home` | V_HOME 환경변수 경로 | Yes | `/data/.stable` |
| `daemon_name` | System daemon 이름 | Yes | `stable.service` |
| `validators` | 생성할 validator 수 | Yes | `4` |
| `accounts` | 생성할 dummy account 수 | Yes | `10` |
| `account_balance` | 각 account의 초기 잔액 | Yes | `1000000000000000000000astable,500000000000000000000agasusdt` |
| `validator_balance` | 각 validator의 초기 잔액 | Yes | `1000000000000000000000astable,500000000000000000000agasusdt` |
| `validator_stake` | 각 validator의 스테이킹 금액 | Yes | `100000000000000000000` |
| `chain_id` | Chain ID | No | `stabletestnet_2200-1` |
| `devnet_output_dir` | Devnet builder 출력 디렉터리 | Yes | `./devnet` |
| `devnet_base_dir` | Devnet 노드 배포 기본 디렉터리 | Yes | `/data/.devnet` |

### 전제 조건

#### Self-hosted Runner 설정

1. **Runner 태그**: `ubuntu`
2. **필수 소프트웨어**:
   - Git
   - Make
   - Go (stable 빌드용)
   - jq (JSON 파싱용)
   - curl
   - screen (노드 실행용)
   - systemd (daemon 관리용)

3. **실행 중인 서비스**:
   - stable.service systemd daemon이 실행 중이어야 함
   - RPC 엔드포인트 (localhost:26657) 접근 가능해야 함

4. **디렉터리 권한**:
   - Runner가 `/data/.devnet` (또는 지정된 경로)에 쓰기 권한 필요
   - Stable repository 경로에 읽기 권한 필요

#### Stable Repository 구조

워크플로우는 자동으로 https://github.com/stablelabs/stable 레포지토리를 처리합니다:

1. **자동 클론**: 기존 stable 레포지토리를 다음 위치에서 검색합니다:
   - `/data/stable`
   - `/data/.stable`
   - `$HOME/stable`
   - `$HOME/.stable`
   - `/opt/stable`
   - `/tmp/stable-cache`

2. **찾지 못한 경우**: `/tmp/stable-cache`에 자동으로 클론합니다.

3. **태그 체크아웃**: stable-devnet의 태그와 동일한 태그를 체크아웃합니다.

4. **심볼릭 링크 생성**: `stable-devnet/stable/` 경로에 심볼릭 링크를 생성합니다.

예: stable-devnet의 태그가 `v7.0.2-testnet`이면:
```
stable-devnet/
├── .github/
├── build/
├── stable/  -> /data/stable (v7.0.2-testnet 체크아웃됨)
```

### 워크플로우 출력

#### Artifacts

- **이름**: `devnet-{TAG}-{RUN_NUMBER}`
- **포함 내용**: 전체 devnet 디렉터리 구조
- **보관 기간**: 30일

#### 배포된 파일

```
/data/.devnet/
├── node0/
│   ├── config/
│   │   ├── genesis.json
│   │   └── priv_validator_key.json
│   ├── data/
│   └── keyring-test/
├── node1/
├── node2/
├── node3/
├── accounts/
├── node0.log
├── node1.log
├── node2.log
└── node3.log
```

#### Screen 세션

각 validator 노드는 독립적인 screen 세션으로 실행됩니다:

```bash
# 모든 screen 세션 확인
screen -list

# 특정 노드에 attach
screen -r node0

# Screen 세션에서 detach
Ctrl+A, D
```

#### 로그 파일

각 노드의 로그는 다음 위치에 저장됩니다:

```bash
# 실시간 로그 확인
tail -f /data/.devnet/node0.log

# 특정 노드 로그 확인
cat /data/.devnet/node1.log
```

### 문제 해결

#### 1. Daemon이 실행 중이 아님

**에러**: `Error: System daemon 'stable.service' is not running`

**해결**:
```bash
sudo systemctl start stable.service
sudo systemctl status stable.service
```

#### 2. 노드가 동기화되지 않음

**에러**: `Error: Node is not fully synced`

**해결**:
- 노드가 완전히 동기화될 때까지 대기
- 동기화 상태 확인:
```bash
curl -s http://localhost:26657/status | jq .result.sync_info.catching_up
```

#### 3. Stable repository 태그를 찾을 수 없음

**에러**: `Error: Tag {TAG} does not exist in stable repository`

**해결**:
- stable repository에 해당 태그가 존재하는지 확인:
```bash
git ls-remote --tags https://github.com/stablelabs/stable.git | grep {TAG}
```
- 또는 로컬에 클론된 stable 레포지토리에서:
```bash
cd /data/stable
git fetch --tags
git tag | grep {TAG}
```
- 태그가 존재하지 않으면 stable-devnet의 태그 이름을 확인하거나 `stable_tag` 파라미터로 올바른 태그를 지정

#### 4. Screen 세션이 시작되지 않음

**해결**:
```bash
# Screen이 설치되어 있는지 확인
which screen

# 수동으로 노드 시작
cd /path/to/stable
./build/stabled start --home /data/.devnet/node0 --chain-id stabletestnet_2200-1
```

### 보안 고려사항

1. **Self-hosted Runner 보안**
   - Runner는 신뢰할 수 있는 환경에서 실행
   - Private repository에서만 사용 권장

2. **권한 관리**
   - Runner 사용자에게 최소 권한 부여
   - sudo 권한이 필요한 작업 최소화

3. **백업**
   - 기존 devnet은 자동으로 백업됨
   - 백업 위치: `{DEVNET_BASE_DIR}_backup_{TIMESTAMP}`

### 고급 사용법

#### 커스텀 Balance 설정

```bash
gh workflow run deploy-devnet.yml \
  -f account_balance="5000000000000000000000astable,1000000000000000000000agasusdt,2000000000000000000000ausdc" \
  -f validator_balance="10000000000000000000000astable,5000000000000000000000agasusdt"
```

#### 대규모 Validator 네트워크

```bash
gh workflow run deploy-devnet.yml \
  -f validators=10 \
  -f accounts=50 \
  -f validator_stake="500000000000000000000"
```

#### 특정 태그로 배포

```bash
gh workflow run deploy-devnet.yml \
  -f stable_tag=v7.0.2-testnet \
  -f v_home=/data/.stable
```

### 모니터링

#### 워크플로우 진행 상황

```bash
# 최근 워크플로우 실행 확인
gh run list --workflow=deploy-devnet.yml

# 특정 실행의 로그 확인
gh run view {RUN_ID} --log
```

#### 노드 상태 확인

```bash
# 모든 노드의 상태 확인
for i in {0..3}; do
  echo "Node $i:"
  curl -s http://localhost:$((26657 + i))/status | jq .result.sync_info
done
```

### 추가 리소스

- [Stable Documentation](https://docs.stable.network)
- [GitHub Actions Documentation](https://docs.github.com/actions)
- [Self-hosted Runners Guide](https://docs.github.com/actions/hosting-your-own-runners)
