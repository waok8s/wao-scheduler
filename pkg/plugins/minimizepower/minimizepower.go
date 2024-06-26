package minimizepower

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/discovery"
	cacheddiscovery "k8s.io/client-go/discovery/cached"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/restmapper"
	"k8s.io/klog/v2"
	framework "k8s.io/kubernetes/pkg/scheduler/framework"
	metricsclientv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
	custommetricsclient "k8s.io/metrics/pkg/client/custom_metrics"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	waov1beta1 "github.com/waok8s/wao-core/api/wao/v1beta1"
	waoclient "github.com/waok8s/wao-core/pkg/client"
	waometrics "github.com/waok8s/wao-core/pkg/metrics"
	"github.com/waok8s/wao-core/pkg/predictor"
)

type MinimizePower struct {
	snapshotSharedLister framework.SharedLister
	ctrlclient           client.Client

	metricsclient   *waoclient.CachedMetricsClient
	predictorclient *waoclient.CachedPredictorClient

	args *MinimizePowerArgs
}

var _ framework.PreFilterPlugin = (*MinimizePower)(nil)
var _ framework.ScorePlugin = (*MinimizePower)(nil)
var _ framework.ScoreExtensions = (*MinimizePower)(nil)

var (
	Name = "MinimizePower"

	ReasonResourceRequest = "at least one container in the pod must have a requests.cpu or limits.cpu set"
)

const (
	AnnotationWeightPowerConsumption = "wao.bitmedia.co.jp/weight-power-consumption"
	AnnotationWeightResponseTime     = "wao.bitmedia.co.jp/weight-response-time"

	LabelAppName = "wao.bitmedia.co.jp/app"
)

var (
	// LabelsAppName is the list of labels that are used to identify the application name,
	// in order of preference (LabelsAppName[0] has the highest preference).
	LabelsAppName = []string{
		LabelAppName,
		"app.kubernetes.io/name",
		"app",
	}
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(waov1beta1.AddToScheme(scheme))
}

func getArgs(obj runtime.Object) (MinimizePowerArgs, error) {
	ptr, ok := obj.(*MinimizePowerArgs)
	if !ok {
		return MinimizePowerArgs{}, fmt.Errorf("want args to be of type MinimizePowerArgs, got %T", obj)
	}
	ptr.Default()
	if err := ptr.Validate(); err != nil {
		return MinimizePowerArgs{}, err
	}
	return *ptr, nil
}

// New initializes a new plugin and returns it.
func New(_ context.Context, obj runtime.Object, fh framework.Handle) (framework.Plugin, error) {

	// get plugin args
	args, err := getArgs(obj)
	if err != nil {
		return nil, err
	}
	klog.InfoS("MinimizePower.New", "args", args)

	cfg := fh.KubeConfig()

	// init metrics client
	mc, err := metricsclientv1beta1.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	// init custom metrics client
	// https://github.com/kubernetes/kubernetes/blob/7b9d244efd19f0d4cce4f46d1f34a6c7cff97b18/test/e2e/instrumentation/monitoring/custom_metrics_stackdriver.go#L59
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}
	rm := restmapper.NewDeferredDiscoveryRESTMapper(cacheddiscovery.NewMemCacheClient(dc))
	rm.Reset()
	avg := custommetricsclient.NewAvailableAPIsGetter(dc)
	cmc := custommetricsclient.NewForConfig(cfg, rm, avg)

	// init controller-runtime client
	ca, err := cache.New(fh.KubeConfig(), cache.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, err
	}
	go ca.Start(context.TODO()) // NOTE: this context needs live until the scheduler stops
	c, err := client.New(fh.KubeConfig(), client.Options{
		Scheme: scheme,
		Cache:  &client.CacheOptions{Reader: ca},
	})
	if err != nil {
		return nil, err
	}

	return &MinimizePower{
		snapshotSharedLister: fh.SnapshotSharedLister(),
		ctrlclient:           c,
		metricsclient:        waoclient.NewCachedMetricsClient(mc, cmc, args.MetricsCacheTTL.Duration),
		predictorclient:      waoclient.NewCachedPredictorClient(fh.ClientSet(), args.PredictorCacheTTL.Duration),
		args:                 &args,
	}, nil
}

