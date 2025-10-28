package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"

	"prv_telegrambot/internal/api"
	"prv_telegrambot/internal/config"
	"prv_telegrambot/internal/model"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// AllData: 메시지 생성을 위해 로드된 모든 데이터를 담는 구조체
type AllData struct {
	CoinGecko []model.CoinGeckoCoin
	Prv       model.PrvDataContainer
}

// loadAllData: 모든 JSON 파일에서 데이터를 읽어오는 헬퍼 함수
func loadAllData() (AllData, error) {
	var data AllData

	// CoinGecko Data
	cgDataBytes, err1 := os.ReadFile(config.CoingeckoDataFile)
	prvInfoBytes, err2 := os.ReadFile(config.PrvInfoFile)

	if err1 != nil {
		return data, fmt.Errorf("CoinGecko data read error: %w", err1)
	}
	if err2 != nil {
		return data, fmt.Errorf("PRV Info data read error: %w", err2)
	}

	// Unmarshal CoinGecko
	if err := json.Unmarshal(cgDataBytes, &data.CoinGecko); err != nil {
		return data, fmt.Errorf("CoinGecko JSON unmarshal error: %w", err)
	}

	// Unmarshal PRV Info
	if err := json.Unmarshal(prvInfoBytes, &data.Prv); err != nil {
		return data, fmt.Errorf("PRV Info JSON unmarshal error: %w", err)
	}

	return data, nil
}

// findCoinGeckoData: CoinGecko 리스트에서 특정 ID의 코인 데이터를 찾는 헬퍼 함수
func findCoinGeckoData(data []model.CoinGeckoCoin, id string) *model.CoinGeckoCoin {
	for _, coin := range data {
		if coin.ID == id {
			return &coin
		}
	}
	return nil
}

// calculateFDV: FDV 값을 계산하거나 MarketCap으로 대체하는 헬퍼 함수
func calculateFDV(tokenData *model.CoinGeckoCoin) float64 {
	// FDV가 nil이 아니거나 0보다 클 경우 사용
	if tokenData.FullyDilutedValuation != nil && *tokenData.FullyDilutedValuation > 0 {
		return *tokenData.FullyDilutedValuation
	}
	// 그렇지 않으면 Market Cap 사용
	return tokenData.MarketCap
}

// calculateStakingRatio: Bonded Tokens와 Circulating Supply를 사용하여 비율을 계산
func calculateStakingRatio(tokenData *model.CoinGeckoCoin, poolInfo model.StakingPoolInfo) (staked float64, unstaked float64, err error) {
	bondedUdenom, err := strconv.ParseFloat(poolInfo.BondedTokens, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("bonded tokens parsing error: %w", err)
	}

	bondedTokensDenom := bondedUdenom / config.MicroDenomMultiplier
	circulatingSupply := tokenData.CirculatingSupply

	if circulatingSupply <= 0 {
		return 0, 0, fmt.Errorf("circulating supply is zero or negative")
	}

	stakedRatio := (bondedTokensDenom / circulatingSupply) * 100.0
	unstakedRatio := 100.0 - stakedRatio

	// 소수점 2자리까지 반올림
	stakedRatio = math.Round(stakedRatio*100) / 100
	unstakedRatio = math.Round(unstakedRatio*100) / 100

	return stakedRatio, unstakedRatio, nil
}

