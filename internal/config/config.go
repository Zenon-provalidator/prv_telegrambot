package config

import (
	"log"
)

// 데이터 폴더 상수 정의
const DataDir = "data"

// API 경로 상수
const CoingeckoAPIURL = "https://api.coingecko.com/api/v3/coins/markets?ids=cosmos,osmosis,atomone,photon-2&vs_currency=usd"
const StakingValidatorsEndpoint = "/cosmos/staking/v1beta1/validators?status=BOND_STATUS_BONDED&pagination.limit=10000"
const StakingPoolEndpoint = "/cosmos/staking/v1beta1/pool"
const ExchangeRateAPIURL = "https://open.er-api.com/v6/latest/USD" // 💡 환율 API 추가

// 파일 경로 상수 (DataDir 사용)
const CoingeckoDataFile = DataDir + "/coingecko_data.json"
const PrvValidatorsFile = "prv_validators.json" // 설정 파일은 루트에 유지
const PrvInfoFile = DataDir + "/prv_info.json"

// 통화/계산 상수
const MicroDenomMultiplier = 1_000_000.0
const DefaultUSDtoKRWRate = 1400.0 // 💡 환율 API 에러 시 사용할 기본값

// ChainConfigV2 구조체: prv_validators.json 파일 구조
type ValidatorConfig struct {
	RestPrefix      string `json:"rest_prefix"`
	OperatorAddress string `json:"operator_address"`
}

// 체인별 메타데이터 구조체
type ChainMeta struct {
	CoinGeckoID string
	Emoji       string
	TokenName   string // Cosmos
	Ticker      string // ATOM
}

// 모든 명령어 및 메타데이터 정의
//var ChainMetadata = map[string]ChainMeta{
//	"cosmos":  {"cosmos", "⚛️", "Cosmos", "ATOM"},
//	"osmosis": {"osmosis", "🧪", "Osmosis", "OSMO"},
//	"atomone": {"atomone", "🪐", "AtomOne", "ATONE"},
//	"photon":  {"photon-2", "🛰", "PHOTON", "PHOTON"},
//}

var ChainMetadata = map[string]ChainMeta{
	"cosmos":  {"cosmos", "⚛️", "코스모스", "ATOM"},
	"osmosis": {"osmosis", "🧪", "오스모시스", "OSMO"},
	"atomone": {"atomone", "🪐", "아톰원", "ATONE"},
	"photon":  {"photon-2", "🛰", "포톤", "PHOTON"},
}

// 봇 토큰 로드 함수
func LoadBotToken() string {
	token := "8220707682:AAG0wUjwgXaWoWJ3bVlsjDQ3TC5l2KBbNhk" //
	//token := "2100284989:AAHGootU-e-ZQeAurkgHa_zunlWbx_1F1DY"
	if token == "" {
		log.Fatal("Error: TELEGRAM_BOT_TOKEN environment variable not set.")
	}
	return token
}