// Name returns name of the plugin. It is used in logs, etc.
func (*MinimizePower) Name() string { return Name }

// PreFilterExtensions returns nil as this plugin does not have PreFilterExtensions.
func (pl *MinimizePower) PreFilterExtensions() framework.PreFilterExtensions { return nil }

// PreFilter rejects a pod if it does not have at least one container that has a CPU request or limit set.
func (pl *MinimizePower) PreFilter(ctx context.Context, state *framework.CycleState, pod *corev1.Pod) (*framework.PreFilterResult, *framework.Status) {
	klog.InfoS("MinimizePower.PreFilter", "pod", pod.Name)

	if PodCPURequestOrLimit(pod) == 0 {
		return nil, framework.NewStatus(framework.Unschedulable, ReasonResourceRequest)
	}

	return &framework.PreFilterResult{NodeNames: nil}, nil
}

// ScoreExtensions returns a ScoreExtensions interface.
func (pl *MinimizePower) ScoreExtensions() framework.ScoreExtensions { return pl }

var (
	// ScoreBase is the base score for all nodes.
	// This is the lowest score except for special scores (will be replaced to 0,1,...,<ScoreBase)
	ScoreBase int64 = 20
)

const (
	ScoreError int64 = math.MaxInt32 - 1
	ScoreMax   int64 = math.MaxInt32 - 2
)

var (
	// ScoreReplaceMap are scores that have special meanings.
	// NormalizeScore will replace them with the mapped values (should be less than ScoreBase).
	ScoreReplaceMap = map[int64]int64{
		ScoreError: 0,
		ScoreMax:   1,
	}
)

