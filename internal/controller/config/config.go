package config

const (
	// Note that the historySize is the max number of non-live revisions allowed
	// A live revision is a revision that is either being used by at least one
	// pod or is the updaterevision or the currenrevision of PartitionWorkload
	// It does not represent the total number of controllerrevisions
	DefaultHistoryLimit = 10
)
