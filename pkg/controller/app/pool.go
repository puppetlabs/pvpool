package app

import (
	pvpoolv1alpha1 "github.com/puppetlabs/pvpool/pkg/apis/pvpool.puppet.com/v1alpha1"
	"github.com/puppetlabs/pvpool/pkg/obj"
)

func ConfigurePool(ps *PoolState) *obj.Pool {
	ps.Pool.Object.Status.ObservedGeneration = ps.Pool.Object.GetGeneration()
	ps.Pool.Object.Status.Replicas = int32(len(ps.Available) + len(ps.Initializing) + len(ps.Stale))
	ps.Pool.Object.Status.AvailableReplicas = int32(len(ps.Available))

	var conds []pvpoolv1alpha1.PoolCondition
	for _, typ := range []pvpoolv1alpha1.PoolConditionType{pvpoolv1alpha1.PoolAvailable, pvpoolv1alpha1.PoolSettlement} {
		prev, _ := ps.Pool.Condition(typ)
		next := ps.Conds[typ]
		conds = append(conds, pvpoolv1alpha1.PoolCondition{
			Condition: UpdateCondition(prev.Condition, next),
			Type:      typ,
		})
	}
	ps.Pool.Object.Status.Conditions = conds

	return ps.Pool
}
