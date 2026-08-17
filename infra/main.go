package main

import (
	"os"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"
)

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	account := jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT"))

	main := NewBrokersSyncStack(app, "BrokersSyncStack", &BrokersSyncStackProps{
		StackProps: awscdk.StackProps{
			Env: &awscdk.Environment{
				Account: account,
				Region:  jsii.String("eu-central-1"),
			},
			CrossRegionReferences: jsii.Bool(true),
		},
	})

	// CloudFront metrics are only published in us-east-1, so the egress alarm
	// cannot live alongside the rest of the stack.
	NewCostGuardStack(app, "BrokersSyncCostGuardStack", &CostGuardStackProps{
		StackProps: awscdk.StackProps{
			Env: &awscdk.Environment{
				Account: account,
				Region:  jsii.String("us-east-1"),
			},
			CrossRegionReferences: jsii.Bool(true),
		},
		DistributionID: main.DistributionId,
	})

	app.Synth(nil)
}
