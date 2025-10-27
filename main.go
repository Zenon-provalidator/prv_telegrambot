package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// API 및 파일 경로 상수
const coingeckoAPIURL = "https://api.coingecko.com/api/v3/coins/markets?ids=cosmos,osmosis,atomone,photon-2&vs_currency=usd"
const coingeckoDataFile = "coingecko_data.json"
const prvValidatorsFile = "prv_validators.json"
const prvInfoFile = "prv_info.json"

// 업데이트된 스테이킹 API 엔드포인트 (검증인 목록)
const stakingValidatorsEndpoint = "/cosmos/staking/v1beta1/validators?status=BOND_STATUS_BONDED&pagination.limit=10000"

// 새로 추가된 스테이킹 풀 API 엔드포인트
const stakingPoolEndpoint = "/cosmos/staking/v1beta1/pool"

// 전역 변수: 파일 접근 보호를 위한 뮤텍스
var fileMutex sync.RWMutex

// udenom을 denom으로 변환하기 위한 상수
const MicroDenomMultiplier = 1_000_000.0

// ChainConfigV2 구조체: prv_validators.json 파일 구조
type ChainConfigV2 struct {
	RestPrefix      string `json:"rest_prefix"`
	Moniker         string `json:"moniker"`
	OperatorAddress string `json:"operator_address"`
}

// StakingPoolInfo 구조체: 네트워크 전체 스테이킹 풀 정보 (원시 문자열 저장)
type StakingPoolInfo struct {
	BondedTokens string `json:"bonded_tokens"` // udenom 문자열
}

// PrvInfo 구조체: prv_info.json 파일 구조
type PrvInfo struct {
	Rank       int    `json:"rank"`
	Staked     string `json:"staked"`     // ATOM 단위 (쉼표 포맷팅)
	Commission string `json:"commission"` // % 단위
}

// PrvDataContainer: prv_info.json에 저장될 전체 데이터 구조
type PrvDataContainer struct {
	Validators map[string]PrvInfo         `json:"validators"`
	Pools      map[string]StakingPoolInfo `json:"pools"`
}

func main() {
	// 봇 토큰 설정
	botToken := "8220707682:AAG0wUjwgXaWoWJ3bVlsjDQ3TC5l2KBbNhk" // 2100284989:AAHGootU-e-ZQeAurkgHa_zunlWbx_1F1DY
	if botToken == "" {
		log.Fatal("Error: TELEGRAM_BOT_TOKEN environment variable not set.")
	}

	// 1. 데이터 수집기 고루틴 시작
	go runCoingeckoDataCollector()
	go runPrvDataCollector()

	// 2. 텔레그램 봇 로직 시작
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil || !update.Message.IsCommand() {
			continue
		}

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")

		var message string
		var msgErr error

		command := update.Message.Command()

		switch command {
		case "cosmos": // 요청하신 명령어 사용
			message, msgErr = generateCosmosMessage()
		case "osmosis": // 요청하신 명령어 사용
			message, msgErr = generateOsmosisMessage()
		case "atomone": // 요청하신 명령어 사용
			message, msgErr = generateAtomoneMessage()
		case "photon": // 요청하신 명령어 사용
			message, msgErr = generatePhotonMessage()
		case "help":
			msg.Text = "Available commands:\n" +
				"/cosmos - Cosmos ($ATOM) Info\n" +
				"/osmosis - Osmosis ($OSMO) Info\n" +
				"/atomone - AtomOne ($ATONE) Info\n" +
				"/photon - PHOTON ($PHOTON) Info"
			if _, err := bot.Send(msg); err != nil {
				log.Println("Error sending message:", err)
			}
			continue
		default:
			msg.Text = "알 수 없는 명령어입니다. /cosmos, /osmosis, /atomone, /photon 중 하나를 입력해 주세요."
			if _, err := bot.Send(msg); err != nil {
				log.Println("Error sending message:", err)
			}
			continue
		}

		if msgErr != nil {
			log.Println("Error generating message:", command, msgErr)
			msg.Text = fmt.Sprintf("데이터를 불러오는 데 실패했습니다. (%s). 잠시 후 다시 시도해 주세요.", command)
		} else {
			msg.Text = message
			msg.ParseMode = tgbotapi.ModeHTML
		}

		if _, err := bot.Send(msg); err != nil {
			log.Println("Error sending message:", err)
		}
	}
}

