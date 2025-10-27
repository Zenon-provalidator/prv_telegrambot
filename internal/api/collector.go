package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"prv_telegrambot/internal/config"
	"prv_telegrambot/internal/model"
)

// DataCollector: 모든 데이터 수집 및 관리를 담당하는 메인 구조체
type DataCollector struct {
	mutex            sync.RWMutex
	validatorConfigs map[string]config.ValidatorConfig
}

// NewDataCollector: DataCollector 인스턴스 초기화
func NewDataCollector() *DataCollector {
	dc := &DataCollector{}
	if err := dc.loadValidatorConfigs(); err != nil {
		log.Fatalf("Fatal error loading validator configs: %v", err)
	}
	// 데이터 디렉토리 생성 확인
	ensureDataDir()
	return dc
}

// ensureDataDir: data 폴더가 존재하는지 확인하고 없으면 생성
func ensureDataDir() {
	if _, err := os.Stat(config.DataDir); os.IsNotExist(err) {
		log.Printf("Data directory '%s' not found. Creating it...", config.DataDir)
		if err := os.Mkdir(config.DataDir, 0755); err != nil {
			log.Fatalf("Fatal error creating data directory: %v", err)
		}
	}
}

// loadValidatorConfigs: prv_validators.json 파일을 로드
func (dc *DataCollector) loadValidatorConfigs() error {
	// prv_validators.json은 루트에 위치
	configBytes, err := os.ReadFile(config.PrvValidatorsFile)
	if err != nil {
		return fmt.Errorf("error reading validator config file: %w", err)
	}

	var configs map[string]config.ValidatorConfig
	if err := json.Unmarshal(configBytes, &configs); err != nil {
		return fmt.Errorf("error unmarshalling validator configs: %w", err)
	}
	dc.validatorConfigs = configs
	return nil
}

// RunCollectors: 모든 데이터 수집기 고루틴을 시작
func (dc *DataCollector) RunCollectors() {
	go dc.runCoingeckoDataCollector()
	go dc.runPrvDataCollector()
}

// runCoingeckoDataCollector: 1분마다 코인 가격 정보를 저장
func (dc *DataCollector) runCoingeckoDataCollector() {
	ticker := time.NewTicker(1 * time.Minute)
	dc.collectAndSaveCoingeckoData()

	for {
		select {
		case <-ticker.C:
			dc.collectAndSaveCoingeckoData()
		}
	}
}