// formatNumber: float64 숫자를 쉼표로 구분된 문자열로 포맷팅하는 헬퍼 함수 (collector.go의 함수를 직접 사용)
// main.go에 복사하지 않고, api 패키지의 함수를 사용해야 합니다.
// (다만, 현재 Go 모듈 구조상 main.go에서 api 패키지 함수를 직접 호출하려면 해당 함수가 export 되어야 합니다.
// 현재는 collector.go에 정의되어 있으므로, main.go에서는 헬퍼 함수를 재정의하지 않고 사용합니다.)
func formatNumber(f float64) string {
	i := int64(f)
	in := strconv.FormatInt(i, 10)
	numOfDigits := len(in)
	if numOfDigits <= 3 {
		return in
	}

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

// generateChainMessage: 공통 메시지 생성 로직
func generateChainMessage(
	chainID string,
	includeStakingRatio bool,
) (string, error) {

	data, err := loadAllData()
	if err != nil {
		return "", err
	}

	meta, ok := config.ChainMetadata[chainID]
	if !ok {
		return "", fmt.Errorf("unknown chain ID: %s", chainID)
	}

	tokenData := findCoinGeckoData(data.CoinGecko, meta.CoinGeckoID)
	if tokenData == nil {
		return "", fmt.Errorf("%s data not found in CoinGecko JSON", meta.TokenName)
	}

	// 검증인 및 풀 정보 키 설정 (Photon은 AtomOne 사용)
	validatorInfoKey := chainID
	poolChainID := chainID
	if chainID == "photon" {
		validatorInfoKey = "atomone"
		poolChainID = "atomone"
	}

	prvInfo, ok := data.Prv.Validators[validatorInfoKey]
	if !ok {
		return "", fmt.Errorf("validator info not found for %s", validatorInfoKey)
	}
	poolInfo, ok := data.Prv.Pools[poolChainID]
	if !ok {
		return "", fmt.Errorf("pool info not found for %s", poolChainID)
	}

	// Staked Token Display 설정
	stakedTokenDisplay := config.ChainMetadata[validatorInfoKey].Ticker

	// 2. 스테이킹 비율 계산 및 라인 생성
	var stakingRatioLine string
	if includeStakingRatio {
		stakedRatio, unstakedRatio, err := calculateStakingRatio(tokenData, poolInfo)
		if err != nil {
			log.Printf("Warning: Failed to calculate staking ratio for %s: %v", chainID, err)
		} else {
			// 비율 라인 생성: \n을 앞에 추가하여 한 줄 띄움
			stakingRatioLine = fmt.Sprintf("\n\n🔐Staked / 🔓Unstaked: %.2f%% / %.2f%%", stakedRatio, unstakedRatio)
		}
	}

	// 3. CoinGecko 데이터 포맷팅
	currentPriceUSD := tokenData.CurrentPrice
	marketCapUSD := tokenData.MarketCap
	totalVolumeUSD := tokenData.TotalVolume

	fullyDilutedValuationUSD := calculateFDV(tokenData)

	priceUSDStr := fmt.Sprintf("%.2f", currentPriceUSD)
	marketCapStr := formatNumber(marketCapUSD)
	fdvStr := formatNumber(fullyDilutedValuationUSD)
	volumeStr := formatNumber(totalVolumeUSD)
	/*
	   `
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
	   `
	*/
	// 4. 최종 HTML 메시지 생성
	htmlMessage := fmt.Sprintf(`
%s <b>%s ($%s)</b>
----------------------------------------

<b>$%s: $%s</b>%s

💵 Market Cap: $%s

🏛️ FDV: $%s

📊 Volume(24h): $%s

<b>프로밸리와 $%s  스테이킹 하세요❤️</b>

<b>🏆검증인 순위: #%d</b>

<b>🔖수수료: %s%%</b>

<b>🤝위임량: %s %s</b>

----------------------------------------
프로밸리(<a href='https://provalidator.com' target='_blank'>Provalidator</a>) 검증인 만듦
`,
		meta.Emoji, meta.TokenName, meta.Ticker,
		meta.Ticker, priceUSDStr,
		stakingRatioLine,
		marketCapStr, fdvStr, volumeStr,
		meta.Ticker,
		prvInfo.Rank, prvInfo.Commission,
		prvInfo.Staked, stakedTokenDisplay,
	)

	return htmlMessage, nil
}

// 각 명령어별 래퍼 함수 (Photon은 비율 제외)

func generateCosmosMessage() (string, error) {
	return generateChainMessage("cosmos", true)
}

func generateOsmosisMessage() (string, error) {
	return generateChainMessage("osmosis", true)
}

func generateAtomoneMessage() (string, error) {
	return generateChainMessage("atomone", true)
}

func generatePhotonMessage() (string, error) {
	return generateChainMessage("photon", false)
}

func main() {
	// 봇 토큰 설정
	botToken := config.LoadBotToken()

	// 데이터 수집기 초기화 및 고루틴 시작
	collector := api.NewDataCollector()
	collector.RunCollectors()

	// 텔레그램 봇 로직 시작
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
		case "cosmos":
			message, msgErr = generateCosmosMessage()
		case "osmosis":
			message, msgErr = generateOsmosisMessage()
		case "atomone":
			message, msgErr = generateAtomoneMessage()
		case "photon":
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
