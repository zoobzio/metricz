package metricz

// Standard bucket definitions for different types of histograms.
var (
	// DefaultLatencyBuckets provides reasonable latency buckets in milliseconds.
	DefaultLatencyBuckets = []float64{
		1, 2, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000,
	}

	// DefaultSizeBuckets provides reasonable size buckets in bytes.
	DefaultSizeBuckets = []float64{
		64, 256, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304,
	}

	// DefaultDurationBuckets provides reasonable duration buckets in seconds.
	DefaultDurationBuckets = []float64{
		0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0,
	}
)
