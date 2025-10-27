## README.md

````markdown
# Provalidator Telegram Bot

이 프로젝트는 Go 언어로 작성된 텔레그램 봇입니다. CoinGecko API와 Cosmos SDK REST API를 사용하여 실시간 코인 가격, 시장 데이터, 그리고 특정 체인의 프로밸리(Provalidator) 검증인 순위 및 스테이킹 정보를 수집하여 텔레그램 메시지로 제공합니다.

---

## 📁 프로젝트 구조

| 경로 | 역할 |
| :--- | :--- |
| `cmd/main.go` | 봇 초기화, 텔레그램 메시지 핸들링, 메시지 생성 로직 (엔트리 포인트) |
| `internal/api/collector.go` | 외부 API 호출, 데이터 수집 및 파일 저장, 순위 계산 등 핵심 비즈니스 로직 |
| `internal/config/config.go` | API 경로, 파일 경로 (`data/` 폴더 경로 포함), 체인별 메타데이터 등 모든 상수 관리 |
| `internal/model/data_structs.go` | JSON 언마샬링을 위한 모든 Go 구조체 (CoinGecko, Cosmos Pool, Validator) 정의 |
| `data/` | **(실행 시 자동 생성)** CoinGecko 가격(`coingecko_data.json`) 및 검증인 정보(`prv_info.json`) 저장 |
| `prv_validators.json` | 각 체인의 REST 엔드포인트와 프로밸리 Operator 주소를 정의하는 설정 파일 (수동 관리 필요) |

---

## 🚀 프로젝트 실행 방법

### 1. 전제 조건

* **Go 언어** (Go 1.18+ 권장)
* **Git**

### 2. 환경 설정

#### A. 코드 다운로드

```bash
# 이 프로젝트를 모듈로 사용하기 위해 적절한 폴더에 위치시킵니다.
git clone <프로젝트_URL> prv_telegrambot
cd prv_telegrambot
go mod tidy
````

#### B. 필수 설정 파일 (`prv_validators.json`) 생성

프로젝트 루트 디렉토리에 `prv_validators.json` 파일을 생성하고 지원할 체인의 정보를 입력합니다.

```json
{
    "cosmos": {
        "rest_prefix": "[https://cosmos-rest.publicnode.com](https://cosmos-rest.publicnode.com)",
        "operator_address": "cosmosvaloper1qru73rflllclslgcxglchvuj2z5pvhzldr6lem" 
    },
    "osmosis": {
        "rest_prefix": "[https://osmosis-rest.publicnode.com](https://osmosis-rest.publicnode.com)",
        "operator_address": "osmovaloper14r2z5h6s6384074y9gftz8a0s4f3w8233j522t"
    },
    "atomone": {
        "rest_prefix": "[https://atomone-rest.publicnode.com](https://atomone-rest.publicnode.com)",
        "operator_address": "atonevaloper1q4p7fn0wafmt64c23d78k64ggxr34mzaszzxt8"
    }
}
```

> ⚠️ **주의:** `operator_address`는 반드시 **실제 프로밸리 검증인의 Operator 주소**로 대체해야 합니다.

#### C. 텔레그램 봇 토큰 설정

봇 토큰을 환경 변수로 설정합니다.

```bash
export TELEGRAM_BOT_TOKEN="YOUR:BOT_TOKEN_HERE"
```

### 3\. 봇 실행

루트 디렉토리에서 다음 명령어를 실행하여 봇을 시작합니다. `main.go`는 `cmd` 폴더 안에 있습니다.

```bash
go run ./cmd/main.go
```

봇이 실행되면 자동으로 `data/` 폴더를 생성하고 1분마다 필요한 데이터를 수집합니다.

-----

## ➕ 새로운 체인 추가 가이드

새로운 체인(예: **Juno**)을 봇에 추가하려면 **세 개의 파일**을 수정해야 합니다.

### 1\. 설정 파일 추가 (`prv_validators.json`)

새 체인의 REST 엔드포인트와 프로밸리 Operator 주소를 JSON 파일에 추가합니다.

```json
{
    // ... 기존 체인 ...
    "juno": { 
        "rest_prefix": "[https://juno-rest.publicnode.com](https://juno-rest.publicnode.com)",
        "operator_address": "junovaloper1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" 
    }
}
```

### 2\. 메타데이터 정의 (`internal/config/config.go`)

`ChainMetadata` 맵에 새로운 체인 ID, 이모지, 코인 이름, 티커를 정의합니다.

```go
// internal/config/config.go 파일 내
var ChainMetadata = map[string]ChainMeta{
    // ... 기존 체인 ...
    "juno": {"juno", "☄️", "Juno Network", "JUNO"}, // ✨ 새로운 체인 추가 ✨
}
```

### 3\. 명령어 및 래퍼 함수 추가 (`cmd/main.go`)

`cmd/main.go` 파일을 열어 텔레그램 명령어 처리 로직과 래퍼 함수를 추가합니다.

#### A. `switch` 문에 명령어 추가

```go
// cmd/main.go 파일 내, switch command 문에 추가
case "juno": 
    message, msgErr = generateJunoMessage()
// ... (default: 문 이전)
```

#### B. 래퍼 함수 정의

`generateChainMessage`를 호출하는 래퍼 함수를 추가합니다.

```go
// cmd/main.go 파일 내, 래퍼 함수 목록에 추가
func generateJunoMessage() (string, error) {
	// [chainID, includeStakingRatio]
	// "juno"의 Validator/Pool 정보 사용, stakedTokenDisplay는 "JUNO"
	return generateChainMessage("juno", true)
}
```

> 💡 **참고:** 새 체인의 `chainID`가 `CoinGeckoID`, `validatorInfoKey`, `poolChainID`로 모두 사용되므로, `config.go`에 정의한 `"juno"` ID 하나만으로 모든 정보가 자동으로 연결됩니다.