// ScorePowerConsumption returns how many watts will be increased by the given pod (lower is better).
//
// This function never returns an error (as errors cause the pod to be rejected).
// If an error occurs, it is logged and the score is set to ScoreError.
func (pl *MinimizePower) ScorePowerConsumption(ctx context.Context, _ *framework.CycleState, pod *corev1.Pod, nodeName string) (int64, *framework.Status) {
	klog.InfoS("MinimizePower.ScorePowerConsumption", "pod", pod.Name, "node", nodeName)

	nodeInfo, err := pl.snapshotSharedLister.NodeInfos().Get(nodeName)
	if err != nil {
		klog.ErrorS(err, "MinimizePower.ScorePowerConsumption score=ScoreError as error occurred", "pod", pod.Name, "node", nodeName)
		return ScoreError, nil
	}

	// get node and node metrics
	node := nodeInfo.Node()
	nodeMetrics, err := pl.metricsclient.GetNodeMetrics(ctx, node.Name)
	if err != nil {
		klog.ErrorS(err, "MinimizePower.ScorePowerConsumption score=ScoreError as error occurred", "pod", pod.Name, "node", nodeName)
		return ScoreError, nil
	}

	// get additional usage (add assumed pod CPU usage for not running pods)
	// NOTE: We need to assume the CPU usage of pods that have scheduled to this node but are not yet running,
	// as the next replica (if the pod belongs to a deployment, etc.) will be scheduled soon and we need to consider the CPU usage of these pods.
	var assumedAdditionalUsage float64
	for _, p := range nodeInfo.Pods {
		if p.Pod.Spec.NodeName != node.Name {
			continue
		}
		if p.Pod.Status.Phase == corev1.PodRunning ||
			p.Pod.Status.Phase == corev1.PodSucceeded ||
			p.Pod.Status.Phase == corev1.PodFailed ||
			p.Pod.Status.Phase == corev1.PodUnknown {
			continue // only pending pods are counted
		}
		// NOTE: No need to check pod.Status.Conditions as pods on this node with pending status are just what we want.
		// However, pods that have just been started and are not yet using CPU are not counted. (this is a restriction for now)
		assumedAdditionalUsage += PodCPURequestOrLimit(p.Pod) * pl.args.PodUsageAssumption
	}
	// prepare beforeUsage and afterUsage
	beforeUsage := nodeMetrics.Usage.Cpu().AsApproximateFloat64()
	beforeUsage += assumedAdditionalUsage
	afterUsage := beforeUsage + PodCPURequestOrLimit(pod)
	if beforeUsage == afterUsage { // The Pod has both requests.cpu and limits.cpu empty or zero. Normally, this should not happen.
		klog.ErrorS(fmt.Errorf("beforeUsage == afterUsage v=%v", beforeUsage), "MinimizePower.ScorePowerConsumption score=ScoreError as error occurred", "pod", pod.Name, "node", nodeName)
		return ScoreError, nil
	}
	// NOTE: Normally, status.capacity.cpu and status.allocatable.cpu are the same.
	cpuCapacity := node.Status.Capacity.Cpu().AsApproximateFloat64()
	if afterUsage > cpuCapacity { // CPU overcommitment, make the node nearly lowest priority.
		klog.InfoS("MinimizePower.ScorePowerConsumption score=ScoreMax as CPU overcommitment", "pod", pod.Name, "node", nodeName, "usage_after", afterUsage, "cpu_capacity", cpuCapacity)
		return ScoreMax, nil
	}
	klog.InfoS("MinimizePower.ScorePowerConsumption usage", "pod", pod.Name, "node", nodeName, "usage_before", beforeUsage, "usage_after", afterUsage, "additional_usage_included", assumedAdditionalUsage)

	// format usage
	switch pl.args.CPUUsageFormat {
	case CPUUsageFormatRaw:
		// do nothing
	case CPUUsageFormatPercent:
		beforeUsage = (beforeUsage / cpuCapacity) * 100
		afterUsage = (afterUsage / cpuCapacity) * 100
	default:
		// this never happens as args.Validate() checks the value
	}
	klog.InfoS("MinimizePower.ScorePowerConsumption usage (formatted)", "pod", pod.Name, "node", nodeName, "format", pl.args.CPUUsageFormat, "usage_before", beforeUsage, "usage_after", afterUsage, "cpu_capacity", cpuCapacity)

	// get custom metrics
	inletTemp, err := pl.metricsclient.GetCustomMetricForNode(ctx, nodeName, waometrics.ValueInletTemperature)
	if err != nil {
		klog.ErrorS(err, "MinimizePower.ScorePowerConsumption score=ScoreError as error occurred", "pod", pod.Name, "node", nodeName)
		return ScoreError, nil
	}
	deltaP, err := pl.metricsclient.GetCustomMetricForNode(ctx, nodeName, waometrics.ValueDeltaPressure)
	if err != nil {
		klog.ErrorS(err, "MinimizePower.ScorePowerConsumption score=ScoreError as error occurred", "pod", pod.Name, "node", nodeName)
		return ScoreError, nil
	}
	klog.InfoS("MinimizePower.ScorePowerConsumption metrics", "pod", pod.Name, "node", nodeName, "inlet_temp", inletTemp.Value.AsApproximateFloat64(), "delta_p", deltaP.Value.AsApproximateFloat64())

	// get NodeConfig
	var nc *waov1beta1.NodeConfig
	var ncs waov1beta1.NodeConfigList
	if err := pl.ctrlclient.List(ctx, &ncs); err != nil {
		klog.ErrorS(err, "MinimizePower.ScorePowerConsumption score=ScoreError as error occurred", "pod", pod.Name, "node", nodeName)
		return ScoreError, nil
	}
	for _, e := range ncs.Items {
		// TODO: handle node with multiple NodeConfig
		if e.Spec.NodeName == nodeName {
			nc = e.DeepCopy()
			break
		}
	}
	if nc == nil {
		klog.ErrorS(fmt.Errorf("nodeconfig == nil"), "MinimizePower.ScorePowerConsumption score=ScoreError as error occurred", "pod", pod.Name, "node", nodeName)
		return ScoreError, nil
	}

	// init predictor endpoint
	var ep *waov1beta1.EndpointTerm
	if nc.Spec.Predictor.PowerConsumption != nil {
		ep = nc.Spec.Predictor.PowerConsumption
	} else {
		ep = &waov1beta1.EndpointTerm{}
	}

	if nc.Spec.Predictor.PowerConsumptionEndpointProvider != nil {
		ep2, err := pl.predictorclient.GetPredictorEndpoint(ctx, nc.Namespace, nc.Spec.Predictor.PowerConsumptionEndpointProvider, predictor.TypePowerConsumption)
		if err != nil {
			klog.ErrorS(err, "MinimizePower.ScorePowerConsumption score=ScoreError as error occurred", "pod", pod.Name, "node", nodeName)
			return ScoreError, nil
		}
		ep.Type = ep2.Type
		ep.Endpoint = ep2.Endpoint
	}

	// do predict
	beforeWatt, err := pl.predictorclient.PredictPowerConsumption(ctx, nc.Namespace, ep, beforeUsage, inletTemp.Value.AsApproximateFloat64(), deltaP.Value.AsApproximateFloat64())
	if err != nil {
		klog.ErrorS(err, "MinimizePower.ScorePowerConsumption score=ScoreError as error occurred", "pod", pod.Name, "node", nodeName)
		return ScoreError, nil
	}
	afterWatt, err := pl.predictorclient.PredictPowerConsumption(ctx, nc.Namespace, ep, afterUsage, inletTemp.Value.AsApproximateFloat64(), deltaP.Value.AsApproximateFloat64())
	if err != nil {
		klog.ErrorS(err, "MinimizePower.ScorePowerConsumption score=ScoreError as error occurred", "pod", pod.Name, "node", nodeName)
		return ScoreError, nil
	}
	klog.InfoS("MinimizePower.ScorePowerConsumption prediction", "pod", pod.Name, "node", nodeName, "watt_before", beforeWatt, "watt_after", afterWatt)

	podPowerConsumption := int64(afterWatt - beforeWatt)
	if podPowerConsumption < 0 {
		klog.InfoS("MinimizePower.ScorePowerConsumption round negative scores to 0", "pod", pod.Name, "node", nodeName, "watt", afterWatt-beforeWatt)
		podPowerConsumption = 0
	}

	return podPowerConsumption, nil
}

