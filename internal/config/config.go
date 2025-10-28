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
	CoinGeckoID   string
	Emoji         string
	TokenName     string // Cosmos
	Ticker        string // ATOM
	StakingTicker string // Body staking Ticker
}

// 모든 명령어 및 메타데이터 정의
//var ChainMetadata = map[string]ChainMeta{
//	"cosmos":  {"cosmos", "⚛️", "Cosmos", "ATOM"},
//	"osmosis": {"osmosis", "🧪", "Osmosis", "OSMO"},
//	"atomone": {"atomone", "🪐", "AtomOne", "ATONE"},
//	"photon":  {"photon-2", "🛰", "PHOTON", "PHOTON"},
//}

var ChainMetadata = map[string]ChainMeta{
	"cosmos":  {"cosmos", "⚛️", "코스모스", "ATOM", "ATOM"},
	"osmosis": {"osmosis", "🧪", "오스모시스", "OSMO", "OSMO"},
	"atomone": {"atomone", "🪐", "아톰원", "ATONE", "ATONE"},
	"photon":  {"photon-2", "🛰", "포톤", "PHOTON", "ATONE"},
}

// 봇 토큰 로드 함수

func LoadBotToken(isProduction bool) string { // 💡 isProduction 플래그를 받도록 수정
	LocalDevToken := "2100284989:AAHGootU-e-ZQeAurkgHa_zunlWbx_1F1DY"
	ProductionToken := "8220707682:AAG0wUjwgXaWoWJ3bVlsjDQ3TC5l2KBbNhk"

	// 플래그를 확인하여 환경별 토큰을 사용 (LocalDevToken과 ProductionToken은 config.go에 정의되어 있다고 가정)
	if isProduction {
		log.Println("Using Production Token based on flag.")
		return ProductionToken
	}

	// 로컬 개발 환경으로 간주
	log.Println("Using Local Development Token.")
	return LocalDevToken
}
