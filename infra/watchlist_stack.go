package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscertificatemanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudfront"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudfrontorigins"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatchactions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53targets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3deployment"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssecretsmanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssnssubscriptions"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// WatchlistStack runs the standalone watchlist app: same Docker image as
// brokers-sync started with -app=watchlist, its own Lambda, API Gateway,
// bucket and distribution, so a failure or deploy in one app never reaches
// the other. See BrokersSyncStack for the reasoning behind each piece; only
// the differences are commented here.

type WatchlistStack struct {
	awscdk.Stack
	DistributionId *string
}

func NewWatchlistStack(scope constructs.Construct, id string, props *awscdk.StackProps) *WatchlistStack {
	stack := awscdk.NewStack(scope, &id, props)

	originSecret := awssecretsmanager.NewSecret(stack, jsii.String("OriginVerifySecret"), &awssecretsmanager.SecretProps{
		SecretName:  jsii.String("/brokers-sync-watchlist/cloudfront-origin-verify"),
		Description: jsii.String("X-Origin-Verify header value shared between the watchlist CloudFront and API Gateway"),
		GenerateSecretString: &awssecretsmanager.SecretStringGenerator{
			ExcludePunctuation:   jsii.Bool(true),
			PasswordLength:       jsii.Number(32),
			GenerateStringKey:    jsii.String("v"),
			SecretStringTemplate: jsii.String(`{"v":""}`),
		},
	})
	secretValue := originSecret.SecretValueFromJson(jsii.String("v")).UnsafeUnwrap()

	// The table is shared with brokers-sync so a portfolio code works in both
	// apps. Imported by name rather than passed across stacks: a cross-stack
	// export would block any future change to the table in BrokersSyncStack.
	watchlistTable := awsdynamodb.Table_FromTableName(stack, jsii.String("WatchlistTable"), jsii.String("brokers-sync-watchlist"))

	lambdaFn := awslambda.NewDockerImageFunction(stack, jsii.String("BackendFn"), &awslambda.DockerImageFunctionProps{
		FunctionName: jsii.String("brokers-sync-watchlist-backend"),
		Code: awslambda.DockerImageCode_FromImageAsset(
			jsii.String(".."),
			&awslambda.AssetImageCodeProps{
				File: jsii.String("Dockerfile.lambda"),
				Cmd:  jsii.Strings("./server", "-addr", ":8080", "-app", "watchlist", "-web", "/app/web/dist"),
			},
		),
		Architecture: awslambda.Architecture_ARM_64(),
		MemorySize:   jsii.Number(256),
		Timeout:      awscdk.Duration_Seconds(jsii.Number(29)),
		Description:  jsii.String("brokers-sync watchlist app via Lambda Web Adapter"),
		Environment: &map[string]*string{
			"WATCHLIST_TABLE": watchlistTable.TableName(),
			"ORIGIN_VERIFY":   secretValue,
			// Hard cap on portfolio codes ever created. Raise it here and
			// redeploy; the counter in the table is never reset.
			"MAX_CODES": jsii.String("1000"),
		},
	})
	watchlistTable.GrantReadWriteData(lambdaFn)

	integration := awsapigateway.NewLambdaIntegration(lambdaFn, &awsapigateway.LambdaIntegrationOptions{
		Proxy: jsii.Bool(true),
	})

	// No binary media types: this app takes no uploads. CloudWatchRole is off
	// because the API Gateway account settings are an account-wide singleton
	// that BrokersSyncStack already owns; two stacks managing it would fight.
	api := awsapigateway.NewRestApi(stack, jsii.String("WatchlistApi"), &awsapigateway.RestApiProps{
		RestApiName:    jsii.String("brokers-sync-watchlist-api"),
		Description:    jsii.String("brokers-sync-standalone-watchlist-backend"),
		CloudWatchRole: jsii.Bool(false),
		DeployOptions: &awsapigateway.StageOptions{
			StageName:            jsii.String("prod"),
			ThrottlingRateLimit:  jsii.Number(10),
			ThrottlingBurstLimit: jsii.Number(20),
		},
	})
	api.Root().AddMethod(jsii.String("ANY"), integration, nil)
	api.Root().AddProxy(&awsapigateway.ProxyResourceOptions{
		DefaultIntegration: integration,
		AnyMethod:          jsii.Bool(true),
	})

	spaBucket := awss3.NewBucket(stack, jsii.String("SpaBucket"), &awss3.BucketProps{
		BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
		RemovalPolicy:     awscdk.RemovalPolicy_DESTROY,
		AutoDeleteObjects: jsii.Bool(true),
		Encryption:        awss3.BucketEncryption_S3_MANAGED,
	})

	oac := awscloudfront.NewS3OriginAccessControl(stack, jsii.String("SpaOAC"), &awscloudfront.S3OriginAccessControlProps{
		Description: jsii.String("OAC for brokers-sync watchlist SPA bucket"),
		Signing:     awscloudfront.Signing_SIGV4_NO_OVERRIDE(),
	})
	s3Origin := awscloudfrontorigins.S3BucketOrigin_WithOriginAccessControl(spaBucket, &awscloudfrontorigins.S3BucketOriginWithOACProps{
		OriginAccessControl: oac,
	})

	apiDomain := awscdk.Fn_Select(jsii.Number(2), awscdk.Fn_Split(jsii.String("/"), api.Url(), nil))
	apiOrigin := awscloudfrontorigins.NewHttpOrigin(apiDomain, &awscloudfrontorigins.HttpOriginProps{
		OriginPath:     jsii.String("/prod"),
		ProtocolPolicy: awscloudfront.OriginProtocolPolicy_HTTPS_ONLY,
		CustomHeaders: &map[string]*string{
			"X-Origin-Verify": secretValue,
		},
	})

	// The entry page is watchlist.html, not index.html: vite emits the HTML
	// under the name of its input file.
	spaRewrite := awscloudfront.NewFunction(stack, jsii.String("SpaRewrite"), &awscloudfront.FunctionProps{
		Runtime: awscloudfront.FunctionRuntime_JS_2_0(),
		Code: awscloudfront.FunctionCode_FromInline(jsii.String(`function handler(event) {
  var request = event.request;
  if (request.uri.indexOf('.') === -1) {
    request.uri = '/watchlist.html';
  }
  return request;
}`)),
	})

	// Reuses the *.dot-core.com wildcard certificate that also serves
	// brokersync.dot-core.com. No web ACL yet.
	distribution := awscloudfront.NewDistribution(stack, jsii.String("Distribution"), &awscloudfront.DistributionProps{
		Comment:           jsii.String("brokers-sync watchlist"),
		DefaultRootObject: jsii.String("watchlist.html"),
		DomainNames:       jsii.Strings("watchlist.dot-core.com"),
		Certificate: awscertificatemanager.Certificate_FromCertificateArn(stack, jsii.String("SpaCert"),
			jsii.String("arn:aws:acm:us-east-1:133874726017:certificate/995d1a4f-b536-4861-b8fb-b30c4e561ab7")),
		DefaultBehavior: &awscloudfront.BehaviorOptions{
			Origin:               s3Origin,
			ViewerProtocolPolicy: awscloudfront.ViewerProtocolPolicy_REDIRECT_TO_HTTPS,
			CachePolicy:          awscloudfront.CachePolicy_CACHING_OPTIMIZED(),
			FunctionAssociations: &[]*awscloudfront.FunctionAssociation{
				{
					Function:  spaRewrite,
					EventType: awscloudfront.FunctionEventType_VIEWER_REQUEST,
				},
			},
		},
		AdditionalBehaviors: &map[string]*awscloudfront.BehaviorOptions{
			"/api/*": {
				Origin:               apiOrigin,
				ViewerProtocolPolicy: awscloudfront.ViewerProtocolPolicy_REDIRECT_TO_HTTPS,
				AllowedMethods:       awscloudfront.AllowedMethods_ALLOW_ALL(),
				CachedMethods:        awscloudfront.CachedMethods_CACHE_GET_HEAD(),
				CachePolicy:          awscloudfront.CachePolicy_CACHING_DISABLED(),
				OriginRequestPolicy:  awscloudfront.OriginRequestPolicy_ALL_VIEWER_EXCEPT_HOST_HEADER(),
			},
		},
		ErrorResponses: &[]*awscloudfront.ErrorResponse{
			{
				HttpStatus:         jsii.Number(404),
				ResponseHttpStatus: jsii.Number(200),
				ResponsePagePath:   jsii.String("/watchlist.html"),
				Ttl:                awscdk.Duration_Seconds(jsii.Number(0)),
			},
		},
	})

	spaBucket.AddToResourcePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect:     awsiam.Effect_ALLOW,
		Principals: &[]awsiam.IPrincipal{awsiam.NewServicePrincipal(jsii.String("cloudfront.amazonaws.com"), nil)},
		Actions:    jsii.Strings("s3:GetObject", "s3:ListBucket"),
		Resources:  jsii.Strings(*spaBucket.ArnForObjects(jsii.String("*")), *spaBucket.BucketArn()),
		Conditions: &map[string]interface{}{
			"StringEquals": map[string]interface{}{
				"AWS:SourceArn": awscdk.Stack_Of(stack).FormatArn(&awscdk.ArnComponents{
					Service:      jsii.String("cloudfront"),
					Resource:     jsii.String("distribution"),
					ResourceName: distribution.DistributionId(),
					ArnFormat:    awscdk.ArnFormat_SLASH_RESOURCE_NAME,
				}),
			},
		},
	}))

	// dot-core.com is a Route 53 zone in this account. Imported by ID so synth
	// needs no lookup; unlike brokersync, whose records were made by hand,
	// these records are owned by this stack.
	zone := awsroute53.HostedZone_FromHostedZoneAttributes(stack, jsii.String("DotCoreZone"), &awsroute53.HostedZoneAttributes{
		HostedZoneId: jsii.String("Z04001592I3L4UGODH6WM"),
		ZoneName:     jsii.String("dot-core.com"),
	})
	target := awsroute53.RecordTarget_FromAlias(awsroute53targets.NewCloudFrontTarget(distribution))
	awsroute53.NewARecord(stack, jsii.String("AliasA"), &awsroute53.ARecordProps{
		Zone:       zone,
		RecordName: jsii.String("watchlist"),
		Target:     target,
	})
	awsroute53.NewAaaaRecord(stack, jsii.String("AliasAAAA"), &awsroute53.AaaaRecordProps{
		Zone:       zone,
		RecordName: jsii.String("watchlist"),
		Target:     target,
	})

	// ── Code creation alarm ───────────────────────────────────────────────────
	// The server logs one EMF line per created code (cmd/server/metrics.go).
	// A burst well above normal sign-ups means someone is draining the
	// MAX_CODES cap. Alarm actions must be in the alarm's region, so this
	// cannot reuse the cost guard's us-east-1 topic; it mails the same address.
	alertEmail := contextString(stack, "alertEmail", "brokersync@dot-core.com")
	if alertEmail == "" {
		panic("alertEmail context value must not be empty")
	}
	alertTopic := awssns.NewTopic(stack, jsii.String("WatchlistAlerts"), &awssns.TopicProps{
		DisplayName: jsii.String("brokers-sync watchlist alerts"),
	})
	alertTopic.AddSubscription(awssnssubscriptions.NewEmailSubscription(jsii.String(alertEmail), nil))

	codesAlarm := awscloudwatch.NewAlarm(stack, jsii.String("CodeCreationAlarm"), &awscloudwatch.AlarmProps{
		AlarmDescription: jsii.String("Unusual number of watchlist codes created in an hour — possible attempt to drain MAX_CODES"),
		Metric: awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:     jsii.String("BrokersSync/Watchlist"),
			MetricName:    jsii.String("CodesCreated"),
			DimensionsMap: &map[string]*string{"App": jsii.String("watchlist")},
			Statistic:     jsii.String("Sum"),
			Period:        awscdk.Duration_Hours(jsii.Number(1)),
		}),
		Threshold:          jsii.Number(contextNumber(stack, "codesPerHourAlarm", 50)),
		EvaluationPeriods:  jsii.Number(1),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_THRESHOLD,
		// No sign-ups in an hour is the normal case, not a fault.
		TreatMissingData: awscloudwatch.TreatMissingData_NOT_BREACHING,
	})
	codesAlarm.AddAlarmAction(awscloudwatchactions.NewSnsAction(alertTopic))

	awss3deployment.NewBucketDeployment(stack, jsii.String("SpaDeployment"), &awss3deployment.BucketDeploymentProps{
		Sources: &[]awss3deployment.ISource{
			awss3deployment.Source_Asset(jsii.String("../web/dist-watchlist"), nil),
		},
		DestinationBucket: spaBucket,
		Distribution:      distribution,
		DistributionPaths: jsii.Strings("/*"),
		MemoryLimit:       jsii.Number(256),
		Prune:             jsii.Bool(true),
	})

	awscdk.NewCfnOutput(stack, jsii.String("AppUrl"), &awscdk.CfnOutputProps{
		Value:       jsii.String("https://" + *distribution.DomainName()),
		Description: jsii.String("Watchlist CloudFront URL (served at https://watchlist.dot-core.com)"),
	})
	awscdk.NewCfnOutput(stack, jsii.String("AlertTopic"), &awscdk.CfnOutputProps{
		Value:       alertTopic.TopicArn(),
		Description: jsii.String("Confirm the email subscription or no alert is ever delivered"),
	})
	awscdk.NewCfnOutput(stack, jsii.String("ApiUrl"), &awscdk.CfnOutputProps{
		Value:       api.Url(),
		Description: jsii.String("Watchlist API Gateway URL (backend returns 403 without the X-Origin-Verify header)"),
	})

	return &WatchlistStack{Stack: stack, DistributionId: distribution.DistributionId()}
}
