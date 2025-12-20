# Package Cleanup Analysis

## 현재 패키지 구조 (2025-12-20 Updated)

```
internal/
├── application/     # [NEW] Clean Architecture - UseCases & Ports ✅
│   ├── ports/       # Interface definitions
│   ├── dto/         # Data Transfer Objects
│   ├── devnet/      # Devnet UseCases (Provision, Run, Stop, Health, Reset, Destroy)
│   ├── upgrade/     # Upgrade UseCases (Propose, Vote, Switch, Execute)
│   └── build/       # Build UseCases (Build, CacheList, CacheClean)
├── infrastructure/  # [NEW] Infrastructure adapters ✅
│   ├── persistence/ # DevnetRepository, NodeRepository
│   ├── process/     # LocalExecutor, DockerExecutor
│   ├── rpc/         # CosmosRPCClient
│   ├── cache/       # BinaryCacheAdapter
│   ├── builder/     # BuilderAdapter
│   ├── node/        # NodeManagerFactory
│   ├── genesis/     # GenesisFetcherAdapter
│   └── snapshot/    # SnapshotFetcherAdapter
├── di/              # [NEW] Dependency Injection container ✅
│   ├── container.go # DI Container with lazy UseCase initialization
│   └── factory.go   # InfrastructureFactory for wiring
├── domain/          # [NEW] Domain entities ✅
├── builder/         # [LEGACY] → infrastructure/builder/ 로 래핑됨
├── cache/           # [LEGACY] → infrastructure/cache/ 로 래핑됨
├── config/          # [KEEP] Configuration management
├── devnet/          # [LEGACY] → application/devnet/ 로 래핑됨
├── github/          # [KEEP] GitHub API client
├── helpers/         # [KEEP] Utility functions
├── interactive/     # [KEEP] Interactive prompts
├── network/         # [KEEP] Core plugin interface
├── node/            # [LEGACY] → infrastructure/node/ 로 래핑됨
├── nodeconfig/      # [LEGACY] 향후 삭제 예정
├── output/          # [KEEP] Logging/output formatting
├── plugin/          # [KEEP] gRPC plugin system
├── prereq/          # [LEGACY] 향후 삭제 예정
├── provision/       # [LEGACY] 향후 삭제 예정
├── snapshot/        # [LEGACY] → infrastructure/snapshot/ 로 래핑됨
└── upgrade/         # [LEGACY] → application/upgrade/ 로 래핑됨
```

## 삭제 가능 패키지

### 1. 삭제 완료 ✅
| 패키지 | 이유 | 상태 |
|--------|------|------|
| `internal/networks/` | 빈 디렉토리 | **삭제됨** |

### 2. 마이그레이션 후 삭제 가능 (향후)
현재 cmd/에서 직접 사용 중이므로, 새로운 Clean Architecture로 마이그레이션 완료 후 삭제 가능:

| 패키지 | 대체 패키지 | 어댑터 상태 | cmd/ 마이그레이션 |
|--------|------------|-------------|-------------------|
| `internal/devnet/` | `application/devnet/` | ✅ 완료 | ⏳ 진행 중 |
| `internal/upgrade/` | `application/upgrade/` | ✅ 완료 | ⏳ 대기 |
| `internal/builder/` | `infrastructure/builder/` | ✅ 완료 | ⏳ 대기 |
| `internal/node/` | `infrastructure/node/` | ✅ 완료 | ⏳ 대기 |
| `internal/snapshot/` | `infrastructure/snapshot/` | ✅ 완료 | ⏳ 대기 |
| `internal/cache/` | `infrastructure/cache/` | ✅ 완료 | ⏳ 대기 |
| `internal/nodeconfig/` | infrastructure 통합 | ❌ 미완료 | ⏳ 대기 |
| `internal/prereq/` | application 통합 | ❌ 미완료 | ⏳ 대기 |

### 3. 유지 필요 (공용 유틸리티)
| 패키지 | 이유 |
|--------|------|
| `internal/output/` | Logger - 전역 사용 |
| `internal/helpers/` | 유틸리티 함수 - 전역 사용 |
| `internal/config/` | 설정 관리 - cmd/ 사용 |
| `internal/interactive/` | TUI 인터페이스 - cmd/ 사용 |
| `internal/github/` | GitHub API - builder/ 사용 |
| `internal/network/` | 플러그인 인터페이스 - 핵심 |
| `internal/plugin/` | gRPC 플러그인 - 핵심 |
| `internal/provision/` | Provisioner - 핵심 기능 |

## 의존성 그래프

```
cmd/ (진입점)
 ├── config
 ├── output
 ├── interactive
 ├── helpers
 ├── network
 ├── devnet ──────┬── node
 │                ├── nodeconfig
 │                ├── prereq
 │                ├── builder
 │                └── cache
 ├── provision
 ├── snapshot
 ├── upgrade ─────┬── node
 │                ├── cache
 │                └── builder
 └── github

infrastructure/ (새로운 어댑터)
 ├── persistence ─── application/ports
 ├── process ─────── application/ports
 ├── rpc ─────────── application/ports
 ├── cache ───────── cache (legacy adapter)
 ├── builder ─────── builder (legacy adapter)
 ├── node ────────── node (legacy adapter)
 ├── genesis ─────── snapshot (legacy adapter)
 └── snapshot ────── snapshot (legacy adapter)
```

## 마이그레이션 진행 현황

### Phase 1: 즉시 실행 ✅ 완료
1. ✅ `internal/networks/` 삭제 (빈 디렉토리)

### Phase 2: Clean Architecture 기반 구축 ✅ 완료
1. ✅ `internal/application/ports/` - 모든 인터페이스 정의
2. ✅ `internal/application/dto/` - 입출력 DTO 정의
3. ✅ `internal/application/devnet/` - Devnet UseCases
4. ✅ `internal/application/upgrade/` - Upgrade UseCases
5. ✅ `internal/application/build/` - Build UseCases
6. ✅ `internal/infrastructure/` - 모든 어댑터 구현
7. ✅ `internal/di/container.go` - DI Container
8. ✅ `internal/di/factory.go` - Infrastructure Factory
9. ✅ `cmd/devnet-builder/app.go` - cmd 레이어 DI 초기화

### Phase 3: cmd/ 마이그레이션 ⏳ 진행 중
1. 각 command에서 `InitContainerForCommand()` 호출
2. 기존 직접 import → container.XXXUseCase() 사용
3. 테스트 추가

### Phase 4: Legacy 패키지 정리 (중기)
cmd/ 마이그레이션 완료 후:
1. `internal/devnet/` 제거 (application/devnet으로 완전 대체)
2. `internal/upgrade/` 제거 (application/upgrade으로 완전 대체)
3. `internal/nodeconfig/` 제거 (infrastructure로 통합)
4. `internal/prereq/` 제거 (application으로 통합)

### Phase 5: 어댑터 직접 구현 (장기)
infrastructure/의 legacy 어댑터들이 직접 구현으로 대체되면:
1. `internal/builder/` 제거
2. `internal/node/` 제거
3. `internal/cache/` 제거
4. `internal/snapshot/` 제거

## 참고 문서

- [ARCHITECTURE.md](./ARCHITECTURE.md) - Clean Architecture 상세 가이드
