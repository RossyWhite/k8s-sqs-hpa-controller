package main

import (
	"encoding/json"
	"flag"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/typed/autoscaling/v1"
	"log"
	"strconv"
	"sync"
	"time"
)

type Target struct {
	QueueURL  string `json:"queue-url"`
	HPAName   string `json:"hpa-name"`
	Namespace string `json:"namespace"`
}

type Targets []Target

func (ts *Targets) String() string { return "" }

func (ts *Targets) Set(value string) error {
	var t Target
	if err := json.Unmarshal([]byte(value), &t); err != nil {
		log.Fatal(err)
	}
	if t.Namespace == "" {
		t.Namespace = apiv1.NamespaceDefault
	}
	*ts = append(*ts, t)
	return nil
}

var (
	pollInterval time.Duration
	targets      Targets
	min          int
)

func GetSQSMessageNum(s *sqs.SQS, u string) (int32, error) {
	in := &sqs.GetQueueAttributesInput{
		QueueUrl: aws.String(u),
		AttributeNames: aws.StringSlice([]string{
			"ApproximateNumberOfMessages",
		}),
	}
	out, err := s.GetQueueAttributes(in)
	if err != nil {
		return 0, errors.Wrap(err, "GetQueueAttributes failed")
	}

	i, _ := strconv.Atoi(*out.Attributes["ApproximateNumberOfMessages"])
	return int32(i), nil
}

func GetHpaCurrentMinReplicas(hpaCli v1.HorizontalPodAutoscalerInterface, hpaName string) (int32, error) {
	hpa, err := hpaCli.Get(hpaName, metav1.GetOptions{})
	if err != nil {
		return 0, errors.Wrap(err, "HorizontalPodAutoscaler.Get failed")
	}
	return *hpa.Spec.MinReplicas, nil
}

func UpdateHpaMinReplicas(hpaCli v1.HorizontalPodAutoscalerInterface, hpaName string, new *int32) error {
	hpa, err := hpaCli.Get(hpaName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "HorizontalPodAutoscaler.Get failed")
	}

	hpa.Spec.MinReplicas = new
	_, err = hpaCli.Update(hpa)
	if err != nil {
		return errors.Wrap(err, "HorizontalPodAutoscalerInterface.Update failed")
	}
	return nil
}

func main() {
	flag.DurationVar(&pollInterval, "poll-interval", 10*time.Second, "Interval to get attributes from SQS")
	flag.IntVar(&min, "min-hpa", 1, "minimum number of pods")
	flag.Var(&targets, "target", "target")
	flag.Parse()

	sess, err := session.NewSession(aws.NewConfig())
	if err != nil {
		log.Fatalf("session.NewSession failed: %v", err)
	}
	sqsClient := sqs.New(sess)

	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("rest.InClusterConfig failed: %v", err)
	}

	k8s, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		log.Fatalf("kubernetes.NewForConfig failed: %v", err)
	}

	for {
		select {
		case <-time.After(pollInterval):
			var wg sync.WaitGroup
			for _, t := range targets {
				t := t
				wg.Add(1)
				go func() {
					defer wg.Done()
					log.Printf("Check start: %s", t.HPAName)
					mNum, err := GetSQSMessageNum(sqsClient, t.QueueURL)
					if err != nil {
						log.Fatalln(errors.Wrap(err, "get SQS message failed"))
					}

					hpaClient := k8s.AutoscalingV1().HorizontalPodAutoscalers(t.Namespace)
					rNum, err := GetHpaCurrentMinReplicas(hpaClient, t.HPAName)
					if err != nil {
						log.Fatalln(errors.Wrap(err, "get HPA current minReplicas failed"))
					}

					log.Printf("queue_length: %d, currentmin_replicas: %d", mNum, rNum)

					if mNum != rNum {
						var n int32
						min := int32(min)
						if mNum >= min {
							n = mNum
						} else {
							n = min
						}
						err = UpdateHpaMinReplicas(hpaClient, t.HPAName, &n)
						if err != nil {
							log.Fatalln(errors.Wrap(err, "update HPA minReplicas failed"))
						}
						log.Printf("update HPA success: %d", n)
					}
				}()
			}
			wg.Wait()
		}
	}
}
