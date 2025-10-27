package model

// CoinGecko API 응답 구조체 (필요한 필드만 정의)
type CoinGeckoCoin struct {
	ID                    string   `json:"id"`
	Symbol                string   `json:"symbol"`
	CurrentPrice          float64  `json:"current_price"`
	MarketCap             float64  `json:"market_cap"`
	FullyDilutedValuation *float64 `json:"fully_diluted_valuation"` // null 처리 위해 포인터 사용
	TotalVolume           float64  `json:"total_volume"`
	CirculatingSupply     float64  `json:"circulating_supply"`
	TotalSupply           float64  `json:"total_supply"`
}

// Cosmos Validators API 응답 구조체 (순위 계산을 위해)
type CosmosValidator struct {
	OperatorAddress string `json:"operator_address"`
	Tokens          string `json:"tokens"`
	Commission      struct {
		CommissionRates struct {
			Rate string `json:"rate"`
		} `json:"commission_rates"`
	} `json:"commission"`
}
type CosmosValidatorsResponse struct {
	Validators []CosmosValidator `json:"validators"`
}

// Cosmos Pool API 응답 구조체
type CosmosPoolResponse struct {
	Pool struct {
		BondedTokens string `json:"bonded_tokens"` // udenom 문자열
	} `json:"pool"`
}

// StakingPoolInfo 구조체: 네트워크 전체 스테이킹 풀 정보 (prv_info.json에 저장)
type StakingPoolInfo struct {
	BondedTokens string `json:"bonded_tokens"` // udenom 문자열
}

// PrvInfo 구조체: 검증인 정보 (prv_info.json에 저장)
type PrvInfo struct {
	Rank       int    `json:"rank"`
	Staked     string `json:"staked"`
	Commission string `json:"commission"`
}

// PrvDataContainer: prv_info.json에 저장될 전체 데이터 구조
type PrvDataContainer struct {
	Validators map[string]PrvInfo         `json:"validators"`
	Pools      map[string]StakingPoolInfo `json:"pools"`
}
