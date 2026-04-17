package integrations

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	thanosTypes "github.com/tokamak-network/trh-sdk/pkg/stacks/thanos"
)

// CrossTradeDAppConfig holds all configuration needed to generate the .env.crosstrade file.
type CrossTradeDAppConfig struct {
	L1ChainID              uint64
	L2ChainID              uint64
	L2ChainName            string
	L2RPCURL               string
	L2BlockExplorerURL     string
	DeployOutput           *thanosTypes.DeployCrossTradeLocalOutput
	L1CrossTradeProxyAddr  string
	L2toL2CrossTradeL1Addr string
	// L2NativeTokenName and L2NativeTokenSymbol describe the gas token of the new L2.
	// Defaults to "Tokamak Network" / "TON" when empty (standard Thanos preset).
	// Set to "Ethereum" / "ETH" for defi-eth presets where ETH is the fee token.
	L2NativeTokenName   string
	L2NativeTokenSymbol string
}

// chainConfigEntry is the per-chain JSON structure for NEXT_PUBLIC_CHAIN_CONFIG_* env vars.
type chainConfigEntry struct {
	Name              string            `json:"name"`
	DisplayName       string            `json:"display_name"`
	NativeTokenName   string            `json:"native_token_name"`
	NativeTokenSymbol string            `json:"native_token_symbol"`
	RPCURL            string            `json:"rpc_url"`
	BlockExplorerURL  string            `json:"block_explorer_url"`
	Contracts         map[string]string `json:"contracts"`
	Tokens            interface{}       `json:"tokens"`
}

// l2TokenEntry is the per-token JSON structure used in L2_L2 config's L2 chain entry.
// The CrossTrade dApp expects an array of these (not a flat map) for source L2 chains.
type l2TokenEntry struct {
	Name              string   `json:"name"`
	Address           string   `json:"address"`
	DestinationChains []uint64 `json:"destination_chains"`
}

// dockerizeRPCURL replaces localhost/127.0.0.1 with host.docker.internal so that
// the CrossTrade dApp container can reach the host machine's L2 RPC endpoint.
func dockerizeRPCURL(rpcURL string) string {
	r := strings.ReplaceAll(rpcURL, "localhost", "host.docker.internal")
	return strings.ReplaceAll(r, "127.0.0.1", "host.docker.internal")
}

