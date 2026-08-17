package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscertificatemanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudfront"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudfrontorigins"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3deployment"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssecretsmanager"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// Limitations:
// - API Gateway has a 29s integration timeout — the /api/upload/zip SSE stream must
//   complete within 29s. Lambda Web Adapter buffers the full response; no true streaming.
// - API Gateway enforces a 10 MB max request payload. Upload zips must be < 10 MB.
// - Lambda sync response is capped at 6 MB. Total SSE event output must be < 6 MB.
// - Provisioned concurrency (1 instance) costs ~$11/month in eu-central-1.
//   Set ProvisionedConcurrentExecutions to 0 on the alias to eliminate this cost.

type BrokersSyncStackProps struct {
	awscdk.StackProps
}

// BrokersSyncStack exposes the distribution ID so the cost guard stack, which
// must live in us-east-1, can alarm on this distribution's metrics.
type BrokersSyncStack struct {
	awscdk.Stack
	DistributionId *string
}

func NewBrokersSyncStack(scope constructs.Construct, id string, props *BrokersSyncStackProps) *BrokersSyncStack {
	stack := awscdk.NewStack(scope, &id, &props.StackProps)

	// ── Secret ────────────────────────────────────────────────────────────────
	// Random token added by CloudFront as X-Origin-Verify on every request to
	// API Gateway. The API Gateway resource policy rejects requests without it,
	// preventing direct access that bypasses CloudFront.
	originSecret := awssecretsmanager.NewSecret(stack, jsii.String("OriginVerifySecret"), &awssecretsmanager.SecretProps{
		SecretName:  jsii.String("/brokers-sync/cloudfront-origin-verify"),
		Description: jsii.String("X-Origin-Verify header value shared between CloudFront and API Gateway"),
		GenerateSecretString: &awssecretsmanager.SecretStringGenerator{
			ExcludePunctuation:   jsii.Bool(true),
			PasswordLength:       jsii.Number(32),
			GenerateStringKey:    jsii.String("v"),
			SecretStringTemplate: jsii.String(`{"v":""}`),
		},
	})
	// UnsafeUnwrap writes the plaintext value into the CloudFormation template.
	// This is acceptable for a random bearer token with no other privileges.
	secretValue := originSecret.SecretValueFromJson(jsii.String("v")).UnsafeUnwrap()

	// ── DynamoDB (watchlist) ───────────────────────────────────────────────────
	// Provisioned 1/1 is a hard cost ceiling (~$0.47/month, inside the free
	// tier): excess traffic is throttled rather than billed. Autoscaling must
	// stay off, otherwise that ceiling disappears.
	watchlistTable := awsdynamodb.NewTable(stack, jsii.String("WatchlistTable"), &awsdynamodb.TableProps{
		TableName:           jsii.String("brokers-sync-watchlist"),
		PartitionKey:        &awsdynamodb.Attribute{Name: jsii.String("PK"), Type: awsdynamodb.AttributeType_STRING},
		SortKey:             &awsdynamodb.Attribute{Name: jsii.String("SK"), Type: awsdynamodb.AttributeType_STRING},
		BillingMode:         awsdynamodb.BillingMode_PROVISIONED,
		ReadCapacity:        jsii.Number(1),
		WriteCapacity:       jsii.Number(1),
		TimeToLiveAttribute: jsii.String("ttl"),
		RemovalPolicy:       awscdk.RemovalPolicy_RETAIN,
	})

	// ── Lambda (Docker) ───────────────────────────────────────────────────────
	// CDK builds Dockerfile.lambda (at repo root, one level above infra/),
	// auto-creates an ECR repository, and pushes the image on cdk deploy.
	lambdaFn := awslambda.NewDockerImageFunction(stack, jsii.String("BackendFn"), &awslambda.DockerImageFunctionProps{
		FunctionName: jsii.String("brokers-sync-backend"),
		Code: awslambda.DockerImageCode_FromImageAsset(
			jsii.String(".."), // repo root relative to infra/
			&awslambda.AssetImageCodeProps{
				File: jsii.String("Dockerfile.lambda"),
			},
		),
		Architecture: awslambda.Architecture_ARM_64(),
		MemorySize:   jsii.Number(256),
		Timeout:      awscdk.Duration_Seconds(jsii.Number(29)), // matches API GW integration timeout
		Description:  jsii.String("brokers-sync Go backend via Lambda Web Adapter"),
		Environment: &map[string]*string{
			"WATCHLIST_TABLE": watchlistTable.TableName(),
		},
		// ReservedConcurrentExecutions is deliberately unset. Lambda requires at
		// least 50 unreserved concurrent executions to remain account-wide, and
		// this account's limit is 50, so any reservation is rejected outright.
		// The 10 rps API Gateway throttle is the binding constraint on compute
		// anyway; raise the account concurrency quota first if a hard per-function
		// cap is ever wanted.
	})
	watchlistTable.GrantReadWriteData(lambdaFn)

	// Provisioned concurrency requires a version + alias.
	// API Gateway integrates with the alias so provisioned instances are used.
	version := lambdaFn.CurrentVersion()
	alias := awslambda.NewAlias(stack, jsii.String("BackendAlias"), &awslambda.AliasProps{
		AliasName: jsii.String("live"),
		Version:   version,
	})

	// ── API Gateway ───────────────────────────────────────────────────────────
	integration := awsapigateway.NewLambdaIntegration(alias, &awsapigateway.LambdaIntegrationOptions{
		Proxy: jsii.Bool(true),
	})

	api := awsapigateway.NewRestApi(stack, jsii.String("BrokersSyncApi"), &awsapigateway.RestApiProps{
		RestApiName: jsii.String("brokers-sync-api"),
		Description: jsii.String("brokers-sync backend"),
		// Required for multipart/form-data uploads: API Gateway must treat the body as
		// binary so it base64-encodes it in the Lambda event instead of corrupting it
		// by trying to UTF-8 decode the binary zip payload.
		BinaryMediaTypes: jsii.Strings("multipart/form-data", "application/octet-stream"),
		DeployOptions: &awsapigateway.StageOptions{
			StageName:            jsii.String("prod"),
			ThrottlingRateLimit:  jsii.Number(10), // 10 req/s steady state
			ThrottlingBurstLimit: jsii.Number(20), // burst up to 20 concurrent
		},
	})

	// Proxy all paths (root + wildcard) to the Lambda alias.
	// LWA forwards the full path, so /api/upload/zip reaches the Go mux unchanged.
	api.Root().AddMethod(jsii.String("ANY"), integration, nil)
	api.Root().AddProxy(&awsapigateway.ProxyResourceOptions{
		DefaultIntegration: integration,
		AnyMethod:          jsii.Bool(true),
	})

	// Usage plan enforces throttle limits independently of stage-level settings.
	plan := api.AddUsagePlan(jsii.String("UsagePlan"), &awsapigateway.UsagePlanProps{
		Name: jsii.String("brokers-sync-plan"),
		Throttle: &awsapigateway.ThrottleSettings{
			RateLimit:  jsii.Number(10),
			BurstLimit: jsii.Number(20),
		},
	})
	plan.AddApiStage(&awsapigateway.UsagePlanPerApiStage{
		Api:   api,
		Stage: api.DeploymentStage(),
	})

	// ── S3 bucket for React SPA ───────────────────────────────────────────────
	spaBucket := awss3.NewBucket(stack, jsii.String("SpaBucket"), &awss3.BucketProps{
		BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
		RemovalPolicy:     awscdk.RemovalPolicy_DESTROY,
		AutoDeleteObjects: jsii.Bool(true),
		Encryption:        awss3.BucketEncryption_S3_MANAGED,
	})

	// ── CloudFront ────────────────────────────────────────────────────────────
	oac := awscloudfront.NewS3OriginAccessControl(stack, jsii.String("SpaOACV2"), &awscloudfront.S3OriginAccessControlProps{
		Description: jsii.String("OAC for brokers-sync SPA bucket"),
		Signing:     awscloudfront.Signing_SIGV4_NO_OVERRIDE(),
	})

	s3Origin := awscloudfrontorigins.S3BucketOrigin_WithOriginAccessControl(
		spaBucket,
		&awscloudfrontorigins.S3BucketOriginWithOACProps{
			OriginAccessControl: oac,
		},
	)

	// Extract the API Gateway hostname from its URL token.
	// api.Url() resolves to "https://{id}.execute-api.eu-central-1.amazonaws.com/prod/"
	// Splitting on "/" and selecting index 2 gives the bare hostname.
	apiDomain := awscdk.Fn_Select(
		jsii.Number(2),
		awscdk.Fn_Split(jsii.String("/"), api.Url(), nil),
	)

	apiOrigin := awscloudfrontorigins.NewHttpOrigin(apiDomain, &awscloudfrontorigins.HttpOriginProps{
		OriginPath:     jsii.String("/prod"),
		ProtocolPolicy: awscloudfront.OriginProtocolPolicy_HTTPS_ONLY,
		// CloudFront adds this header on every request forwarded to API Gateway.
		// The API Gateway resource policy allows only requests that carry it.
		CustomHeaders: &map[string]*string{
			"X-Origin-Verify": secretValue,
		},
	})

	// Deep links such as /watchlist are React Router paths, not S3 keys. S3 has
	// no object of that name and answers 403 (not 404, which it reserves for
	// callers allowed to list the bucket), so the SPA fallback below never
	// fires and the browser gets raw AccessDenied XML. Rewriting the URI before
	// the origin request means S3 is only ever asked for files that exist.
	// Attached to the default behaviour only, so /api/* is untouched.
	spaRewrite := awscloudfront.NewFunction(stack, jsii.String("SpaRewrite"), &awscloudfront.FunctionProps{
		Runtime: awscloudfront.FunctionRuntime_JS_2_0(),
		Code: awscloudfront.FunctionCode_FromInline(jsii.String(`function handler(event) {
  var request = event.request;
  if (request.uri.indexOf('.') === -1) {
    request.uri = '/index.html';
  }
  return request;
}`)),
	})

	distribution := awscloudfront.NewDistribution(stack, jsii.String("Distribution"), &awscloudfront.DistributionProps{
		Comment:           jsii.String("brokers-sync"),
		DefaultRootObject: jsii.String("index.html"),
		// The alias and its certificate were attached in the console. Every
		// CloudFormation update ships the full distribution config, so leaving
		// them out here drops the alias and the site starts answering 403 to
		// anything addressed to the custom domain. CloudFront only accepts
		// us-east-1 certificates, whatever region the rest of the stack uses.
		DomainNames: jsii.Strings("brokersync.dot-core.com"),
		Certificate: awscertificatemanager.Certificate_FromCertificateArn(stack, jsii.String("SpaCert"),
			jsii.String("arn:aws:acm:us-east-1:133874726017:certificate/995d1a4f-b536-4861-b8fb-b30c4e561ab7")),
		// Core protections were switched on in the CloudFront console, which
		// provisioned this web ACL and a pricing plan subscription. CloudFormation
		// sends the whole distribution config on every update, so omitting the ACL
		// reads as removing it and CloudFront rejects the deploy outright. Keep
		// this in step with the console: toggling protections there mints a new
		// ACL and this ARN goes stale.
		WebAclId: jsii.String("arn:aws:wafv2:us-east-1:133874726017:global/webacl/CreatedByCloudFront-5178bc85/806b32e7-6ade-4446-accf-db7b7788f611"),
		// Default behavior: serve the React SPA from S3.
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
		// /api/* behavior: proxy to API Gateway with the secret header.
		AdditionalBehaviors: &map[string]*awscloudfront.BehaviorOptions{
			"/api/*": {
				Origin:               apiOrigin,
				ViewerProtocolPolicy: awscloudfront.ViewerProtocolPolicy_REDIRECT_TO_HTTPS,
				AllowedMethods:       awscloudfront.AllowedMethods_ALLOW_ALL(),
				CachedMethods:        awscloudfront.CachedMethods_CACHE_GET_HEAD(),
				// No caching — SSE and file uploads must never be cached.
				CachePolicy: awscloudfront.CachePolicy_CACHING_DISABLED(),
				// Forward request body, headers, and query strings to API Gateway.
				OriginRequestPolicy: awscloudfront.OriginRequestPolicy_ALL_VIEWER_EXCEPT_HOST_HEADER(),
			},
		},
		// No price class: the distribution is on the Free pricing plan, which
		// rejects the field outright. Restricting edge locations to EU + US
		// requires moving off that plan first.
		// SPA routing: S3 returns 404 for unknown paths → serve index.html so
		// React Router handles client-side navigation.
		// 403 is intentionally NOT mapped: API Gateway errors must reach the browser
		// as real status codes, not silently replaced with HTML.
		ErrorResponses: &[]*awscloudfront.ErrorResponse{
			{
				HttpStatus:         jsii.Number(404),
				ResponseHttpStatus: jsii.Number(200),
				ResponsePagePath:   jsii.String("/index.html"),
				Ttl:                awscdk.Duration_Seconds(jsii.Number(0)),
			},
		},
	})

	// Grant CloudFront OAC permission to read from S3 via bucket resource policy.
	// ListBucket is required for SPA deep links: without it S3 answers a missing
	// key with 403 rather than 404, and only 404 is mapped to index.html, so
	// /watchlist would return raw AccessDenied XML instead of the app.
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

	// ── SPA deployment ────────────────────────────────────────────────────────
	// Syncs web/dist (built before cdk deploy in CI) to S3 and invalidates
	// the CloudFront cache so users immediately see the new version.
	awss3deployment.NewBucketDeployment(stack, jsii.String("SpaDeployment"), &awss3deployment.BucketDeploymentProps{
		Sources: &[]awss3deployment.ISource{
			awss3deployment.Source_Asset(jsii.String("../web/dist"), nil),
		},
		DestinationBucket: spaBucket,
		Distribution:      distribution,
		DistributionPaths: jsii.Strings("/*"),
		MemoryLimit:       jsii.Number(256),
		Prune:             jsii.Bool(true),
	})

	// ── Outputs ───────────────────────────────────────────────────────────────
	awscdk.NewCfnOutput(stack, jsii.String("AppUrl"), &awscdk.CfnOutputProps{
		Value:       jsii.String("https://" + *distribution.DomainName()),
		Description: jsii.String("CloudFront URL — open this in your browser"),
	})
	awscdk.NewCfnOutput(stack, jsii.String("ApiUrl"), &awscdk.CfnOutputProps{
		Value:       api.Url(),
		Description: jsii.String("API Gateway URL (direct access blocked without X-Origin-Verify header)"),
	})

	return &BrokersSyncStack{Stack: stack, DistributionId: distribution.DistributionId()}
}