func (dc *DataCollector) collectAndSaveCoingeckoData() {
	resp, err := http.Get(config.CoingeckoAPIURL)
	if err != nil {
		log.Printf("Error during CoinGecko API call: %v", err)
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

	dc.mutex.Lock()
	defer dc.mutex.Unlock()
	if err := os.WriteFile(config.CoingeckoDataFile, body, 0644); err != nil {
		log.Printf("Error writing CoinGecko JSON file: %v", err)
		return
	}
	log.Printf("CoinGecko data successfully saved to %s", config.CoingeckoDataFile)
}

// runPrvDataCollector: 1분마다 검증인 정보를 저장
func (dc *DataCollector) runPrvDataCollector() {
	ticker := time.NewTicker(1 * time.Minute)
	dc.collectAndSavePrvInfo()

	for {
		select {
		case <-ticker.C:
			dc.collectAndSavePrvInfo()
		}
	}
}

func (dc *DataCollector) collectAndSavePrvInfo() {
	log.Println("Starting Provalidator info collection...")

	container := model.PrvDataContainer{
		Validators: make(map[string]model.PrvInfo),
		Pools:      make(map[string]model.StakingPoolInfo),
	}

	for chainID, cfg := range dc.validatorConfigs {
		// a) 검증인 순위, 위임량, 수수료 정보 수집
		info, err := dc.fetchValidatorInfo(cfg)
		if err != nil {
			log.Printf("Error fetching validator info for %s: %v", chainID, err)
		}
		container.Validators[chainID] = info

		// b) 스테이킹 풀 정보 수집
		poolInfo, err := dc.fetchStakingPoolInfo(cfg)
		if err != nil {
			log.Printf("Error fetching pool info for %s: %v", chainID, err)
		}
		container.Pools[chainID] = poolInfo
	}

	dc.mutex.Lock()
	defer dc.mutex.Unlock()
	outputBytes, err := json.MarshalIndent(container, "", "  ")
	if err != nil {
		log.Printf("Error marshalling prv info: %v", err)
		return
	}
	if err := os.WriteFile(config.PrvInfoFile, outputBytes, 0644); err != nil {
		log.Printf("Error writing %s: %v", config.PrvInfoFile, err)
		return
	}
	log.Printf("Provalidator info successfully saved to %s", config.PrvInfoFile)
}

// fetchStakingPoolInfo: /staking/v1beta1/pool API에서 BondedTokens 원시 값을 가져옴
func (dc *DataCollector) fetchStakingPoolInfo(cfg config.ValidatorConfig) (model.StakingPoolInfo, error) {
	url := cfg.RestPrefix + config.StakingPoolEndpoint

	resp, err := http.Get(url)
	if err != nil {
		return model.StakingPoolInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return model.StakingPoolInfo{}, fmt.Errorf("API response code error: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return model.StakingPoolInfo{}, err
	}

	var data model.CosmosPoolResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return model.StakingPoolInfo{}, fmt.Errorf("pool data unmarshal error: %w", err)
	}

	return model.StakingPoolInfo{
		BondedTokens: data.Pool.BondedTokens,
	}, nil
}

// ValidatorData 구조체: 정렬을 위해 임시로 사용할 구조체
type ValidatorData struct {
	OperatorAddress string
	Tokens          float64               // udenom 단위
	RawValidator    model.CosmosValidator // 원본 데이터 저장
}

// fetchValidatorInfo: /staking/v1beta1/validators API에서 순위, 위임량, 수수료 추출
func (dc *DataCollector) fetchValidatorInfo(cfg config.ValidatorConfig) (model.PrvInfo, error) {
	url := cfg.RestPrefix + config.StakingValidatorsEndpoint

	resp, err := http.Get(url)
	if err != nil {
		return model.PrvInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return model.PrvInfo{}, fmt.Errorf("API response code error: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return model.PrvInfo{}, err
	}

	var data model.CosmosValidatorsResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return model.PrvInfo{}, fmt.Errorf("validators data unmarshal error: %w", err)
	}

	var validatorList []ValidatorData

	// 1. 모든 검증인 데이터 파싱 및 리스트에 추가
	for _, v := range data.Validators {
		tokensFloat, err := strconv.ParseFloat(v.Tokens, 64)
		if err != nil {
			log.Printf("Warning: Skipping validator due to token parsing error: %v", err)
			continue
		}

		validatorList = append(validatorList, ValidatorData{
			OperatorAddress: v.OperatorAddress,
			Tokens:          tokensFloat,
			RawValidator:    v,
		})
	}

	// 2. 위임량(Tokens)을 기준으로 내림차순 정렬 (순위 계산을 위해)
	sort.Slice(validatorList, func(i, j int) bool {
		return validatorList[i].Tokens > validatorList[j].Tokens
	})

	// 3. 정렬된 리스트에서 프로밸리 검증인을 찾고 정보 추출
	for i, vd := range validatorList {
		if vd.OperatorAddress == cfg.OperatorAddress {
			rank := i + 1

			// Tokens (위임량) 포맷팅
			stakedATOM := vd.Tokens / config.MicroDenomMultiplier
			stakedStr := formatNumber(stakedATOM)

			// Commission Rate (수수료) 추출 및 변환
			rateStr := vd.RawValidator.Commission.CommissionRates.Rate
			rateFloat, err := strconv.ParseFloat(rateStr, 64)
			if err != nil {
				return model.PrvInfo{}, fmt.Errorf("commission rate parsing error: %w", err)
			}
			commissionRate := fmt.Sprintf("%.0f", rateFloat*100)

			return model.PrvInfo{
				Rank:       rank,
				Staked:     stakedStr,
				Commission: commissionRate,
			}, nil
		}
	}

	return model.PrvInfo{}, fmt.Errorf("designated validator address not found: %s", cfg.OperatorAddress)
}

// formatNumber: float64 숫자를 쉼표로 구분된 문자열로 포맷팅하는 헬퍼 함수
func formatNumber(f float64) string {
	i := int64(f) // 간결성을 위해 math.Round 없이 int64로 변환하여 정수부만 사용
	in := strconv.FormatInt(i, 10)
	numOfDigits := len(in)
	if numOfDigits <= 3 {
		return in
	}

	// 3자리마다 쉼표 추가 로직
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
