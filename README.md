# k8s-sqs-hpa-controller

## Introduction

`k8s-sqs-hpa-controller` autoscales a kubernetes horizontal pod autoscaler(HPA) based on AWS SQS.
It periodically fetch the number of messages in SQS queue and update HPA's minimum replicas accordingly.

## Usage

Sample Kubernetes manifests are available in the [deploy](https://github.com/RossyWhite/k8s-sqs-hpa-controller/tree/master/deploy) directory.
`sqs:GetQueueAttributes` permission is required to your IAM Access Keys.

The folloing is an example IAM permission.

```json
{
    "Version": "2012-10-17",
    "Statement": [{
        "Effect": "Allow",
        "Action": "sqs:GetQueueAttributes",
        "Resource": "*"
    }]
}
```

And here's an example `deployment` you shold apply.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    run: k8s-sqs-hpa-controller
  name: k8s-sqs-hpa-controller
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      run: k8s-sqs-hpa-controller
  template:
    metadata:
      labels:
        run: k8s-sqs-hpa-controller
    spec:
      serviceAccountName: k8s-sqs-hpa-controller
      containers:
      - image: rossy4613/k8s-sqs-hpa-controller:latest
        name: k8s-sqs-hpa-controller
        args:
          - --poll-interval=10s
          - --min-hpa=1
          - --target='{"queue-url":"sample1", "hpa-name":"hpa1","namespace":"default"}'
          - --target='{"queue-url":"sample2", "hpa-name":"hpa2","namespace":"default"}'
        env:
          - name: AWS_REGION
            value: ap-northeast-1
          - name: AWS_ACCESS_KEY_ID
            value: <your_key>
          - name: AWS_SECRET_ACCESS_KEY
            value: <your_secret>
        resources:
          requests:
            memory: "100Mi"
            cpu: "100m"
          limits:
            memory: "100Mi"
            cpu: "100m"

```

### Configuration Parameters

- `poll-interval` ... Interval to poll SQS queue(default is `10s`)
- `target` ... Pair of SQS URL and HPA name(namespace can be ommited, in which case it will be `default`)
