package trh_sdk

import (
	"errors"
	"math/rand"
	"time"
)

type ThanosStack interface {
	DeployInfrastructure() error
	DestroyInfrastructure() error
	DeployL1Contracts() error
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

func (t *ThanosStackImpl) DeployL1Contracts() error {
	time.Sleep(10 * time.Second)

	// Randomly return error or success
	if rand.Float32() < 0.5 {
		return errors.New("random deployment failure")
	}
	return nil
}
