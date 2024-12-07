package minimizepower

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMinimizePowerArgs_Default(t *testing.T) {
	type fields struct {
		TypeMeta               metav1.TypeMeta
		MetricsCacheTTL        metav1.Duration
		PredictorCacheTTL      metav1.Duration
		PodUsageAssumption     float64
		CPUUsageFormat         string
		WeightPowerConsumption int
		WeightResponseTime     int
	}
	tests := []struct {
		name   string
		fields fields
		want   *MinimizePowerArgs
	}{
		{
			name: "weights should be automatically set",
			fields: fields{
				TypeMeta:               metav1.TypeMeta{},
				MetricsCacheTTL:        metav1.Duration{Duration: time.Minute},
				PredictorCacheTTL:      metav1.Duration{Duration: time.Hour},
				PodUsageAssumption:     0.5,
				CPUUsageFormat:         "Raw",
				WeightPowerConsumption: 0,
				WeightResponseTime:     0,
			},
			want: &MinimizePowerArgs{
				TypeMeta:               metav1.TypeMeta{},
				MetricsCacheTTL:        metav1.Duration{Duration: time.Minute},
				PredictorCacheTTL:      metav1.Duration{Duration: time.Hour},
				PodUsageAssumption:     0.5,
				CPUUsageFormat:         "Raw",
				WeightPowerConsumption: 1,
				WeightResponseTime:     1,
			},
		},
		{
			name: "weights should NOT be automatically set",
			fields: fields{
				TypeMeta:               metav1.TypeMeta{},
				MetricsCacheTTL:        metav1.Duration{Duration: time.Minute},
				PredictorCacheTTL:      metav1.Duration{Duration: time.Hour},
				PodUsageAssumption:     0.5,
				CPUUsageFormat:         "Raw",
				WeightPowerConsumption: 0,
				WeightResponseTime:     1,
			},
			want: &MinimizePowerArgs{
				TypeMeta:               metav1.TypeMeta{},
				MetricsCacheTTL:        metav1.Duration{Duration: time.Minute},
				PredictorCacheTTL:      metav1.Duration{Duration: time.Hour},
				PodUsageAssumption:     0.5,
				CPUUsageFormat:         "Raw",
				WeightPowerConsumption: 0,
				WeightResponseTime:     1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := &MinimizePowerArgs{
				TypeMeta:               tt.fields.TypeMeta,
				MetricsCacheTTL:        tt.fields.MetricsCacheTTL,
				PredictorCacheTTL:      tt.fields.PredictorCacheTTL,
				PodUsageAssumption:     tt.fields.PodUsageAssumption,
				CPUUsageFormat:         tt.fields.CPUUsageFormat,
				WeightPowerConsumption: tt.fields.WeightPowerConsumption,
				WeightResponseTime:     tt.fields.WeightResponseTime,
			}
			args.Default()
			if got := args; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Default() = %v, want %v", got, tt.want)
			} else {
				t.Logf("Default() = %v, want %v", got, tt.want)
			}
		})
	}
}
