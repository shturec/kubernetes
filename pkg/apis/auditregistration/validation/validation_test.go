/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package validation

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/apis/auditregistration"
	utilpointer "k8s.io/utils/pointer"
)

func TestValidateAuditSink(t *testing.T) {
	testQPS := int64(10)
	testURL := "http://localhost"
	testCases := []struct {
		name   string
		conf   auditregistration.AuditSink
		numErr int
	}{
		{
			name: "should pass full config",
			conf: auditregistration.AuditSink{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myconf",
				},
				Spec: auditregistration.AuditSinkSpec{
					Policy: auditregistration.Policy{
						Level: auditregistration.LevelRequest,
					},
					Webhook: auditregistration.Webhook{
						Throttle: &auditregistration.WebhookThrottleConfig{
							QPS: &testQPS,
						},
						ClientConfig: auditregistration.WebhookClientConfig{
							URL: &testURL,
						},
					},
				},
			},
			numErr: 0,
		},
		{
			name: "should fail no policy",
			conf: auditregistration.AuditSink{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myconf",
				},
				Spec: auditregistration.AuditSinkSpec{
					Webhook: auditregistration.Webhook{
						ClientConfig: auditregistration.WebhookClientConfig{
							URL: &testURL,
						},
					},
				},
			},
			numErr: 1,
		},
		{
			name: "should fail no webhook",
			conf: auditregistration.AuditSink{
				ObjectMeta: metav1.ObjectMeta{
					Name: "myconf",
				},
				Spec: auditregistration.AuditSinkSpec{
					Policy: auditregistration.Policy{
						Level: auditregistration.LevelMetadata,
					},
				},
			},
			numErr: 1,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			errs := ValidateAuditSink(&test.conf)
			require.Len(t, errs, test.numErr)
		})
	}
}

func TestValidatePolicy(t *testing.T) {
	successCases := []auditregistration.Policy{}
	successCases = append(successCases, auditregistration.Policy{ // Policy with level only
		Level: auditregistration.LevelNone,
	})
	successCases = append(successCases, auditregistration.Policy{ // Policy with level and rule
		Level: auditregistration.LevelRequest,
		Rules: []auditregistration.PolicyRule{
			{
				WithAuditClass: "sample",
				Level:          auditregistration.LevelRequest,
			},
		},
	})

	for i, policy := range successCases {
		if errs := ValidatePolicy(policy, field.NewPath("policy")); len(errs) != 0 {
			t.Errorf("[%d] Expected policy %#v to be valid: %v", i, policy, errs)
		}
	}

	errorCases := []auditregistration.Policy{}
	errorCases = append(errorCases, auditregistration.Policy{}) // Policy with missing level
	errorCases = append(errorCases, auditregistration.Policy{   // Policy with rule and without global level
		Rules: []auditregistration.PolicyRule{
			{
				WithAuditClass: "test",
				Level:          auditregistration.LevelRequest,
			},
		},
	})
	errorCases = append(errorCases, auditregistration.Policy{Level: auditregistration.Level("invalid")}) // Policy with bad level
	errorCases = append(errorCases, auditregistration.Policy{                                            // PolicyRule with bad level
		Level: auditregistration.LevelRequest,
		Rules: []auditregistration.PolicyRule{
			{
				WithAuditClass: "test",
				Level:          auditregistration.Level("invalid"),
			},
		},
	})
	errorCases = append(errorCases, auditregistration.Policy{ // PolicyRule without level
		Level: auditregistration.LevelRequest,
		Rules: []auditregistration.PolicyRule{
			{
				WithAuditClass: "test",
			},
		},
	})
	errorCases = append(errorCases, auditregistration.Policy{ // PolicyRule rule without class reference
		Level: auditregistration.LevelRequest,
		Rules: []auditregistration.PolicyRule{
			{
				Level: auditregistration.Level("invalid"),
			},
		},
	})

	for i, policy := range errorCases {
		if errs := ValidatePolicy(policy, field.NewPath("policy")); len(errs) == 0 {
			t.Errorf("[%d] Expected policy %#v to be invalid!", i, policy)
		}
	}
}

