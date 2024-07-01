package minimizepower

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TODO: use code-generator to generate DeepCopy functions
// TODO: use standard defaulting/validation mechanisms with code-generator

const (
	DefaultMetricsCacheTTL            = 30 * time.Second
	DefaultPredictorCacheTTL          = 30 * time.Minute
	DefaultPodUsageAssumption float64 = 0.5
	DefaultCPUUsageFormat             = CPUUsageFormatRaw
)

const (
	CPUUsageFormatRaw     string = "Raw"
	CPUUsageFormatPercent string = "Percent"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type MinimizePowerArgs struct {
	metav1.TypeMeta `json:",inline"`

	MetricsCacheTTL   metav1.Duration `json:"metricsCacheTTL,omitempty"`
	PredictorCacheTTL metav1.Duration `json:"predictorCacheTTL,omitempty"`

	// PodUsageAssumption is the assumed fraction of CPU usage that a pod uses, must be [0.0, 1.0]
	PodUsageAssumption float64 `json:"podUsageAssumption,omitempty"`

	// CPUUsageFormat is the format of the CPU usage to use for predictions,
	// options are `Raw` [0.0, NumLogicalCores] or `Percent` [0, 100].
	CPUUsageFormat string `json:"cpuUsageFormat,omitempty"`

	// WeightPowerConsumption is the weight to give to power consumption in the score, must be >= 0.
	WeightPowerConsumption int `json:"weightPowerConsumption,omitempty"`
	// WeightResponseTime is the weight to give to response time in the score, must be >= 0.
	WeightResponseTime int `json:"weightResponseTime,omitempty"`
}

func (args *MinimizePowerArgs) Default() {

	if args.MetricsCacheTTL.Duration == 0 {
		args.MetricsCacheTTL = metav1.Duration{Duration: DefaultMetricsCacheTTL}
	}

	if args.PredictorCacheTTL.Duration == 0 {
		args.PredictorCacheTTL = metav1.Duration{Duration: DefaultPredictorCacheTTL}
	}

	if args.PodUsageAssumption == 0.0 {
		args.PodUsageAssumption = DefaultPodUsageAssumption
	}

	if args.CPUUsageFormat == "" {
		args.CPUUsageFormat = CPUUsageFormatPercent
	}

	// If both weights are 0, it means that the user has not set them, so we set them to 1
	if args.WeightPowerConsumption == 0 && args.WeightResponseTime == 0 {
		args.WeightPowerConsumption = 1
		args.WeightResponseTime = 1
	}

}

func (args *MinimizePowerArgs) Validate() error {

	if args.PodUsageAssumption < 0.0 || args.PodUsageAssumption > 1.0 {
		return fmt.Errorf("podUsageAssumption must be between 0.0 and 1.0")
	}

	if args.CPUUsageFormat != CPUUsageFormatRaw && args.CPUUsageFormat != CPUUsageFormatPercent {
		return fmt.Errorf("cpuUsageFormat must be either `Raw` or `Percent`")
	}

	if args.WeightPowerConsumption < 0 {
		return fmt.Errorf("weightPowerConsumption must be greater than or equal to 0")
	}

	if args.WeightResponseTime < 0 {
		return fmt.Errorf("weightResponseTime must be greater than or equal to 0")
	}

	return nil
}

func (in *MinimizePowerArgs) DeepCopyInto(out *MinimizePowerArgs) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.MetricsCacheTTL = in.MetricsCacheTTL
	out.PredictorCacheTTL = in.PredictorCacheTTL
}

func (in *MinimizePowerArgs) DeepCopy() *MinimizePowerArgs {
	if in == nil {
		return nil
	}
	out := new(MinimizePowerArgs)
	in.DeepCopyInto(out)
	return out
}

func (in *MinimizePowerArgs) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