// BuildDAppEnvConfig generates config/.env.crosstrade for the CrossTrade dApp container (BE-07).
// configPath: absolute or relative path to the output file (e.g. "config/.env.crosstrade").
func BuildDAppEnvConfig(configPath string, cfg *CrossTradeDAppConfig) error {
	l2ChainIDStr := fmt.Sprintf("%d", cfg.L2ChainID)
	l1ChainIDStr := fmt.Sprintf("%d", cfg.L1ChainID)

	// Resolve native token metadata with fallback to standard Thanos (TON) values.
	l2NativeTokenName := cfg.L2NativeTokenName
	if l2NativeTokenName == "" {
		l2NativeTokenName = "Tokamak Network"
	}
	l2NativeTokenSymbol := cfg.L2NativeTokenSymbol
	if l2NativeTokenSymbol == "" {
		l2NativeTokenSymbol = "TON"
	}

	sepoliaTokens := map[string]string{
		"ETH":  "0x0000000000000000000000000000000000000000",
		"USDC": "0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238",
		"USDT": "", // TODO(usdt): Add Sepolia USDT address once confirmed
		"TON":  "",
	}
	// L2_L1 config uses flat map format for L2 tokens.
	l2l1Tokens := map[string]string{
		"ETH":  "0x0000000000000000000000000000000000000000",
		"USDC": "0x4200000000000000000000000000000000000778", // L2 USDC predeploy
		"USDT": "", // TODO(usdt): Add L2 USDT address once confirmed
		"TON":  "",
	}
	// L2_L2 config uses array format for L2 tokens so the CrossTrade dApp can
	// resolve destination_chains correctly when displaying the source chain selector.
	// destination_chains points to Thanos Sepolia (the fixed bridge partner), not itself.
	l2l2Tokens := []l2TokenEntry{
		{Name: "ETH", Address: "0x0000000000000000000000000000000000000000", DestinationChains: []uint64{thanosSepolia}},
		{Name: "USDC", Address: "0x4200000000000000000000000000000000000778", DestinationChains: []uint64{thanosSepolia}},
	}
	// Thanos Sepolia tokens: ETH is at a predeploy address (TON is the native gas token).
	// destination_chains is intentionally empty: the Thanos Sepolia L2toL2CrossTradeProxy
	// does not have the newly deployed L2's chainId registered, so Thanos→new-L2 requests
	// would fail at gas estimation (wallet refuses to sign). Disabling this direction until
	// the Thanos team registers the chain. The reverse direction (new-L2→Thanos) still works.
	thanosL2L2Tokens := []l2TokenEntry{
		{Name: "ETH", Address: "0x4200000000000000000000000000000000000486", DestinationChains: []uint64{}},
		{Name: "TON", Address: "0xDeadDeAddeAddEAddeadDEaDDEAdDeaDDeAD0000", DestinationChains: []uint64{}},
		{Name: "USDC", Address: "0x4200000000000000000000000000000000000778", DestinationChains: []uint64{}},
	}

	// Replace localhost with host.docker.internal so the CrossTrade dApp container
	// (which runs inside Docker) can reach the host machine's L2 RPC endpoint.
	dockerL2RPCURL := dockerizeRPCURL(cfg.L2RPCURL)

	// NEXT_PUBLIC_CHAIN_CONFIG_L2_L1:
	// L1 side: l1_cross_trade = L1CrossTradeProxy address
	// L2 side: l2_cross_trade = L2CrossTradeProxy address
	l2l1Config := map[string]chainConfigEntry{
		l1ChainIDStr: {
			Name:              "Ethereum Sepolia",
			DisplayName:       "Ethereum",
			NativeTokenName:   "Ethereum",
			NativeTokenSymbol: "ETH",
			RPCURL:            "",
			BlockExplorerURL:  "https://sepolia.etherscan.io",
			Contracts:         map[string]string{"l1_cross_trade": cfg.L1CrossTradeProxyAddr},
			Tokens:            sepoliaTokens,
		},
		l2ChainIDStr: {
			Name:              cfg.L2ChainName,
			DisplayName:       cfg.L2ChainName,
			NativeTokenName:   l2NativeTokenName,
			NativeTokenSymbol: l2NativeTokenSymbol,
			RPCURL:            dockerL2RPCURL,
			BlockExplorerURL:  cfg.L2BlockExplorerURL,
			Contracts:         map[string]string{"l2_cross_trade": cfg.DeployOutput.L2CrossTradeProxy},
			Tokens:            l2l1Tokens,
		},
	}

	// NEXT_PUBLIC_CHAIN_CONFIG_L2_L2:
	// L1 side: l1_cross_trade = L2toL2CrossTradeL1 address (different from L2L1 config!)
	// L2 side: l2_cross_trade = L2toL2CrossTradeProxy address
	// Thanos Sepolia is always included as a fixed bridge partner for L2-L2 bridging.
	l2l2Config := map[string]chainConfigEntry{
		l1ChainIDStr: {
			Name:              "Ethereum Sepolia",
			DisplayName:       "Ethereum",
			NativeTokenName:   "Ethereum",
			NativeTokenSymbol: "ETH",
			RPCURL:            "",
			BlockExplorerURL:  "https://sepolia.etherscan.io",
			Contracts:         map[string]string{"l1_cross_trade": cfg.L2toL2CrossTradeL1Addr},
			Tokens:            sepoliaTokens,
		},
		l2ChainIDStr: {
			Name:              cfg.L2ChainName,
			DisplayName:       cfg.L2ChainName,
			NativeTokenName:   l2NativeTokenName,
			NativeTokenSymbol: l2NativeTokenSymbol,
			RPCURL:            dockerL2RPCURL,
			BlockExplorerURL:  cfg.L2BlockExplorerURL,
			Contracts:         map[string]string{"l2_cross_trade": cfg.DeployOutput.L2toL2CrossTradeProxy},
			Tokens:            l2l2Tokens,
		},
		fmt.Sprintf("%d", thanosSepolia): {
			Name:              "Thanos Sepolia",
			DisplayName:       "Thanos Sepolia",
			NativeTokenName:   "Tokamak Network",
			NativeTokenSymbol: "TON",
			RPCURL:            thanosSepoliaRPCURL,
			BlockExplorerURL:  thanosSepoliaExplorerURL,
			Contracts:         map[string]string{"l2_cross_trade": thanosSepoliaL2CTProxy},
			Tokens:            thanosL2L2Tokens,
		},
	}

	l2l1JSON, err := json.Marshal(l2l1Config)
	if err != nil {
		return fmt.Errorf("marshal L2L1 chain config: %w", err)
	}
	l2l2JSON, err := json.Marshal(l2l2Config)
	if err != nil {
		return fmt.Errorf("marshal L2L2 chain config: %w", err)
	}

	content := fmt.Sprintf(
		"NEXT_PUBLIC_CHAIN_CONFIG_L2_L1=%s\nNEXT_PUBLIC_CHAIN_CONFIG_L2_L2=%s\nNEXT_PUBLIC_WALLETCONNECT_PROJECT_ID=\n",
		string(l2l1JSON),
		string(l2l2JSON),
	)

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write .env.crosstrade: %w", err)
	}
	return nil
}

