package trh_sdk

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"trh-backend/pkg/interfaces/api/dtos"

	trh_sdk_logging "github.com/tokamak-network/trh-sdk/pkg/logging"
	trh_sdk_thanos "github.com/tokamak-network/trh-sdk/pkg/stacks/thanos"
	trh_sdk_types "github.com/tokamak-network/trh-sdk/pkg/types"
)

// TODO: This is a mock implementation of the ThanosStack interface.
// We should use the actual implementation of the ThanosStack interface.
type ThanosStack interface {
	DeployInfrastructure() error
	DestroyInfrastructure() error
	DeployL1Contracts(req *dtos.DeployL1ContractsRequest) error
}

type ThanosStackImpl struct {
}

func NewThanosStack() ThanosStack {
	return &ThanosStackImpl{}
}

func (t *ThanosStackImpl) DeployInfrastructure() error {
	time.Sleep(10 * time.Second)

	// Randomly return error or success
	if rand.Float32() < 0.5 {
		return errors.New("random deployment failure")
	}
	return nil
}

func (t *ThanosStackImpl) DestroyInfrastructure() error {
	time.Sleep(10 * time.Second)

	// Randomly return error or success
	if rand.Float32() < 0.5 {
		return errors.New("random deployment failure")
	}
	return nil
}

func (t *ThanosStackImpl) DeployL1Contracts(req *dtos.DeployL1ContractsRequest) error {
	logger := trh_sdk_logging.InitLogger(req.LogPath)
	s, err := trh_sdk_thanos.NewThanosStack(logger, string(req.Network), true, req.DeploymentPath, nil)
	if err != nil {
		return err
	}
	chainConfig := trh_sdk_types.ChainConfiguration{
		BatchSubmissionFrequency: uint64(req.BatchSubmissionFrequency),
		ChallengePeriod:          uint64(req.ChallengePeriod),
		OutputRootFrequency:      uint64(req.OutputRootFrequency),
		L2BlockTime:              uint64(req.L2BlockTime),
		L1BlockTime:              12,
	}
	operators := trh_sdk_types.Operators{
		AdminPrivateKey:      req.AdminAccount,
		SequencerPrivateKey:  req.SequencerAccount,
		BatcherPrivateKey:    req.BatcherAccount,
		ProposerPrivateKey:   req.ProposerAccount,
		ChallengerPrivateKey: "",
	}

	contractDeploymentInput := trh_sdk_thanos.DeployContractsInput{
		L1RPCurl:           req.L1RpcUrl,
		FraudProof:         false,
		ChainConfiguration: &chainConfig,
		Operators:          &operators,
	}
	err = s.DeployContracts(context.Background(), &contractDeploymentInput)
	if err != nil {
		return err
	}
	return nil
}