func TestAuditClass(t *testing.T) {

	validClass := auditregistration.AuditClass{
		Spec: auditregistration.AuditClassSpec{
			Rules: []auditregistration.Rule{
				{ // Matching non-humans
					BaseSelector: auditregistration.BaseSelector {
						UserGroups: []string{"system:serviceaccounts", "system:nodes"},
					},
				},
				{ // Matching user accounts
					BaseSelector: auditregistration.BaseSelector {
						Users: []string{"system:administrator"},
					},
				},
				{ // Matching user accounts
					BaseSelector: auditregistration.BaseSelector {
						Users: []string{"system:administrator"},
					},
				},
				{ // Matching verbs
					BaseSelector: auditregistration.BaseSelector {
						Verbs: []string{"create", "update"},
					},
				},
				{ // Matching combination
					BaseSelector: auditregistration.BaseSelector {
						Verbs: []string{"delete"},
						Users: []string{"system:administrator"},
					},
				},
				{ // Specific request for namespaced resources
					BaseSelector: auditregistration.BaseSelector {
						Verbs: []string{"get"},
					},
					RequestSelector: auditregistration.RequestSelector{
						GroupResourceSelectors: []auditregistration.GroupResourceSelector{
							{
								Group: "rbac.authorization.k8s.io", 
								Resources: []auditregistration.ResourceSelector {
									{
										Kind: "roles", 
									},
									{
										Kind:"rolebindings",
									},
								},
								Scope: auditregistration.ScopeSelector{
									Scope: "namespace",
									Namespaces: []auditregistration.NamespaceSelector{"kube-system"},
								},
							},
						},
					},
				},
				{ // Specific request for cluster resource
					BaseSelector: auditregistration.BaseSelector {
						Verbs: []string{"get"},
					},
					RequestSelector: auditregistration.RequestSelector{
						GroupResourceSelectors: []auditregistration.GroupResourceSelector{
							{
								Group: "rbac.authorization.k8s.io", 
								Resources: []auditregistration.ResourceSelector {
									{
										Kind: "clusterrolebindings", 
									},
								},
								Scope: auditregistration.ScopeSelector{
									Scope: "cluster",
								},
							},
						},
					},
				},
				{ // Some non-resource URLs
					BaseSelector: auditregistration.BaseSelector {
						UserGroups: []string{"developers"},
					},
					RequestSelector: auditregistration.RequestSelector{
						NonResourceSelectors: []auditregistration.NonResourceSelector{
							{
								URLs: []string{
									"/logs*",
									"/healthz*",
									"/metrics",
									"*",
								},
							},
						},
					},					
				},
			},
		},
	}

	if errs := ValidateAuditClass(validClass); len(errs) != 0 {
		t.Errorf("Expected class %#v to be valid: %v", validClass, errs)
	}

	invalidClass := auditregistration.AuditClass{
		Spec: auditregistration.AuditClassSpec{
			Rules: []auditregistration.Rule{
				{ // invalid group name
					RequestSelector: auditregistration.RequestSelector{
						GroupResourceSelectors: []auditregistration.GroupResourceSelector{
							{
								Group: "rbac.authorization.k8s.io/v1beta1", 
								Resources: []auditregistration.ResourceSelector {
									{
										Kind: "roles", 
									},
								},
							},
						},
					},
				},
				{ // ObjectNames without Resources
					BaseSelector: auditregistration.BaseSelector {
						Verbs: []string{"get"},
					},
					RequestSelector: auditregistration.RequestSelector{
						GroupResourceSelectors: []auditregistration.GroupResourceSelector{
							{
								Group: "rbac.authorization.k8s.io", 
								Resources: []auditregistration.ResourceSelector {
									{
										ObjectNames: []string{"leader"},
									},
								},
								Scope: auditregistration.ScopeSelector{
									Scope: "namespace",
									Namespaces: []auditregistration.NamespaceSelector{"kube-system"},
								},
							},
						},
					},
				},
				{
					RequestSelector: auditregistration.RequestSelector{
						NonResourceSelectors: []auditregistration.NonResourceSelector{
							{ // invalid non-resource URLs
								URLs: []string{
									"logs",
									"/healthz*",
								},
							},
							{ // empty non-resource URLs
								URLs: []string{
									"",
									"/healthz*",
								},
							},
							{ // invalid non-resource URLs with multi "*"
								URLs: []string{
									"/logs/*/*",
									"/metrics",
								},
							},
							{ // invalid non-resrouce URLs with "*" not in the end
								URLs: []string{
									"/logs/*.log",
									"/metrics",
								},
							},
						},
					},
				},
			},
		},
	}

	if errs := ValidateAuditClass(invalidClass); len(errs) == 0 {
		t.Errorf("Expected class %#v to be invalid!", invalidClass)
	}
}

