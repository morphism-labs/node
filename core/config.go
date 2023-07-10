package node

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/morphism-labs/node/flags"
	"github.com/morphism-labs/node/types"
	"github.com/scroll-tech/go-ethereum/common"
	"github.com/scroll-tech/go-ethereum/common/hexutil"
	tmconfig "github.com/tendermint/tendermint/config"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"github.com/urfave/cli"
)

type Config struct {
	L2                            *types.L2Config `json:"l2"`
	L2CrossDomainMessengerAddress common.Address  `json:"cross_domain_messenger_address"`
	MaxL1MessageNumPerBlock       uint64          `json:"max_l1_message_num_per_block"`
	Logger                        tmlog.Logger    `json:"logger"`
}

func DefaultConfig() *Config {
	return &Config{
		L2:                            new(types.L2Config),
		Logger:                        tmlog.NewTMLogger(tmlog.NewSyncWriter(os.Stdout)),
		MaxL1MessageNumPerBlock:       100,
		L2CrossDomainMessengerAddress: common.HexToAddress("0x4200000000000000000000000000000000000007"),
	}
}

func (c *Config) SetCliContext(ctx *cli.Context) error {
	// logger setting
	logger := tmlog.NewTMLogger(tmlog.NewSyncWriter(os.Stdout))
	if format := ctx.GlobalString(flags.LogFormat.Name); len(format) > 0 && format == tmconfig.LogFormatJSON {
		logger = tmlog.NewTMJSONLogger(tmlog.NewSyncWriter(os.Stdout))
	}

	if ctx.GlobalIsSet(flags.LogLevel.Name) {
		logLevel := ctx.GlobalString(flags.LogLevel.Name)
		option, err := tmlog.AllowLevel(logLevel)
		if err != nil {
			return err
		}
		logger = tmlog.NewFilter(logger, option)
	}
	c.Logger = logger

	l2EthAddr := ctx.GlobalString(flags.L2EthAddr.Name)
	l2EngineAddr := ctx.GlobalString(flags.L2EngineAddr.Name)
	fileName := ctx.GlobalString(flags.L2EngineJWTSecret.Name)
	var secret [32]byte
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		return fmt.Errorf("file-name of jwt secret is empty")
	}
	if data, err := os.ReadFile(fileName); err == nil {
		jwtSecret := common.FromHex(strings.TrimSpace(string(data)))
		if len(jwtSecret) != 32 {
			return fmt.Errorf("invalid jwt secret in path %s, not 32 hex-formatted bytes", fileName)
		}
		copy(secret[:], jwtSecret)
	} else {
		logger.Info("Failed to read JWT secret from file, generating a new one now. Configure L2 geth with --authrpc.jwt-secret=" + fmt.Sprintf("%q", fileName))
		if _, err := io.ReadFull(rand.Reader, secret[:]); err != nil {
			return fmt.Errorf("failed to generate jwt secret: %w", err)
		}
		if err := os.WriteFile(fileName, []byte(hexutil.Encode(secret[:])), 0600); err != nil {
			return err
		}
	}
	c.L2.EthAddr = l2EthAddr
	c.L2.EngineAddr = l2EngineAddr
	c.L2.JwtSecret = secret

	if ctx.GlobalIsSet(flags.MaxL1MessageNumPerBlock.Name) {
		c.MaxL1MessageNumPerBlock = ctx.GlobalUint64(flags.MaxL1MessageNumPerBlock.Name)
		if c.MaxL1MessageNumPerBlock == 0 {
			return fmt.Errorf("MaxL1MessageNumPerBlock must be above 0")
		}
	}

	if ctx.GlobalIsSet(flags.L2CrossDomainMessengerContractAddr.Name) {
		addr := common.HexToAddress(ctx.GlobalString(flags.L2CrossDomainMessengerContractAddr.Name))
		c.L2CrossDomainMessengerAddress = addr
		if len(c.L2CrossDomainMessengerAddress.Bytes()) == 0 {
			return errors.New("invalid SyncDepositContractAddr")
		}
	}

	return nil
}
