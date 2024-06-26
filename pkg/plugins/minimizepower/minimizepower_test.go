package minimizepower

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	framework "k8s.io/kubernetes/pkg/scheduler/framework"
)

func Test_NormalizeValues2Scores(t *testing.T) {
	tests := []struct {
		name       string
		input      framework.NodeScoreList
		baseScore  int64
		replaceMap map[int64]int64
		want       framework.NodeScoreList
	}{
		{
			name: "1",
			input: framework.NodeScoreList{
				{Name: "n0", Score: 0}, // lowest power increase, highest score
				{Name: "n1", Score: 50},
				{Name: "n2", Score: 100}, // worst power increase, lowest score
			},
			baseScore:  ScoreBase,
			replaceMap: ScoreReplaceMap,
			want: framework.NodeScoreList{
				{Name: "n0", Score: 100},
				{Name: "n1", Score: 60},
				{Name: "n2", Score: 20},
			},
		},
		{
			name: "2",
			input: framework.NodeScoreList{
				{Name: "n0", Score: 20},
				{Name: "n1", Score: 30},
				{Name: "n2", Score: 40},
			},
			baseScore:  ScoreBase,
			replaceMap: ScoreReplaceMap,
			want: framework.NodeScoreList{
				{Name: "n0", Score: 100},
				{Name: "n1", Score: 60},
				{Name: "n2", Score: 20},
			},
		},
		{
			name: "3",
			input: framework.NodeScoreList{
				{Name: "n0", Score: 2000},
				{Name: "n1", Score: 3000},
				{Name: "n2", Score: 4000},
			},
			baseScore:  ScoreBase,
			replaceMap: ScoreReplaceMap,
			want: framework.NodeScoreList{
				{Name: "n0", Score: 100},
				{Name: "n1", Score: 60},
				{Name: "n2", Score: 20},
			},
		},
		{
			name: "same",
			input: framework.NodeScoreList{
				{Name: "n0", Score: 33},
				{Name: "n1", Score: 33},
				{Name: "n2", Score: 33},
			},
			baseScore:  ScoreBase,
			replaceMap: ScoreReplaceMap,
			want: framework.NodeScoreList{
				{Name: "n0", Score: 20},
				{Name: "n1", Score: 20},
				{Name: "n2", Score: 20},
			},
		},
		{
			name: "score_error",
			input: framework.NodeScoreList{
				{Name: "n0", Score: ScoreError},
				{Name: "n1", Score: ScoreError},
				{Name: "n2", Score: ScoreMax},
			},
			baseScore:  ScoreBase,
			replaceMap: ScoreReplaceMap,
			want: framework.NodeScoreList{
				{Name: "n0", Score: 0},
				{Name: "n1", Score: 0},
				{Name: "n2", Score: 1},
			},
		},
		{
			name: "with_special_scores",
			input: framework.NodeScoreList{
				{Name: "n0", Score: ScoreError},
				{Name: "n1", Score: ScoreMax},
				{Name: "n2", Score: 10},
				{Name: "n3", Score: 20},
				{Name: "n4", Score: 30},
			},
			baseScore:  ScoreBase,
			replaceMap: ScoreReplaceMap,
			want: framework.NodeScoreList{
				{Name: "n0", Score: 0},
				{Name: "n1", Score: 1},
				{Name: "n2", Score: 100},
				{Name: "n3", Score: 60},
				{Name: "n4", Score: 20},
			},
		},
		{
			name: "score_base_50",
			input: framework.NodeScoreList{
				{Name: "n0", Score: ScoreError},
				{Name: "n1", Score: ScoreMax},
				{Name: "n2", Score: 10},
				{Name: "n3", Score: 20},
				{Name: "n4", Score: 30},
			},
			baseScore:  50,
			replaceMap: ScoreReplaceMap,
			want: framework.NodeScoreList{
				{Name: "n0", Score: 0},
				{Name: "n1", Score: 1},
				{Name: "n2", Score: 100},
				{Name: "n3", Score: 75},
				{Name: "n4", Score: 50},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NormalizeValues2Scores(tt.input, tt.baseScore, tt.replaceMap)
			if got := tt.input; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PowerConsumptions2Scores() = %v, want %v", got, tt.want)
			} else {
				t.Logf("PowerConsumptions2Scores() = %v, want %v", got, tt.want)
			}
		})
	}
}

