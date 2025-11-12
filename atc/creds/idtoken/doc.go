// Package idtoken provides an OIDC ID Token credential provider for Concourse pipelines.
//
// The IDToken provider generates short-lived JWT tokens that can be used to authenticate
// with cloud providers (AWS, GCP, Azure) using OIDC federation, eliminating the need for
// static credentials.
//
// # OIDC Issuer Configuration
//
// For private network deployments, the OIDC issuer URL can be configured separately from
// the Concourse external URL using the --oidc-issuer-url flag:
//
//	concourse web \
//	  --external-url https://concourse.internal.example.com \
//	  --oidc-issuer-url https://oidc.example.com
//
// When set, generated tokens will use the OIDC issuer URL in the 'iss' claim, and the
// .well-known/openid-configuration endpoint will return this URL.
//
// IMPORTANT: The OIDC issuer URL must be publicly accessible for cloud provider verification.
// The .well-known/jwks.json and .well-known/openid-configuration endpoints must be reachable
// at this URL.
//
// # Usage in Pipelines
//
// Configure an IDToken var source in your pipeline:
//
//	var_sources:
//	- name: aws-token
//	  type: idtoken
//	  config:
//	    audience: ["sts.amazonaws.com"]
//
// Then use the token in tasks:
//
//	jobs:
//	- name: deploy
//	  plan:
//	  - task: authenticate
//	    config:
//	      run:
//	        path: aws
//	        args:
//	        - sts
//	        - assume-role-with-web-identity
//	        - --web-identity-token
//	        - ((aws-token.token))
//
// # Token Configuration
//
// The IDToken provider supports the following configuration options:
//
//   - audience: List of token audiences (required for most cloud providers)
//   - subject_scope: Scope of the subject claim (pipeline, team, system)
//   - expires_in: Token expiration duration (default: 1h, max: 24h)
//   - algorithm: Signing algorithm (RS256 or ES256, default: RS256)
//
package idtoken