// Sepolia L1 CrossTrade 컨트랙트 상수 (BE-10, BE-11 관련)
const (
	l2CrossDomainMessenger = "0x4200000000000000000000000000000000000007"
	sepoliaTONAddress      = "0xa30fe40285b8f5c0457dbc3b7c8a280373c40044"
)

// Thanos Sepolia 고정 파트너 L2 상수 (L2-L2 브릿지용)
const (
	thanosSepolia            uint64 = 111551119090
	thanosSepoliaL2CTProxy          = "0x7BbEC445F9BDF6c579e81EAda5df86654184BcE3"
	thanosSepoliaRPCURL             = "https://rpc.thanos-sepolia.tokamak.network"
	thanosSepoliaExplorerURL        = "https://explorer.thanos-sepolia-test.tokamak.network"
)

// ABI 문자열: L1CrossTradeProxy.setChainInfo (3-param)
const l1CrossTradeProxySetChainInfoABI = `[{"name":"setChainInfo","type":"function","inputs":[{"name":"_crossDomainMessenger","type":"address"},{"name":"_l2CrossTrade","type":"address"},{"name":"_l2chainId","type":"uint256"}],"outputs":[]}]`

// ABI 문자열: L2toL2CrossTradeL1.setChainInfo (7-param)
const l2toL2CrossTradeL1SetChainInfoABI = `[{"name":"setChainInfo","type":"function","inputs":[{"name":"_crossDomainMessenger","type":"address"},{"name":"_l2CrossTrade","type":"address"},{"name":"_l2NativeTokenAddressOnL1","type":"address"},{"name":"_l1StandardBridge","type":"address"},{"name":"_l1USDCBridge","type":"address"},{"name":"_l2ChainId","type":"uint256"},{"name":"_useCustomBridge","type":"bool"}],"outputs":[]}]`

