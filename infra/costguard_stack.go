package main

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsbudgets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatchactions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssnssubscriptions"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// CloudFront has no spend cap. A cache hit never reaches API Gateway, so the
// 10 rps throttle that bounds Lambda does nothing for egress: anyone can loop
// the SPA bundle and run up an open-ended bill. The only real ceiling is to
// disable the distribution, which is what this stack automates.
//
// This stack must be deployed to us-east-1 — CloudFront publishes its
// CloudWatch metrics nowhere else.

type CostGuardStackProps struct {
	awscdk.StackProps
	DistributionID *string
}

func NewCostGuardStack(scope constructs.Construct, id string, props *CostGuardStackProps) awscdk.Stack {
	stack := awscdk.NewStack(scope, &id, &props.StackProps)

	alertEmail := contextString(stack, "alertEmail", "brokersync@dot-core.com")
	if alertEmail == "" {
		// An alarm nobody is subscribed to is worse than no alarm: it looks like
		// protection while silently doing nothing.
		panic("alertEmail context value must not be empty")
	}
	budgetLimit := contextNumber(stack, "budgetLimitUsd", 5)
	alarmGB := contextNumber(stack, "bytesAlarmGb", 5)

	topic := awssns.NewTopic(stack, jsii.String("CostAlerts"), &awssns.TopicProps{
		DisplayName: jsii.String("brokers-sync cost alerts"),
	})
	topic.AddSubscription(awssnssubscriptions.NewEmailSubscription(jsii.String(alertEmail), nil))

	// AWS Budgets publishes from its own service principal, so the topic policy
	// must allow it explicitly.
	topic.AddToResourcePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:    jsii.Strings("SNS:Publish"),
		Principals: &[]awsiam.IPrincipal{awsiam.NewServicePrincipal(jsii.String("budgets.amazonaws.com"), nil)},
		Resources:  &[]*string{topic.TopicArn()},
	}))

	// ── Kill switch ───────────────────────────────────────────────────────────
	// Inline Node keeps this to one file; the runtime already bundles AWS SDK v3,
	// so no bundler is needed in an otherwise Go-only CDK app.
	killSwitch := awslambda.NewFunction(stack, jsii.String("DisableDistributionFn"), &awslambda.FunctionProps{
		Runtime:     awslambda.Runtime_NODEJS_22_X(),
		Handler:     jsii.String("index.handler"),
		Timeout:     awscdk.Duration_Seconds(jsii.Number(30)),
		Description: jsii.String("Disables the CloudFront distribution when egress spikes"),
		Environment: &map[string]*string{
			"DISTRIBUTION_ID": props.DistributionID,
		},
		Code: awslambda.Code_FromInline(jsii.String(killSwitchCode)),
	})
	killSwitch.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: jsii.Strings("cloudfront:GetDistributionConfig", "cloudfront:UpdateDistribution"),
		Resources: &[]*string{
			jsii.String(fmt.Sprintf("arn:aws:cloudfront::%s:distribution/%s", *stack.Account(), *props.DistributionID)),
		},
	}))

	// Normal use is well under 1 GB per month, so this threshold sits orders of
	// magnitude above baseline: a trigger means abuse, not a busy day.
	alarm := awscloudwatch.NewAlarm(stack, jsii.String("EgressAlarm"), &awscloudwatch.AlarmProps{
		AlarmDescription: jsii.String("CloudFront egress spike — disables the distribution"),
		Metric: awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/CloudFront"),
			MetricName: jsii.String("BytesDownloaded"),
			Statistic:  jsii.String("Sum"),
			Period:     awscdk.Duration_Minutes(jsii.Number(5)),
			DimensionsMap: &map[string]*string{
				"DistributionId": props.DistributionID,
				"Region":         jsii.String("Global"),
			},
		}),
		Threshold:          jsii.Number(alarmGB * 1024 * 1024 * 1024),
		EvaluationPeriods:  jsii.Number(1),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
		TreatMissingData:   awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	alarm.AddAlarmAction(
		awscloudwatchactions.NewSnsAction(topic),
		awscloudwatchactions.NewLambdaAction(killSwitch, nil),
	)

	// ── Budget ────────────────────────────────────────────────────────────────
	// Backup only: billing data lags 8-24h, so this catches a slow bleed, not a
	// burst. The alarm above is what caps a burst.
	awsbudgets.NewCfnBudget(stack, jsii.String("MonthlyBudget"), &awsbudgets.CfnBudgetProps{
		Budget: &awsbudgets.CfnBudget_BudgetDataProperty{
			BudgetName: jsii.String("brokers-sync-monthly"),
			BudgetType: jsii.String("COST"),
			TimeUnit:   jsii.String("MONTHLY"),
			BudgetLimit: &awsbudgets.CfnBudget_SpendProperty{
				Amount: jsii.Number(budgetLimit),
				Unit:   jsii.String("USD"),
			},
		},
		NotificationsWithSubscribers: &[]any{
			budgetNotification("ACTUAL", 80, topic.TopicArn()),
			budgetNotification("FORECASTED", 100, topic.TopicArn()),
		},
	})

	awscdk.NewCfnOutput(stack, jsii.String("CostAlertTopic"), &awscdk.CfnOutputProps{
		Value:       topic.TopicArn(),
		Description: jsii.String("Confirm the email subscription or no alert is ever delivered"),
	})

	return stack
}

func budgetNotification(kind string, threshold float64, topicArn *string) *awsbudgets.CfnBudget_NotificationWithSubscribersProperty {
	return &awsbudgets.CfnBudget_NotificationWithSubscribersProperty{
		Notification: &awsbudgets.CfnBudget_NotificationProperty{
			NotificationType:   jsii.String(kind),
			ComparisonOperator: jsii.String("GREATER_THAN"),
			Threshold:          jsii.Number(threshold),
			ThresholdType:      jsii.String("PERCENTAGE"),
		},
		Subscribers: &[]any{
			&awsbudgets.CfnBudget_SubscriberProperty{
				SubscriptionType: jsii.String("SNS"),
				Address:          topicArn,
			},
		},
	}
}

func contextString(stack awscdk.Stack, key, fallback string) string {
	v := stack.Node().TryGetContext(jsii.String(key))
	if s, ok := v.(string); ok {
		return s
	}
	return fallback
}

func contextNumber(stack awscdk.Stack, key string, fallback float64) float64 {
	switch v := stack.Node().TryGetContext(jsii.String(key)).(type) {
	case float64:
		return v
	case string:
		var out float64
		if _, err := fmt.Sscanf(v, "%f", &out); err == nil {
			return out
		}
	}
	return fallback
}

const killSwitchCode = `
const {
  CloudFrontClient,
  GetDistributionConfigCommand,
  UpdateDistributionCommand,
} = require("@aws-sdk/client-cloudfront");

exports.handler = async (event) => {
  const Id = process.env.DISTRIBUTION_ID;
  const client = new CloudFrontClient({});

  const current = await client.send(new GetDistributionConfigCommand({ Id }));
  if (!current.DistributionConfig.Enabled) {
    console.log("distribution already disabled");
    return { disabled: false, reason: "already disabled" };
  }

  current.DistributionConfig.Enabled = false;
  await client.send(new UpdateDistributionCommand({
    Id,
    IfMatch: current.ETag,
    DistributionConfig: current.DistributionConfig,
  }));

  console.log("distribution disabled", JSON.stringify(event));
  return { disabled: true };
};
`