func podWithResourceCPU(requests []string, limits []string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
		},
		Spec: corev1.PodSpec{
			InitContainers:      []corev1.Container{},
			Containers:          []corev1.Container{},
			EphemeralContainers: []corev1.EphemeralContainer{},
		},
	}

	if len(requests) != len(limits) {
		panic("len(reqs) != len(limits)")
	}

	for i := range requests {
		container := corev1.Container{
			Name:      fmt.Sprintf("%s-%d", pod.Name, i),
			Resources: corev1.ResourceRequirements{},
		}
		req := requests[i]
		if req != "" {
			container.Resources.Requests = corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse(req),
			}
		}
		lim := limits[i]
		if lim != "" {
			container.Resources.Limits = corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse(lim),
			}
		}
		pod.Spec.Containers = append(pod.Spec.Containers, container)
	}

	return pod
}

var Epsilon float64 = 0.00000001

func floatEquals(a, b float64) bool { return math.Abs(a-b) < Epsilon }

func TestPodCPURequestOrLimit(t *testing.T) {
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name  string
		args  args
		wantV float64
	}{
		{name: "1container", args: args{pod: podWithResourceCPU([]string{"100m"}, []string{"200m"})}, wantV: 0.1},
		{name: "2containers", args: args{pod: podWithResourceCPU([]string{"100m", "200m"}, []string{"200m", "400m"})}, wantV: 0.3},
		{name: "requests_only", args: args{pod: podWithResourceCPU([]string{"100m", "200m"}, []string{"", ""})}, wantV: 0.3},
		{name: "limits_only", args: args{pod: podWithResourceCPU([]string{"", ""}, []string{"200m", "400m"})}, wantV: 0.6},
		{name: "mixed", args: args{pod: podWithResourceCPU([]string{"100m", ""}, []string{"", "400m"})}, wantV: 0.5},
		{name: "empty", args: args{pod: podWithResourceCPU([]string{"", ""}, []string{"", ""})}, wantV: 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotV := PodCPURequestOrLimit(tt.args.pod); !floatEquals(gotV, tt.wantV) {
				t.Errorf("PodCPURequestOrLimit() = %v, want %v", gotV, tt.wantV)
			}
		})
	}
}

func TestMergeInt32AndSplitInt64(t *testing.T) {
	type args struct {
		a int32
		b int32
	}
	tests := []struct {
		name string
		args args
	}{
		{name: "0,0", args: args{a: 0, b: 0}},
		{name: "0,1", args: args{a: 0, b: 1}},
		{name: "1,0", args: args{a: 1, b: 0}},
		{name: "1,1", args: args{a: 1, b: 1}},
		{name: "0,-1", args: args{a: 0, b: -1}},
		{name: "-1,0", args: args{a: -1, b: 0}},
		{name: "-1,-1", args: args{a: -1, b: -1}},
		{name: "max,max", args: args{a: math.MaxInt32, b: math.MaxInt32}},
		{name: "min,max", args: args{a: math.MinInt32, b: math.MaxInt32}},
		{name: "max,min", args: args{a: math.MaxInt32, b: math.MinInt32}},
		{name: "min,min", args: args{a: math.MinInt32, b: math.MinInt32}},
		{name: "random1", args: args{a: 123456, b: -654321}},
		{name: "random2", args: args{a: 342638485, b: 217485171}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, b := SplitInt64(MergeInt32(tt.args.a, tt.args.b))
			if a != tt.args.a || b != tt.args.b {
				t.Errorf("SplitInt64(MergeInt32()) = %v, %v, want %v, %v", a, b, tt.args.a, tt.args.b)
			}
		})
	}
}