// -----------------------------------------------------------------------------
// 데이터 수집 및 파일 저장 로직 (Coingecko)
// -----------------------------------------------------------------------------

func runCoingeckoDataCollector() {
	ticker := time.NewTicker(1 * time.Minute)
	collectAndSaveCoingeckoData()
	for {
		select {
		case <-ticker.C:
			collectAndSaveCoingeckoData()
		}
	}
}

func collectAndSaveCoingeckoData() {
	resp, err := http.Get(coingeckoAPIURL)
	if err != nil {
		log.Printf("CoinGecko API call error: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("CoinGecko API response error: %d", resp.StatusCode)
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading CoinGecko response body: %v", err)
		return
	}
	fileMutex.Lock()
	defer fileMutex.Unlock()
	if err := os.WriteFile(coingeckoDataFile, body, 0644); err != nil {
		log.Printf("Error writing CoinGecko JSON file: %v", err)
		return
	}
	log.Printf("CoinGecko data successfully saved to %s", coingeckoDataFile)
}

// -----------------------------------------------------------------------------
// 검증인 정보 수집 로직 (순위, 스테이킹, 수수료, 풀 정보 포함)
// -----------------------------------------------------------------------------

func runPrvDataCollector() {
	ticker := time.NewTicker(1 * time.Minute)
	collectAndSavePrvInfo()

	for {
		select {
		case <-ticker.C:
			collectAndSavePrvInfo()
		}
	}
}

func collectAndSavePrvInfo() {
	log.Println("Starting Provalidator info collection...")

	fileMutex.RLock()
	configBytes, err := os.ReadFile(prvValidatorsFile)
	fileMutex.RUnlock()
	if err != nil {
		log.Printf("Error reading %s: %v", prvValidatorsFile, err)
		return
	}

	var validatorConfigs map[string]ChainConfigV2
	if err := json.Unmarshal(configBytes, &validatorConfigs); err != nil {
		log.Printf("Error unmarshalling %s: %v", prvValidatorsFile, err)
		return
	}

	container := PrvDataContainer{
		Validators: make(map[string]PrvInfo),
		Pools:      make(map[string]StakingPoolInfo),
	}

	for chainID, config := range validatorConfigs {
		// a) 검증인 순위, 위임량, 수수료 정보 수집
		info, err := fetchValidatorInfoV2(config)
		if err != nil {
			log.Printf("Error fetching validator info for %s: %v", chainID, err)
		}
		container.Validators[chainID] = info

		// b) 스테이킹 풀 정보 수집
		poolInfo, err := fetchStakingPoolInfo(config)
		if err != nil {
			log.Printf("Error fetching pool info for %s: %v", chainID, err)
		}
		container.Pools[chainID] = poolInfo
	}

	fileMutex.Lock()
	defer fileMutex.Unlock()
	outputBytes, err := json.MarshalIndent(container, "", "  ")
	if err != nil {
		log.Printf("Error marshalling prv info: %v", err)
		return
	}
	if err := os.WriteFile(prvInfoFile, outputBytes, 0644); err != nil {
		log.Printf("Error writing %s: %v", prvInfoFile, err)
		return
	}
	log.Printf("Provalidator info successfully saved to %s", prvInfoFile)
}

// fetchStakingPoolInfo: /staking/v1beta1/pool API에서 BondedTokens 원시 값을 가져옴
func fetchStakingPoolInfo(config ChainConfigV2) (StakingPoolInfo, error) {
	url := config.RestPrefix + stakingPoolEndpoint

	resp, err := http.Get(url)
	if err != nil {
		return StakingPoolInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return StakingPoolInfo{}, fmt.Errorf("API 응답 코드 오류: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return StakingPoolInfo{}, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return StakingPoolInfo{}, err
	}

	poolRaw, ok := data["pool"].(map[string]interface{})
	if !ok {
		return StakingPoolInfo{}, fmt.Errorf("pool 필드를 찾거나 파싱할 수 없습니다")
	}

	// bonded_tokens 원시 문자열 추출
	bondedStr, ok := poolRaw["bonded_tokens"].(string)
	if !ok {
		return StakingPoolInfo{}, fmt.Errorf("bonded_tokens 필드를 찾을 수 없습니다")
	}

	return StakingPoolInfo{
		BondedTokens: bondedStr,
	}, nil
}

// ValidatorData 구조체: 정렬을 위해 임시로 사용할 구조체
type ValidatorData struct {
	OperatorAddress string
	Tokens          float64 // udenom 단위
	RawData         map[string]interface{}
}

// fetchValidatorInfoV2: /staking/v1beta1/validators API에서 순위, 위임량, 수수료 추출
func fetchValidatorInfoV2(config ChainConfigV2) (PrvInfo, error) {
	url := config.RestPrefix + stakingValidatorsEndpoint

	resp, err := http.Get(url)
	if err != nil {
		return PrvInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return PrvInfo{}, fmt.Errorf("API 응답 코드 오류: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return PrvInfo{}, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return PrvInfo{}, err
	}

	validatorsRaw, ok := data["validators"].([]interface{})
	if !ok {
		return PrvInfo{}, fmt.Errorf("validators 필드를 찾거나 파싱할 수 없습니다")
	}

	var validatorList []ValidatorData

	// 1. 모든 검증인 데이터 파싱 및 리스트에 추가
	for _, item := range validatorsRaw {
		validator, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		tokensStr, _ := validator["tokens"].(string)
		tokensFloat, err := strconv.ParseFloat(tokensStr, 64)
		if err != nil {
			log.Printf("Warning: Skipping validator due to token parsing error: %v", err)
			continue
		}

		operatorAddress, _ := validator["operator_address"].(string)

		validatorList = append(validatorList, ValidatorData{
			OperatorAddress: operatorAddress,
			Tokens:          tokensFloat, // udenom 단위
			RawData:         validator,
		})
	}

	// 2. 위임량(Tokens)을 기준으로 내림차순 정렬 (순위 계산을 위해)
	sort.Slice(validatorList, func(i, j int) bool {
		return validatorList[i].Tokens > validatorList[j].Tokens
	})

	// 3. 정렬된 리스트에서 프로밸리 검증인을 찾고 정보 추출
	for i, vd := range validatorList {
		if vd.OperatorAddress == config.OperatorAddress {
			// 순위는 1부터 시작하므로 인덱스(i) + 1
			rank := i + 1

			// 4. Tokens (위임량) 포맷팅
			stakedATOM := vd.Tokens / MicroDenomMultiplier
			stakedStr := formatNumber(stakedATOM)

			// 5. Commission Rate (수수료) 추출 및 변환
			validator := vd.RawData
			commission, ok := validator["commission"].(map[string]interface{})
			if !ok {
				return PrvInfo{}, fmt.Errorf("commission 필드 형식이 잘못되었습니다")
			}
			commissionRates, ok := commission["commission_rates"].(map[string]interface{})
			if !ok {
				return PrvInfo{}, fmt.Errorf("commission_rates 필드 형식이 잘못되었습니다")
			}
			rateStr, _ := commissionRates["rate"].(string)
			rateFloat, err := strconv.ParseFloat(rateStr, 64)
			if err != nil {
				return PrvInfo{}, fmt.Errorf("commission rate 파싱 오류: %w", err)
			}
			commissionRate := fmt.Sprintf("%.0f", rateFloat*100)

			return PrvInfo{
				Rank:       rank,
				Staked:     stakedStr,
				Commission: commissionRate,
			}, nil
		}
	}

	return PrvInfo{}, fmt.Errorf("지정된 검증인 주소를 찾을 수 없습니다: %s", config.OperatorAddress)
}

// -----------------------------------------------------------------------------
// 공통 및 개별 메시지 생성 로직 (스테이킹 비율 라인 분리 및 Photon 처리)
// -----------------------------------------------------------------------------

// generateChainMessage: 공통 메시지 생성 로직
// includeStakingRatio: Photon처럼 스테이킹 비율 라인을 제외할지 여부
func generateChainMessage(
	chainID string,
	coinGeckoID string,
	emojiSymbol string,
	tokenName string,
	coinTickerSymbol string,
	validatorInfoKey string,
	stakedTokenDisplay string,
	includeStakingRatio bool,
) (string, error) {
	fileMutex.RLock()
	cgDataBytes, err := os.ReadFile(coingeckoDataFile)
	prvInfoBytes, err2 := os.ReadFile(prvInfoFile)
	fileMutex.RUnlock()

	if err != nil || err2 != nil {
		return "", fmt.Errorf("데이터 파일 읽기 오류: CoinGecko(%v), PRV Info(%v)", err, err2)
	}

	var cgData []interface{}
	if err := json.Unmarshal(cgDataBytes, &cgData); err != nil {
		return "", fmt.Errorf("CoinGecko JSON 언마샬 오류: %w", err)
	}

	var prvDataContainer PrvDataContainer
	if err := json.Unmarshal(prvInfoBytes, &prvDataContainer); err != nil {
		return "", fmt.Errorf("PRV Info JSON 언마샬 오류: %w", err)
	}

	var tokenData map[string]interface{}
	for _, item := range cgData {
		coin, ok := item.(map[string]interface{})
		if ok && coin["id"].(string) == coinGeckoID {
			tokenData = coin
			break
		}
	}

	if tokenData == nil {
		return "", fmt.Errorf("%s 데이터를 찾을 수 없습니다.", tokenName)
	}

	prvInfo, ok := prvDataContainer.Validators[validatorInfoKey]
	if !ok {
		return "", fmt.Errorf("prv_info.json에서 %s 검증인 정보를 찾을 수 없습니다.", validatorInfoKey)
	}

	// 풀 정보는 chainID에 해당하는 체인에서 가져옴 (Photon은 atomone 사용)
	poolChainID := chainID
	if chainID == "photon" {
		poolChainID = "atomone"
	}
	poolInfo, ok := prvDataContainer.Pools[poolChainID]
	if !ok {
		return "", fmt.Errorf("prv_info.json에서 %s 풀 정보를 찾을 수 없습니다.", poolChainID)
	}

	// ----------------------------------------------------------------------
	// 스테이킹 비율 라인 생성 (요청에 따라 앞에 빈 줄 추가)
	// ----------------------------------------------------------------------
	var stakingRatioLine string

	if includeStakingRatio {
		// 1. CoinGecko Circulating Supply (denom) 추출
		var circulatingSupply float64
		if cs, ok := tokenData["circulating_supply"]; ok && cs != nil {
			if csFloat, isFloat := cs.(float64); isFloat {
				circulatingSupply = csFloat
			}
		}

		// 2. Bonded Tokens (udenom)을 ATOM으로 변환 (denom)
		bondedUdenom, _ := strconv.ParseFloat(poolInfo.BondedTokens, 64)
		bondedTokensDenom := bondedUdenom / MicroDenomMultiplier

		var stakedPercentFinal float64
		var notStakedPercentFinal float64

		if circulatingSupply > 0 && bondedTokensDenom > 0 {
			stakedPercentFinal = (bondedTokensDenom / circulatingSupply) * 100.0
			notStakedPercentFinal = 100.0 - stakedPercentFinal

			// 소수점 2자리까지 반올림
			stakedPercentFinal = math.Round(stakedPercentFinal*100) / 100
			notStakedPercentFinal = math.Round(notStakedPercentFinal*100) / 100
		}

		// 비율 라인 생성: \n을 앞에 추가하여 한 줄 띄움
		stakingRatioLine = fmt.Sprintf("\n\n🔐Staked / 🔓Unstaked: %.2f%% / %.2f%%", stakedPercentFinal, notStakedPercentFinal)
	}

	// CoinGecko 데이터 포맷팅
	currentPriceUSD, _ := tokenData["current_price"].(float64)
	marketCapUSD, _ := tokenData["market_cap"].(float64)
	totalVolumeUSD, _ := tokenData["total_volume"].(float64)

	var fullyDilutedValuationUSD float64
	if fdv, ok := tokenData["fully_diluted_valuation"]; ok && fdv != nil {
		if fdvFloat, isFloat := fdv.(float64); isFloat {
			fullyDilutedValuationUSD = fdvFloat
		}
	}

	if fullyDilutedValuationUSD == 0.0 && marketCapUSD != 0.0 {
		fullyDilutedValuationUSD = marketCapUSD
	}

	priceUSDStr := fmt.Sprintf("%.2f", currentPriceUSD)
	marketCapStr := formatNumber(marketCapUSD)
	fdvStr := formatNumber(fullyDilutedValuationUSD)
	volumeStr := formatNumber(totalVolumeUSD)

	// 최종 HTML 메시지 생성
	htmlMessage := fmt.Sprintf(`
%s <b>%s ($%s)</b>
----------------------------------------

<b>$%s: $%s</b>%s

💵 Market Cap: $%s

🏛️ FDV: $%s

📊 Volume(24h): $%s

<b>Stake $%s with ❤️Provalidator</b>

<b>🏆Validator Ranking: #%d</b>

<b>🔖Commission: %s%%</b>

<b>🤝Staked: %s %s</b>

----------------------------------------
Supported by <a href='https://provalidator.com' target='_blank'>Provalidator</a>
`,
		emojiSymbol, tokenName, coinTickerSymbol, // 1. 헤더
		coinTickerSymbol, priceUSDStr, // 2. 가격
		stakingRatioLine, // 3. 스테이킹 비율 라인 (앞에 \n\n 포함)
		marketCapStr, fdvStr, volumeStr,
		coinTickerSymbol, // 4. Stake 라인
		prvInfo.Rank, prvInfo.Commission,
		prvInfo.Staked, stakedTokenDisplay, // 5. Staked: 라인
	)

	return htmlMessage, nil
}

// 각 명령어별 래퍼 함수 (Photon은 비율 제외)

func generateCosmosMessage() (string, error) {
	// 비율 포함 (true)
	return generateChainMessage("cosmos", "cosmos", "⚛️", "Cosmos", "ATOM", "cosmos", "ATOM", true)
}

func generateOsmosisMessage() (string, error) {
	// 비율 포함 (true)
	return generateChainMessage("osmosis", "osmosis", "🧪", "Osmosis", "OSMO", "osmosis", "OSMO", true)
}

func generateAtomoneMessage() (string, error) {
	// AtomOne 이모지 🪐, 비율 포함 (true)
	return generateChainMessage("atomone", "atomone", "🪐", "AtomOne", "ATONE", "atomone", "ATONE", true)
}

func generatePhotonMessage() (string, error) {
	// Photon 이모지 🛰, 비율 제외 (false)
	return generateChainMessage("photon", "photon-2", "🛰", "PHOTON", "PHOTON", "atomone", "ATONE", false)
}

// formatNumber: float64 숫자를 쉼표로 구분된 문자열로 포맷팅하는 헬퍼 함수
func formatNumber(f float64) string {
	// 소수점 2자리까지만 표시하고 나머지는 int64로 변환하여 쉼표 처리
	i := int64(math.Round(f))
	if i == 0 && math.Abs(f) > 0.01 { // 0이 아니지만 작은 숫자 처리
		return strconv.FormatFloat(f, 'f', 2, 64)
	}
	if i == 0 {
		return "0"
	}

	in := strconv.FormatInt(i, 10)
	numOfDigits := len(in)
	if numOfDigits <= 3 {
		return in
	}

	// 3자리마다 쉼표 추가
	startIndex := numOfDigits % 3
	if startIndex == 0 {
		startIndex = 3
	}

	var result string
	result += in[:startIndex]

	for i := startIndex; i < numOfDigits; i += 3 {
		result += "," + in[i:i+3]
	}
	return result
}