// CrossTradeL1RegistrationInput은 RegisterCrossTradeL2() 호출에 필요한 입력 데이터다 (BE-11).
type CrossTradeL1RegistrationInput struct {
	L1RPCURL              string // L1 Sepolia RPC (예: "https://rpc.sepolia.org")
	L1ChainID             uint64 // 11155111 (Sepolia)
	L2ChainID             uint64 // 새로 배포된 L2 chain ID
	DeployerPrivKey       string // hex-encoded private key (0x prefix 없음), admin key (index 0)
	L2CrossTradeProxy     string // SDK 배포 결과: L2CrossTradeProxy 주소
	L2toL2CrossTradeProxy string // SDK 배포 결과: L2toL2CrossTradeProxy 주소
	L1StandardBridge      string // deploy.json의 L1StandardBridgeProxy (L1 주소, L2 predeploy 아님)
	L1USDCBridge          string // deploy.json의 L1UsdcBridgeProxy
}

// CrossTradeL1RegistrationOutput은 L1 등록 완료 후 반환되는 결과다 (BE-11).
type CrossTradeL1RegistrationOutput struct {
	L2L1TxHash string // L1CrossTradeProxy.setChainInfo() tx hash
	L2L2TxHash string // L2toL2CrossTradeL1.setChainInfo() tx hash
}

// CrossTradePresetConfig는 DeFi/Full preset에서 L1 CrossTrade 관련 설정을 담는다 (BE-10).
type CrossTradePresetConfig struct {
	L1CrossTradeProxy      string // sepoliaL1CrossTradeProxy 상수 사용
	L2toL2CrossTradeL1Addr string // sepoliaL2toL2CrossTradeL1 상수 사용
	OwnerPrivKey           string // deployer/owner private key (Phase 1에서는 admin key와 동일)
}

// sendL1SetChainInfoTx는 L1 컨트랙트에 calldata를 전송하고 receipt를 기다린다.
// 매 호출마다 최신 nonce를 조회하여 재시도 시 nonce 충돌을 방지한다.
func sendL1SetChainInfoTx(
	ctx context.Context,
	client *ethclient.Client,
	privKey *ecdsa.PrivateKey,
	contractAddr common.Address,
	calldata []byte,
	l1ChainID uint64,
) (string, error) {
	senderAddr := crypto.PubkeyToAddress(privKey.PublicKey)
	chainID := new(big.Int).SetUint64(l1ChainID)
	signer := types.NewEIP155Signer(chainID)

	nonce, err := client.PendingNonceAt(ctx, senderAddr)
	if err != nil {
		return "", fmt.Errorf("get nonce: %w", err)
	}
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("suggest gas price: %w", err)
	}
	// setChainInfo 예상 가스: ~120,000. safety margin 포함해 200,000 고정.
	tx := types.NewTransaction(nonce, contractAddr, big.NewInt(0), 200_000, gasPrice, calldata)
	signedTx, err := types.SignTx(tx, signer, privKey)
	if err != nil {
		return "", fmt.Errorf("sign tx: %w", err)
	}
	if err := client.SendTransaction(ctx, signedTx); err != nil {
		return "", fmt.Errorf("send tx: %w", err)
	}
	receipt, err := bind.WaitMined(ctx, client, signedTx)
	if err != nil {
		return "", fmt.Errorf("wait mined: %w", err)
	}
	if receipt.Status == 0 {
		return "", fmt.Errorf("tx reverted: %s", signedTx.Hash().Hex())
	}
	return signedTx.Hash().Hex(), nil
}