func TestMergeScores(t *testing.T) {
	type args struct {
		scoresPC framework.NodeScoreList
		scoresRT framework.NodeScoreList
		weightPC int
		weightRT int
	}
	tests := []struct {
		name string
		args args
		want framework.NodeScoreList
	}{
		{
			name: "1:1",
			args: args{
				scoresPC: framework.NodeScoreList{
					{Name: "n0", Score: 0},
					{Name: "n1", Score: 50},
					{Name: "n2", Score: 100},
				},
				scoresRT: framework.NodeScoreList{
					{Name: "n0", Score: 100},
					{Name: "n1", Score: 50},
					{Name: "n2", Score: 0},
				},
				weightPC: 50,
				weightRT: 50,
			},
			want: framework.NodeScoreList{
				{Name: "n0", Score: 50},
				{Name: "n1", Score: 50},
				{Name: "n2", Score: 50},
			},
		},
		{
			name: "2:1",
			args: args{
				scoresPC: framework.NodeScoreList{
					{Name: "n0", Score: 0},
					{Name: "n1", Score: 50},
					{Name: "n2", Score: 100},
				},
				scoresRT: framework.NodeScoreList{
					{Name: "n0", Score: 100},
					{Name: "n1", Score: 50},
					{Name: "n2", Score: 0},
				},
				weightPC: 2,
				weightRT: 1,
			},
			want: framework.NodeScoreList{
				{Name: "n0", Score: 33},
				{Name: "n1", Score: 50},
				{Name: "n2", Score: 66},
			},
		},
		{
			name: "0:1 (ignore PC)",
			args: args{
				scoresPC: framework.NodeScoreList{
					{Name: "n0", Score: 0},
					{Name: "n1", Score: 50},
					{Name: "n2", Score: 100},
				},
				scoresRT: framework.NodeScoreList{
					{Name: "n0", Score: 100},
					{Name: "n1", Score: 50},
					{Name: "n2", Score: 0},
				},
				weightPC: 0,
				weightRT: 1,
			},
			want: framework.NodeScoreList{
				{Name: "n0", Score: 100},
				{Name: "n1", Score: 50},
				{Name: "n2", Score: 0},
			},
		},
		{
			name: "1:0 (ignore RT)",
			args: args{
				scoresPC: framework.NodeScoreList{
					{Name: "n0", Score: 0},
					{Name: "n1", Score: 50},
					{Name: "n2", Score: 100},
				},
				scoresRT: framework.NodeScoreList{
					{Name: "n0", Score: 100},
					{Name: "n1", Score: 50},
					{Name: "n2", Score: 0},
				},
				weightPC: 1,
				weightRT: 0,
			},
			want: framework.NodeScoreList{
				{Name: "n0", Score: 0},
				{Name: "n1", Score: 50},
				{Name: "n2", Score: 100},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MergeScores(tt.args.scoresPC, tt.args.scoresRT, tt.args.weightPC, tt.args.weightRT); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MergeScores() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAppName(t *testing.T) {
	type args struct {
		podLabels map[string]string
		labelList []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "podLabels is nil",
			args: args{
				podLabels: nil,
				labelList: []string{"app", "name"},
			},
			want: "",
		},
		{
			name: "labelList is nil",
			args: args{
				podLabels: map[string]string{"app": "test-app", "name": "test-name"},
				labelList: nil,
			},
			want: "",
		},
		{
			name: "both are nil",
			args: args{
				podLabels: nil,
				labelList: nil,
			},
			want: "",
		},
		{
			name: "normal",
			args: args{
				podLabels: map[string]string{"app": "test-app", "name": "test-name"},
				labelList: []string{"foobar", "app", "name"},
			},
			want: "test-app",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetAppName(tt.args.podLabels, tt.args.labelList); got != tt.want {
				t.Errorf("GetAppName() = %v, want %v", got, tt.want)
			}
		})
	}
}