// ScoreResponseTime returns how many milliseconds will be increased by the given pod (lower is better).
//
// This function never returns an error (as errors cause the pod to be rejected).
// If an error occurs, it is logged and the score is set to ScoreError.
func (pl *MinimizePower) ScoreResponseTime(ctx context.Context, _ *framework.CycleState, pod *corev1.Pod, nodeName string) (int64, *framework.Status) {
	klog.InfoS("MinimizePower.ScoreResponseTime", "pod", pod.Name, "node", nodeName)

	nodeInfo, err := pl.snapshotSharedLister.NodeInfos().Get(nodeName)
	if err != nil {
		klog.ErrorS(err, "MinimizePower.ScoreResponseTime score=ScoreError as error occurred", "pod", pod.Name, "node", nodeName)
		return ScoreError, nil
	}

	// get node and node metrics
	node := nodeInfo.Node()
	nodeMetrics, err := pl.metricsclient.GetNodeMetrics(ctx, node.Name)
	if err != nil {
		klog.ErrorS(err, "MinimizePower.ScoreResponseTime score=ScoreError as error occurred", "pod", pod.Name, "node", nodeName)
		return ScoreError, nil
	}

	// get additional usage (add assumed pod CPU usage for not running pods)
	// NOTE: We need to assume the CPU usage of pods that have scheduled to this node but are not yet running,
	// as the next replica (if the pod belongs to a deployment, etc.) will be scheduled soon and we need to consider the CPU usage of these pods.
	var assumedAdditionalUsage float64
	for _, p := range nodeInfo.Pods {
		if p.Pod.Spec.NodeName != node.Name {
			continue
		}
		if p.Pod.Status.Phase == corev1.PodRunning ||
			p.Pod.Status.Phase == corev1.PodSucceeded ||
			p.Pod.Status.Phase == corev1.PodFailed ||
			p.Pod.Status.Phase == corev1.PodUnknown {
			continue // only pending pods are counted
		}
		// NOTE: No need to check pod.Status.Conditions as pods on this node with pending status are just what we want.
		// However, pods that have just been started and are not yet using CPU are not counted. (this is a restriction for now)
		assumedAdditionalUsage += PodCPURequestOrLimit(p.Pod) * pl.args.PodUsageAssumption
	}
	// prepare beforeUsage and afterUsage
	beforeUsage := nodeMetrics.Usage.Cpu().AsApproximateFloat64()
	beforeUsage += assumedAdditionalUsage
	afterUsage := beforeUsage + PodCPURequestOrLimit(pod)
	if beforeUsage == afterUsage { // The Pod has both requests.cpu and limits.cpu empty or zero. Normally, this should not happen.
		klog.ErrorS(fmt.Errorf("beforeUsage == afterUsage v=%v", beforeUsage), "MinimizePower.ScoreResponseTime score=ScoreError as error occurred", "pod", pod.Name, "node", nodeName)
		return ScoreError, nil
	}
	// NOTE: Normally, status.capacity.cpu and status.allocatable.cpu are the same.
	cpuCapacity := node.Status.Capacity.Cpu().AsApproximateFloat64()
	if afterUsage > cpuCapacity { // CPU overcommitment, make the node nearly lowest priority.
		klog.InfoS("MinimizePower.ScoreResponseTime score=ScoreMax as CPU overcommitment", "pod", pod.Name, "node", nodeName, "usage_after", afterUsage, "cpu_capacity", cpuCapacity)
		return ScoreMax, nil
	}
	klog.InfoS("MinimizePower.ScoreResponseTime usage", "pod", pod.Name, "node", nodeName, "usage_before", beforeUsage, "usage_after", afterUsage, "additional_usage_included", assumedAdditionalUsage)

	// format usage
	switch pl.args.CPUUsageFormat {
	case CPUUsageFormatRaw:
		// do nothing
	case CPUUsageFormatPercent:
		beforeUsage = (beforeUsage / cpuCapacity) * 100
		afterUsage = (afterUsage / cpuCapacity) * 100
	default:
		// this never happens as args.Validate() checks the value
	}
	klog.InfoS("MinimizePower.ScoreResponseTime usage (formatted)", "pod", pod.Name, "node", nodeName, "format", pl.args.CPUUsageFormat, "usage_before", beforeUsage, "usage_after", afterUsage, "cpu_capacity", cpuCapacity)

	// get appName
	appName := GetAppName(pod.Labels, LabelsAppName)
	klog.InfoS("MinimizePower.ScoreResponseTime appName", "pod", pod.Name, "node", nodeName, "appName", appName)

	// get NodeConfig
	var nc *waov1beta1.NodeConfig
	var ncs waov1beta1.NodeConfigList
	if err := pl.ctrlclient.List(ctx, &ncs); err != nil {
		klog.ErrorS(err, "MinimizePower.ScoreResponseTime score=ScoreError as error occurred", "pod", pod.Name, "node", nodeName)
		return ScoreError, nil
	}
	for _, e := range ncs.Items {
		// TODO: handle node with multiple NodeConfig
		if e.Spec.NodeName == nodeName {
			nc = e.DeepCopy()
			break
		}
	}
	if nc == nil {
		klog.ErrorS(fmt.Errorf("nodeconfig == nil"), "MinimizePower.ScoreResponseTime score=ScoreError as error occurred", "pod", pod.Name, "node", nodeName)
		return ScoreError, nil
	}

	// init predictor endpoint
	var ep *waov1beta1.EndpointTerm
	if nc.Spec.Predictor.ResponseTime != nil {
		ep = nc.Spec.Predictor.ResponseTime
	} else {
		ep = &waov1beta1.EndpointTerm{}
	}

	if nc.Spec.Predictor.ResponseTimeEndpointProvider != nil {
		ep2, err := pl.predictorclient.GetPredictorEndpoint(ctx, nc.Namespace, nc.Spec.Predictor.ResponseTimeEndpointProvider, predictor.TypeResponseTime)
		if err != nil {
			klog.ErrorS(err, "MinimizePower.ScoreResponseTime score=ScoreError as error occurred", "pod", pod.Name, "node", nodeName)
			return ScoreError, nil
		}
		ep.Type = ep2.Type
		ep.Endpoint = ep2.Endpoint
	}

	// do predict
	responseTime, err := pl.predictorclient.PredictResponseTime(ctx, nc.Namespace, ep, appName, afterUsage)
	if err != nil {
		klog.ErrorS(err, "MinimizePower.ScoreResponseTime score=ScoreError as error occurred", "pod", pod.Name, "node", nodeName)
		return ScoreError, nil
	}
	klog.InfoS("MinimizePower.ScoreResponseTime prediction", "pod", pod.Name, "node", nodeName, "appName", appName, "responseTime", responseTime)
	if responseTime < 0 {
		klog.InfoS("MinimizePower.ScoreResponseTime round negative scores to 0", "pod", pod.Name, "node", nodeName, "appName", appName, "responseTime", responseTime)
		responseTime = 0
	}

	return int64(responseTime), nil
}

