package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/morphism-labs/node/core"
	"github.com/morphism-labs/node/db"
	"github.com/morphism-labs/node/flags"
	"github.com/morphism-labs/node/sequencer"
	"github.com/morphism-labs/node/sequencer/mock"
	"github.com/morphism-labs/node/sync"
	"github.com/morphism-labs/node/types"
	"github.com/morphism-labs/node/validator"
	"github.com/scroll-tech/go-ethereum/log"
	tmnode "github.com/tendermint/tendermint/node"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Flags = flags.Flags
	app.Name = "morphnode"
	app.Action = L2NodeMain
	err := app.Run(os.Args)
	if err != nil {
		fmt.Println("Application failed, message: ", err)
		os.Exit(1)
	}
}

func L2NodeMain(ctx *cli.Context) error {
	var (
		err      error
		executor *node.Executor
		syncer   *sync.Syncer
		ms       *mock.Sequencer
		tmNode   *tmnode.Node

		nodeConfig = node.DefaultConfig()
	)
	isSequencer := ctx.GlobalBool(flags.SequencerEnabled.Name)
	isMockSequencer := ctx.GlobalBool(flags.MockEnabled.Name)
	if isSequencer && isMockSequencer {
		return fmt.Errorf("the sequencer and mockSequencer can not be enabled both")
	}
	if err = nodeConfig.SetCliContext(ctx); err != nil {
		return err
	}
	home, err := homeDir(ctx)
	if err != nil {
		return err
	}
	if isSequencer {
		// configure store
		dbConfig := db.DefaultConfig()
		dbConfig.SetCliContext(ctx)
		store, err := db.NewStore(dbConfig, home)
		if err != nil {
			return err
		}

		// launch syncer
		syncConfig := sync.DefaultConfig()
		if err = syncConfig.SetCliContext(ctx); err != nil {
			return err
		}
		syncer, err = sync.NewSyncer(context.Background(), store, syncConfig, nodeConfig.Logger)
		if err != nil {
			return fmt.Errorf("failed to create syncer, error: %v", err)
		}
		syncer.Start()

		// create executor
		executor, err = node.NewSequencerExecutor(nodeConfig, syncer)
		if err != nil {
			return fmt.Errorf("failed to create executor, error: %v", err)
		}

	} else {
		validatorCfg := validator.NewConfig()
		if err := validatorCfg.SetCliContext(ctx); err != nil {
			return fmt.Errorf("validator set cli context error: %v", err)
		}
		validator, err := validator.NewValidator(validatorCfg)
		if err != nil {
			return fmt.Errorf("new validator client error: %v", err)
		}

		// TODO
		validator.ChallengeState(0)

		executor, err = node.NewExecutor(nodeConfig)
		if err != nil {
			return fmt.Errorf("failed to create executor, error: %v", err)
		}
	}

	if isMockSequencer {
		ms, err = mock.NewSequencer(executor)
		if err != nil {
			return err
		}
		go ms.Start()
	} else {
		// launch tendermint node
		if tmNode, err = sequencer.SetupNode(ctx, home, executor, nodeConfig.Logger); err != nil {
			return fmt.Errorf("failed to setup consensus node, error: %v", err)
		}
		if err = tmNode.Start(); err != nil {
			return fmt.Errorf("failed to start consensus node, error: %v", err)
		}
	}

	interruptChannel := make(chan os.Signal, 1)
	signal.Notify(interruptChannel, []os.Signal{
		os.Interrupt,
		os.Kill,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	}...)
	<-interruptChannel

	if ms != nil {
		ms.Stop()
	}
	if tmNode != nil {
		tmNode.Stop()
	}
	if syncer != nil {
		syncer.Stop()
	}

	return nil
}

func homeDir(ctx *cli.Context) (string, error) {
	home := ctx.GlobalString(flags.Home.Name)
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		home = filepath.Join(userHome, types.DefaultHomeDir)
	}
	return home, nil
}
