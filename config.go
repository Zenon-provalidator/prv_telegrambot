package main

import (
	"log"
)

// API 경로 상수
const coingeckoAPIURL = "https://api.coingecko.com/api/v3/coins/markets?ids=cosmos,osmosis,atomone,photon-2&vs_currency=usd"
const stakingValidatorsEndpoint = "/cosmos/staking/v1beta1/validators?status=BOND_STATUS_BONDED&pagination.limit=10000"
const stakingPoolEndpoint = "/cosmos/staking/v1beta1/pool"

// 파일 경로 상수
const coingeckoDataFile = "coingecko_data.json"
const prvValidatorsFile = "prv_validators.json"
const prvInfoFile = "prv_info.json"

// 통화/계산 상수
const MicroDenomMultiplier = 1_000_000.0
const DefaultUSDToKRWRate = 1300.0 // 예시 환율

// UI 및 하드코딩 데이터 (필요 시 API로 대체 가능)
const StakedRatioHardcoded = 62.52

// 체인별 메타데이터 구조체
type ChainMeta struct {
	CoinGeckoID string
	Emoji       string
	TokenName   string // Cosmos
	Ticker      string // ATOM
}

var ChainMetadata = map[string]ChainMeta{
	"cosmos":  {"cosmos", "⚛️", "Cosmos", "ATOM"},
	"osmosis": {"osmosis", "🧪", "Osmosis", "OSMO"},
	"atomone": {"atomone", "🪐", "AtomOne", "ATONE"},
	"photon":  {"photon-2", "🛰", "PHOTON", "PHOTON"},
}

// prv_validators.json 파일 구조체
type ValidatorConfig struct {
	RestPrefix      string `json:"rest_prefix"`
	OperatorAddress string `json:"operator_address"`
}

// 봇 토큰 로드 함수 (편의를 위해)
func loadBotToken() string {
	// 실제 운영 환경에서는 os.Getenv("TELEGRAM_BOT_TOKEN") 사용 권장
	token := "8220707682:AAG0wUjwgXaWoWJ3bVlsjDQ3TC5l2KBbNhk" // 2100284989:AAHGootU-e-ZQeAurkgHa_zunlWbx_1F1DY
	if token == "" {
		// 테스트용으로 임시 botToken을 사용하거나 환경변수 누락 로그를 남길 수 있습니다.
		log.Println("WARNING: TELEGRAM_BOT_TOKEN not set. Using test token.")
		return "TEST_TOKEN_PLACEHOLDER" // 실제 코드를 실행하려면 유효한 토큰을 사용해야 합니다.
	}
	return token
}
