# Deploy Devnet Workflow - 사용 예제

## 기본 사용 예제

### 1. 표준 4-Validator Devnet 배포

가장 기본적인 설정으로 4개의 validator와 10개의 account를 가진 devnet을 배포합니다.

```bash
gh workflow run deploy-devnet.yml \
  -f v_home=/data/.stable \
  -f daemon_name=stable.service \
  -f validators=4 \
  -f accounts=10
```

**시나리오**: 로컬 개발 및 테스트를 위한 기본 환경

### 2. 소규모 테스트 네트워크

빠른 테스트를 위한 최소 구성의 devnet입니다.

```bash
gh workflow run deploy-devnet.yml \
  -f v_home=/data/.stable \
  -f daemon_name=stable.service \
  -f validators=1 \
  -f accounts=5 \
  -f account_balance="100000000000000000000astable" \
  -f validator_balance="500000000000000000000astable" \
  -f validator_stake="50000000000000000000"
```

**시나리오**:
- 단일 validator로 빠른 프로토타입 테스트
- 최소 리소스 사용
- 간단한 기능 검증

### 3. 대규모 Validator 네트워크

프로덕션과 유사한 환경을 시뮬레이션하는 대규모 네트워크입니다.

```bash
gh workflow run deploy-devnet.yml \
  -f v_home=/data/.stable \
  -f daemon_name=stable.service \
  -f validators=10 \
  -f accounts=100 \
  -f validator_stake="1000000000000000000000"
```

**시나리오**:
- 합의 알고리즘 테스트
- 네트워크 성능 벤치마킹
- 스케일 테스트

### 4. 멀티-토큰 경제 테스트

여러 토큰을 사용하는 복잡한 경제 모델을 테스트합니다.

```bash
gh workflow run deploy-devnet.yml \
  -f v_home=/data/.stable \
  -f daemon_name=stable.service \
  -f validators=4 \
  -f accounts=20 \
  -f account_balance="5000000000000000000000astable,2000000000000000000000agasusdt,1000000000000000000000ausdc,500000000000000000000aeth" \
  -f validator_balance="10000000000000000000000astable,5000000000000000000000agasusdt,3000000000000000000000ausdc,1000000000000000000000aeth"
```

**시나리오**:
- DEX 테스트
- 크로스 토큰 거래
- 유동성 풀 테스트

### 5. 특정 버전 테스트

특정 stable 버전으로 devnet을 배포합니다.

```bash
gh workflow run deploy-devnet.yml \
  -f stable_tag=v7.0.2-testnet \
  -f v_home=/data/.stable \
  -f daemon_name=stable.service \
  -f validators=4 \
  -f accounts=10
```

**시나리오**:
- 특정 버전의 버그 재현
- 버전 간 마이그레이션 테스트
- 릴리스 검증

### 6. 커스텀 Chain ID 및 디렉터리

커스텀 설정으로 devnet을 배포합니다.

```bash
gh workflow run deploy-devnet.yml \
  -f v_home=/data/.stable \
  -f daemon_name=stable.service \
  -f validators=4 \
  -f accounts=10 \
  -f chain_id=stable-local-1 \
  -f devnet_output_dir=./my-devnet \
  -f devnet_base_dir=/opt/stable-devnet
```

**시나리오**:
- 여러 devnet 동시 실행
- 커스텀 네트워크 설정
- 조직별 naming convention

## 고급 사용 예제

### 7. 고성능 Validator 네트워크

높은 스테이킹으로 안정적인 네트워크를 구성합니다.

```bash
gh workflow run deploy-devnet.yml \
  -f v_home=/data/.stable \
  -f daemon_name=stable.service \
  -f validators=7 \
  -f accounts=50 \
  -f validator_balance="50000000000000000000000astable,10000000000000000000000agasusdt" \
  -f validator_stake="5000000000000000000000" \
  -f chain_id=stablenet-perf-test
```

**용도**:
- 고부하 트랜잭션 테스트
- 합의 성능 측정
- 네트워크 안정성 검증

**예상 리소스**:
- CPU: 높음 (7개 노드 동시 실행)
- 메모리: 중-높음
- 디스크: 중간

### 8. 개발자 워크스테이션 설정

로컬 개발을 위한 경량 설정입니다.

```bash
gh workflow run deploy-devnet.yml \
  -f v_home=$HOME/.stable \
  -f daemon_name=stable.service \
  -f validators=2 \
  -f accounts=10 \
  -f account_balance="1000000000000000000000astable" \
  -f validator_balance="5000000000000000000000astable" \
  -f validator_stake="100000000000000000000" \
  -f devnet_base_dir=$HOME/.devnet
```

**특징**:
- 최소 리소스 사용
- 빠른 시작 시간
- 로컬 개발에 최적화

### 9. CI/CD 통합 테스트

자동화된 테스트 파이프라인에서 사용할 수 있는 설정입니다.

```bash
gh workflow run deploy-devnet.yml \
  -f v_home=/data/.stable \
  -f daemon_name=stable.service \
  -f validators=3 \
  -f accounts=15 \
  -f chain_id=stable-ci-${{ github.run_number }}
```

**통합 예제** (다른 workflow에서):

```yaml
jobs:
  integration-test:
    runs-on: [self-hosted, ubuntu]
    steps:
      - name: Deploy test devnet
        uses: ./.github/workflows/deploy-devnet.yml
        with:
          v_home: /data/.stable
          validators: 3
          accounts: 15
          chain_id: stable-ci-${{ github.run_number }}

      - name: Run integration tests
        run: |
          npm test -- --network devnet
```

### 10. 멀티 환경 배포

여러 환경을 동시에 관리하는 예제입니다.