// RegisterCrossTradeL2는 Sepolia L1의 CrossTrade 컨트랙트 2개에 새 L2를 등록한다 (BE-04, BE-05, BE-06).
// L1CrossTradeProxy.setChainInfo (3-param)와 L2toL2CrossTradeL1.setChainInfo (7-param)를 순차 호출한다.
// 각 호출은 maxRetries회까지 재시도하며 attempt*5s 간격을 둔다.
func RegisterCrossTradeL2(ctx context.Context, input *CrossTradeL1RegistrationInput, maxRetries int) (*CrossTradeL1RegistrationOutput, error) {
	if input.DeployerPrivKey == "" {
		return nil, fmt.Errorf("deployer private key is required for CrossTrade L1 registration")
	}

	privKey, err := crypto.HexToECDSA(strings.TrimPrefix(input.DeployerPrivKey, "0x"))
	if err != nil {
		return nil, fmt.Errorf("parse deployer private key: %w", err)
	}

	// L1 ethclient 신규 생성 (SDK client와 분리하여 독립적으로 관리)
	l1Client, err := ethclient.DialContext(ctx, input.L1RPCURL)
	if err != nil {
		return nil, fmt.Errorf("connect to L1 RPC %s: %w", input.L1RPCURL, err)
	}
	defer l1Client.Close()

	// ── L1CrossTradeProxy.setChainInfo (3-param) ──
	l1ABI, err := abi.JSON(strings.NewReader(l1CrossTradeProxySetChainInfoABI))
	if err != nil {
		return nil, fmt.Errorf("parse L1CrossTradeProxy ABI: %w", err)
	}
	l1Calldata, err := l1ABI.Pack("setChainInfo",
		common.HexToAddress(l2CrossDomainMessenger),
		common.HexToAddress(input.L2CrossTradeProxy),
		new(big.Int).SetUint64(input.L2ChainID),
	)
	if err != nil {
		return nil, fmt.Errorf("encode L1CrossTradeProxy.setChainInfo calldata: %w", err)
	}

	var l2l1TxHash string
	var l2l1Err error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		l2l1TxHash, l2l1Err = sendL1SetChainInfoTx(
			ctx, l1Client, privKey,
			common.HexToAddress("0x5AbbFe2468F3bb34B3D5B3F72714b73aa3c1D3EB"),
			l1Calldata, input.L1ChainID,
		)
		if l2l1Err == nil {
			break
		}
		if attempt < maxRetries {
			time.Sleep(time.Duration(attempt) * 5 * time.Second)
		}
	}
	if l2l1Err != nil {
		return nil, fmt.Errorf("L1CrossTradeProxy.setChainInfo failed after %d attempts: %w", maxRetries, l2l1Err)
	}

	// ── L2toL2CrossTradeL1.setChainInfo (7-param) ──
	l2l2ABI, err := abi.JSON(strings.NewReader(l2toL2CrossTradeL1SetChainInfoABI))
	if err != nil {
		return nil, fmt.Errorf("parse L2toL2CrossTradeL1 ABI: %w", err)
	}
	// Pitfall 방어: _l1StandardBridge는 L1 배포 주소 (L2 predeploy 0x4200...0010 아님)
	l2l2Calldata, err := l2l2ABI.Pack("setChainInfo",
		common.HexToAddress(l2CrossDomainMessenger),
		common.HexToAddress(input.L2toL2CrossTradeProxy),
		common.HexToAddress(sepoliaTONAddress),
		common.HexToAddress(input.L1StandardBridge),
		common.HexToAddress(input.L1USDCBridge),
		new(big.Int).SetUint64(input.L2ChainID),
		false, // useCustomBridge: Phase 1 TON fee mode
	)
	if err != nil {
		return nil, fmt.Errorf("encode L2toL2CrossTradeL1.setChainInfo calldata: %w", err)
	}

	var l2l2TxHash string
	var l2l2Err error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		l2l2TxHash, l2l2Err = sendL1SetChainInfoTx(
			ctx, l1Client, privKey,
			common.HexToAddress("0xF09Af74810010a0e9A452f71B3921641350c21D0"),
			l2l2Calldata, input.L1ChainID,
		)
		if l2l2Err == nil {
			break
		}
		if attempt < maxRetries {
			time.Sleep(time.Duration(attempt) * 5 * time.Second)
		}
	}
	if l2l2Err != nil {
		return nil, fmt.Errorf("L2toL2CrossTradeL1.setChainInfo failed after %d attempts: %w", maxRetries, l2l2Err)
	}

	return &CrossTradeL1RegistrationOutput{
		L2L1TxHash: l2l1TxHash,
		L2L2TxHash: l2l2TxHash,
	}, nil
}