// Score runs two predictors (power consumption and response time) and returns the combined score.
//
// This function never returns an error (as errors cause the pod to be rejected).
// If an error occurs, it is logged and the score is set to ScoreError.
//
// Q. Why not two separated Score Plugins?
// A. To allow the user to set the weights per pod, we cannot use `weight` in KubeSchedulerConfiguration as it is static.
// So we merge the scores and split them in NormalizeScore, then apply the weights.
// Also, there are many duplicated processes in the two Score functions, combine them into one Plugin helps caching, etc.
// Downside: timeout handling, code complexity.
func (pl *MinimizePower) Score(ctx context.Context, state *framework.CycleState, pod *corev1.Pod, nodeName string) (int64, *framework.Status) {
	klog.InfoS("MinimizePower.Score", "pod", pod.Name, "node", nodeName)

	var scorePC, scoreRT int64
	var statusPC, statusRT *framework.Status

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scorePC, statusPC = pl.ScorePowerConsumption(ctx, state, pod, nodeName)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		scoreRT, statusRT = pl.ScoreResponseTime(ctx, state, pod, nodeName)
	}()
	wg.Wait()

	// NOTE: in our implementation, statusPC and statusRT are always nil, so omit the check
	_, _ = statusPC, statusRT

	// combine values
	if scorePC > math.MaxInt32 {
		klog.ErrorS(fmt.Errorf("scorePC > math.MaxInt32 v=%v", scorePC), "MinimizePower.Score scorePC=math.MaxInt32 as score overflow", "pod", pod.Name, "node", nodeName)
		scorePC = math.MaxInt32
	}
	if scoreRT > math.MaxInt32 {
		klog.ErrorS(fmt.Errorf("scoreRT > math.MaxInt32 v=%v", scoreRT), "MinimizePower.Score scoreRT=math.MaxInt32 as score overflow", "pod", pod.Name, "node", nodeName)
		scoreRT = math.MaxInt32
	}
	return MergeInt32(int32(scorePC), int32(scoreRT)), nil
}