#### Production-like 환경
```bash
gh workflow run deploy-devnet.yml \
  -f v_home=/data/.stable-prod \
  -f daemon_name=stable-prod.service \
  -f validators=10 \
  -f accounts=100 \
  -f devnet_base_dir=/data/.devnet-prod \
  -f chain_id=stable-prod-sim
```

#### Staging 환경
```bash
gh workflow run deploy-devnet.yml \
  -f v_home=/data/.stable-staging \
  -f daemon_name=stable-staging.service \
  -f validators=5 \
  -f accounts=50 \
  -f devnet_base_dir=/data/.devnet-staging \
  -f chain_id=stable-staging
```

#### Development 환경
```bash
gh workflow run deploy-devnet.yml \
  -f v_home=/data/.stable-dev \
  -f daemon_name=stable-dev.service \
  -f validators=2 \
  -f accounts=20 \
  -f devnet_base_dir=/data/.devnet-dev \
  -f chain_id=stable-dev
```

## 실전 시나리오

### 시나리오 A: 버그 재현 및 수정

1. **문제 발생한 버전으로 배포**
```bash
gh workflow run deploy-devnet.yml \
  -f stable_tag=v7.0.1-testnet \
  -f v_home=/data/.stable \
  -f daemon_name=stable.service \
  -f validators=4 \
  -f chain_id=bug-reproduction
```

2. **버그 재현 확인**
```bash
# 노드 로그 확인
tail -f /data/.devnet/node0.log

# 특정 트랜잭션 테스트
stabled tx bank send ... --chain-id bug-reproduction
```

3. **수정 후 새 버전으로 재배포**
```bash
gh workflow run deploy-devnet.yml \
  -f stable_tag=v7.0.2-testnet \
  -f v_home=/data/.stable \
  -f daemon_name=stable.service \
  -f validators=4 \
  -f chain_id=bug-fix-test
```

### 시나리오 B: 업그레이드 테스트

1. **현재 버전 배포**
```bash
gh workflow run deploy-devnet.yml \
  -f stable_tag=v7.0.0-testnet \
  -f v_home=/data/.stable \
  -f daemon_name=stable.service \
  -f validators=4 \
  -f chain_id=upgrade-test-old \
  -f devnet_base_dir=/data/.devnet-upgrade-old
```

2. **데이터 생성 및 검증**
```bash
# 트랜잭션 생성
stabled tx bank send ... --chain-id upgrade-test-old

# State 확인
stabled query bank balances ...
```

3. **Genesis export 및 새 버전으로 마이그레이션**
```bash
gh workflow run deploy-devnet.yml \
  -f stable_tag=v7.1.0-testnet \
  -f v_home=/data/.stable \
  -f daemon_name=stable.service \
  -f validators=4 \
  -f chain_id=upgrade-test-new \
  -f devnet_base_dir=/data/.devnet-upgrade-new
```

### 시나리오 C: 성능 벤치마킹

1. **벤치마크 환경 배포**
```bash
gh workflow run deploy-devnet.yml \
  -f v_home=/data/.stable \
  -f daemon_name=stable.service \
  -f validators=10 \
  -f accounts=1000 \
  -f chain_id=benchmark-test \
  -f devnet_base_dir=/data/.devnet-benchmark
```

2. **부하 테스트 실행**
```bash
# 동시 트랜잭션 전송
for i in {1..100}; do
  stabled tx bank send ... --chain-id benchmark-test &
done

# 성능 모니터링
watch -n 1 'curl -s http://localhost:26657/status | jq ".result.sync_info"'
```

3. **결과 분석**
```bash
# TPS 계산
grep "committed state" /data/.devnet-benchmark/node0.log | wc -l

# 블록 시간 분석
curl -s http://localhost:26657/blockchain | jq '.result.block_metas[] | .header.time'
```

## 문제 해결 예제

### Screen 세션 관리

**모든 노드 재시작**:
```bash
# 기존 세션 종료
for i in {0..3}; do screen -S node$i -X quit; done

# 워크플로우 재실행
gh workflow run deploy-devnet.yml -f v_home=/data/.stable ...
```

**특정 노드만 재시작**:
```bash
# node0 중지
screen -S node0 -X quit

# node0 수동 시작
cd /path/to/stable
screen -dmS node0 -L -Logfile /data/.devnet/node0.log \
  bash -c "./build/stabled start --home /data/.devnet/node0 --chain-id stabletestnet_2200-1"
```

### 로그 분석

**에러 검색**:
```bash
grep -i error /data/.devnet/node*.log
```

**특정 트랜잭션 추적**:
```bash
grep "tx_hash" /data/.devnet/node0.log | grep "{TX_HASH}"
```

**블록 생성 모니터링**:
```bash
tail -f /data/.devnet/node0.log | grep "committed state"
```

## 베스트 프랙티스

1. **리소스 계획**
   - Validator 수에 따라 충분한 CPU/메모리 확보
   - 대략적인 가이드: 1 validator = 1 CPU core + 2GB RAM

2. **백업 전략**
   - 중요한 genesis 상태는 별도 백업
   - Workflow는 자동으로 이전 devnet을 백업하지만, 중요한 데이터는 수동 백업 권장

3. **모니터링**
   - 정기적으로 노드 상태 확인
   - Screen 세션이 살아있는지 확인
   - 로그 파일 크기 모니터링

4. **정리**
   - 불필요한 devnet은 정기적으로 삭제
   - 오래된 artifact 정리
   - 로그 파일 로테이션 설정

## 참고 자료

- [GitHub Actions Documentation](https://docs.github.com/actions)
- [Stable Chain Documentation](https://docs.stable.network)
- [devnet-builder README](../../README.md)
- [Workflow README](.github/workflows/README.md)
