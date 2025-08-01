package enum

type StackType string

const (
	StackTypeOptimisticRollup StackType = "optimistic-rollup"
	StackTypeZkRollup         StackType = "zk-rollup"
)

func (s StackType) String() string {
	return string(s)
}