func MergeInt32(a, b int32) int64 { return int64(a)<<32 | int64(uint32(b)) }

func SplitInt64(v int64) (int32, int32) { return int32(uint32(v >> 32)), int32(uint32(v)) }

func (pl *MinimizePower) NormalizeScore(_ context.Context, _ *framework.CycleState, pod *corev1.Pod, scores framework.NodeScoreList) *framework.Status {
	klog.InfoS("MinimizePower.NormalizeScore before", "pod", pod.Name, "scores", scores)

	// split values
	scoresPC := make(framework.NodeScoreList, len(scores))
	scoresRT := make(framework.NodeScoreList, len(scores))
	for idx, score := range scores {
		vPC, vRT := SplitInt64(score.Score)
		scoresPC[idx] = framework.NodeScore{
			Name:  score.Name,
			Score: int64(vPC),
		}
		scoresRT[idx] = framework.NodeScore{
			Name:  score.Name,
			Score: int64(vRT),
		}
	}

	// normalize scores (values -> scores)
	NormalizeValues2Scores(scoresPC, ScoreBase, ScoreReplaceMap)
	NormalizeValues2Scores(scoresRT, ScoreBase, ScoreReplaceMap)

	klog.InfoS("MinimizePower.NormalizeScore normalized", "pod", pod.Name, "scoresPC", scoresPC, "scoresRT", scoresRT)

	// get weights
	weightPC := pl.args.WeightPowerConsumption
	{
		v, ok := pod.GetAnnotations()[AnnotationWeightPowerConsumption]
		if ok {
			if vv, err := strconv.Atoi(v); err == nil && vv >= 0 {
				weightPC = vv
			}
		}
	}
	weightRT := pl.args.WeightResponseTime
	{
		v, ok := pod.GetAnnotations()[AnnotationWeightResponseTime]
		if ok {
			if vv, err := strconv.Atoi(v); err == nil && vv >= 0 {
				weightRT = vv
			}
		}
	}

	// merge with weights
	scoresMerged := MergeScores(scoresPC, scoresRT, pl.args.WeightPowerConsumption, pl.args.WeightResponseTime)

	// update scores (this function must update the scores in-place)
	for idx := range scores {
		scores[idx].Score = scoresMerged[idx].Score
	}

	klog.InfoS("MinimizePower.NormalizeScore merged with weights", "pod", pod.Name, "weightPC", weightPC, "weightRT", weightRT, "scores", scores)

	return nil
}