func strPtr(s string) *string { return &s }

func TestValidateWebhook(t *testing.T) {
	tests := []struct {
		name          string
		config        auditregistration.Webhook
		expectedError string
	}{
		{
			name: "both service and URL missing",
			config: auditregistration.Webhook{
				ClientConfig: auditregistration.WebhookClientConfig{},
			},
			expectedError: `exactly one of`,
		},
		{
			name: "both service and URL provided",
			config: auditregistration.Webhook{
				ClientConfig: auditregistration.WebhookClientConfig{
					Service: &auditregistration.ServiceReference{
						Namespace: "ns",
						Name:      "n",
						Port:      443,
					},
					URL: utilpointer.StringPtr("example.com/k8s/webhook"),
				},
			},
			expectedError: `webhook.clientConfig: Required value: exactly one of url or service is required`,
		},
		{
			name: "blank URL",
			config: auditregistration.Webhook{
				ClientConfig: auditregistration.WebhookClientConfig{
					URL: utilpointer.StringPtr(""),
				},
			},
			expectedError: `webhook.clientConfig.url: Invalid value: "": host must be provided`,
		},
		{
			name: "missing host",
			config: auditregistration.Webhook{
				ClientConfig: auditregistration.WebhookClientConfig{
					URL: utilpointer.StringPtr("https:///fancy/webhook"),
				},
			},
			expectedError: `host must be provided`,
		},
		{
			name: "fragment",
			config: auditregistration.Webhook{
				ClientConfig: auditregistration.WebhookClientConfig{
					URL: utilpointer.StringPtr("https://example.com/#bookmark"),
				},
			},
			expectedError: `"bookmark": fragments are not permitted`,
		},
		{
			name: "query",
			config: auditregistration.Webhook{
				ClientConfig: auditregistration.WebhookClientConfig{
					URL: utilpointer.StringPtr("https://example.com?arg=value"),
				},
			},
			expectedError: `"arg=value": query parameters are not permitted`,
		},
		{
			name: "user",
			config: auditregistration.Webhook{
				ClientConfig: auditregistration.WebhookClientConfig{
					URL: utilpointer.StringPtr("https://harry.potter@example.com/"),
				},
			},
			expectedError: `"harry.potter": user information is not permitted`,
		},
		{
			name: "just totally wrong",
			config: auditregistration.Webhook{
				ClientConfig: auditregistration.WebhookClientConfig{
					URL: utilpointer.StringPtr("arg#backwards=thisis?html.index/port:host//:https"),
				},
			},
			expectedError: `host must be provided`,
		},
		{
			name: "path must start with slash",
			config: auditregistration.Webhook{
				ClientConfig: auditregistration.WebhookClientConfig{
					Service: &auditregistration.ServiceReference{
						Namespace: "ns",
						Name:      "n",
						Path:      utilpointer.StringPtr("foo/"),
						Port:      443,
					},
				},
			},
			expectedError: `clientConfig.service.path: Invalid value: "foo/": must start with a '/'`,
		},
		{
			name: "invalid port >65535",
			config: auditregistration.Webhook{
				ClientConfig: auditregistration.WebhookClientConfig{
					Service: &auditregistration.ServiceReference{
						Namespace: "ns",
						Name:      "n",
						Path:      utilpointer.StringPtr("foo/"),
						Port:      65536,
					},
				},
			},
			expectedError: `Invalid value: 65536: port is not valid: must be between 1 and 65535, inclusive`,
		},
		{
			name: "invalid port 0",
			config: auditregistration.Webhook{
				ClientConfig: auditregistration.WebhookClientConfig{
					Service: &auditregistration.ServiceReference{
						Namespace: "ns",
						Name:      "n",
						Path:      utilpointer.StringPtr("foo/"),
						Port:      0,
					},
				},
			},
			expectedError: `Invalid value: 0: port is not valid: must be between 1 and 65535, inclusive`,
		},
		{
			name: "path accepts slash",
			config: auditregistration.Webhook{
				ClientConfig: auditregistration.WebhookClientConfig{
					Service: &auditregistration.ServiceReference{
						Namespace: "ns",
						Name:      "n",
						Path:      utilpointer.StringPtr("/"),
						Port:      443,
					},
				},
			},
			expectedError: ``,
		},
		{
			name: "path accepts no trailing slash",
			config: auditregistration.Webhook{
				ClientConfig: auditregistration.WebhookClientConfig{
					Service: &auditregistration.ServiceReference{
						Namespace: "ns",
						Name:      "n",
						Path:      utilpointer.StringPtr("/foo"),
						Port:      443,
					},
				},
			},
			expectedError: ``,
		},
		{
			name: "path fails //",
			config: auditregistration.Webhook{
				ClientConfig: auditregistration.WebhookClientConfig{
					Service: &auditregistration.ServiceReference{
						Namespace: "ns",
						Name:      "n",
						Path:      utilpointer.StringPtr("//"),
						Port:      443,
					},
				},
			},
			expectedError: `clientConfig.service.path: Invalid value: "//": segment[0] may not be empty`,
		},
		{
			name: "path no empty step",
			config: auditregistration.Webhook{
				ClientConfig: auditregistration.WebhookClientConfig{
					Service: &auditregistration.ServiceReference{
						Namespace: "ns",
						Name:      "n",
						Path:      utilpointer.StringPtr("/foo//bar/"),
						Port:      443,
					},
				},
			},
			expectedError: `clientConfig.service.path: Invalid value: "/foo//bar/": segment[1] may not be empty`,
		}, {
			name: "path no empty step 2",
			config: auditregistration.Webhook{
				ClientConfig: auditregistration.WebhookClientConfig{
					Service: &auditregistration.ServiceReference{
						Namespace: "ns",
						Name:      "n",
						Path:      utilpointer.StringPtr("/foo/bar//"),
						Port:      443,
					},
				},
			},
			expectedError: `clientConfig.service.path: Invalid value: "/foo/bar//": segment[2] may not be empty`,
		},
		{
			name: "path no non-subdomain",
			config: auditregistration.Webhook{
				ClientConfig: auditregistration.WebhookClientConfig{
					Service: &auditregistration.ServiceReference{
						Namespace: "ns",
						Name:      "n",
						Path:      utilpointer.StringPtr("/apis/foo.bar/v1alpha1/--bad"),
						Port:      443,
					},
				},
			},
			expectedError: `clientConfig.service.path: Invalid value: "/apis/foo.bar/v1alpha1/--bad": segment[3]: a DNS-1123 subdomain`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := ValidateWebhook(test.config, field.NewPath("webhook"))
			err := errs.ToAggregate()
			if err != nil {
				if e, a := test.expectedError, err.Error(); !strings.Contains(a, e) || e == "" {
					t.Errorf("expected to contain \nerr: %s \ngot: %s", e, a)
				}
			} else {
				if test.expectedError != "" {
					t.Errorf("unexpected no error, expected to contain %s", test.expectedError)
				}
			}
		})
	}
}