func MergeScores(scoresPC, scoresRT framework.NodeScoreList, weightPC, weightRT int) framework.NodeScoreList {
	if len(scoresPC) != len(scoresRT) {
		klog.ErrorS(fmt.Errorf("len(scoresPC) != len(scoresRT) v=%v,%v", len(scoresPC), len(scoresRT)), "MergeScores scoresPC=scoresRT as length mismatch")
		return scoresPC
	}

	scores := make(framework.NodeScoreList, len(scoresPC))
	for idx := range scoresPC {

		v1 := float64(scoresPC[idx].Score*int64(weightPC)) / float64((int64(weightPC) + int64(weightRT)))
		v2 := float64(scoresRT[idx].Score*int64(weightRT)) / float64((int64(weightPC) + int64(weightRT)))
		v := int64(v1 + v2)

		scores[idx] = framework.NodeScore{
			Name:  scoresPC[idx].Name,
			Score: v,
		}
	}

	return scores
}

func NormalizeValues2Scores(scores framework.NodeScoreList, baseScore int64, replaceMap map[int64]int64) {

	var replacedScores []framework.NodeScore
	var calculatedScores []framework.NodeScore

	for _, score := range scores {
		if newScore, ok := replaceMap[score.Score]; ok {
			replacedScores = append(replacedScores, framework.NodeScore{Name: score.Name, Score: newScore})
		} else {
			calculatedScores = append(calculatedScores, framework.NodeScore{Name: score.Name, Score: score.Score})
		}
	}

	// normalize calculatedScores
	highest := int64(math.MinInt64)
	lowest := int64(math.MaxInt64)
	for _, score := range calculatedScores {
		if score.Score > highest {
			highest = score.Score
		}
		if score.Score < lowest {
			lowest = score.Score
		}
	}
	for node, score := range calculatedScores {
		if highest != lowest {
			maxNodeScore := int64(framework.MaxNodeScore)
			minNodeScore := int64(baseScore)
			calculatedScores[node].Score = int64(maxNodeScore - ((maxNodeScore - minNodeScore) * (score.Score - lowest) / (highest - lowest)))
		} else {
			calculatedScores[node].Score = baseScore
		}
	}

	// concat replacedScores and calculatedScores
	scores2 := map[string]framework.NodeScore{}
	for _, score := range replacedScores {
		scores2[score.Name] = score
	}
	for _, score := range calculatedScores {
		scores2[score.Name] = score
	}

	// replace scores
	for i, score := range scores {
		scores[i] = scores2[score.Name]
	}
}

func PodCPURequestOrLimit(pod *corev1.Pod) (v float64) {
	for _, c := range pod.Spec.Containers {
		vv := c.Resources.Requests.Cpu().AsApproximateFloat64()
		if vv == 0 {
			vv = c.Resources.Limits.Cpu().AsApproximateFloat64()
		}
		v += vv
	}
	return
}

// GetAppName returns the application name from the pod labels,
// using the list of labels in order of preference.
// If no label is found, an empty string is returned.
func GetAppName(podLabels map[string]string, labelList []string) string {
	for _, key := range labelList {
		if v, ok := podLabels[key]; ok {
			return v
		}
	}
	return ""
}
