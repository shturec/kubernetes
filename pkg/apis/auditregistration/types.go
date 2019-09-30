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

// +k8s:openapi-gen=true

package auditregistration

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Level defines the amount of information logged during auditing
type Level string

// Valid audit levels
const (
	// LevelNone disables auditing
	LevelNone Level = "None"
	// LevelMetadata provides the basic level of auditing.
	LevelMetadata Level = "Metadata"
	// LevelRequest provides Metadata level of auditing, and additionally
	// logs the request object (does not apply for non-resource requests).
	LevelRequest Level = "Request"
	// LevelRequestResponse provides Request level of auditing, and additionally
	// logs the response object (does not apply for non-resource requests and watches).
	LevelRequestResponse Level = "RequestResponse"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AuditSink represents a cluster level audit sink
type AuditSink struct {
	metav1.TypeMeta
	// +optional
	metav1.ObjectMeta

	// `spec` defines the `auditSink` spec
	Spec AuditSinkSpec
}

// AuditSinkSpec holds the spec for the audit sink
type AuditSinkSpec struct {
	// `policy` defines the policy for selecting which events should be sent to the webhook
	// required
	Policy Policy

	// `webhook` to send events
	// required
	Webhook Webhook
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AuditSinkList is a list of AuditSink items.
type AuditSinkList struct {
	metav1.TypeMeta
	// +optional
	metav1.ListMeta

	// `items` is a list of `auditSink` configurations.
	Items []AuditSink
}

// Policy defines the configuration of how audit events are logged
type Policy struct {
	// `level` is the base level that all requests are recorded at.
	// available options: None, Metadata, Request, RequestResponse
	// required
	Level Level

	// `rules` define how `auditClass` objects should be handled.
	// A request may fall under multiple `auditClass` objects.
	// Rules are evaluated in order (first matching wins).
	// Rules take precedence over the the parent `level`.
	// Unmatched requests use the parent `level` rule.
	// +optional
	Rules []PolicyRule
}

// PolicyRule defines what level is recorded for an `auditClass`.
type PolicyRule struct {
	// `withAuditClass` is the name of the `auditClass` object. This rule matches requests that are
	// classified with this `auditClass`
	// required
	WithAuditClass string

	// `level` that all requests this rule applies to are recorded at.
	// available options: None, Metadata, Request, RequestResponse
	// This will override the parent policy Level.
	// required
	Level Level
}

// Webhook holds the configuration of the webhook.
type Webhook struct {
	// `throttle` holds the options for throttling the webhook.
	// +optional
	Throttle *WebhookThrottleConfig

	// `clientConfig` holds the connection parameters for the webhook.
	// required
	ClientConfig WebhookClientConfig
}

// WebhookThrottleConfig holds the configuration for throttling events.
type WebhookThrottleConfig struct {
	// `qps` maximum number of batches per second.
	// default 10 QPS
	// +optional
	QPS *int64

	// `burst` is the maximum number of events sent at the same moment.
	// default 15 QPS
	// +optional
	Burst *int64
}

// WebhookClientConfig contains the information to make a connection with the webhook
type WebhookClientConfig struct {
	// `url` gives the location of the webhook, in standard URL form
	// (`scheme://host:port/path`). Exactly one of `url` or `service`
	// must be specified.
	//
	// The `host` should not refer to a service running in the cluster; use
	// the `service` field instead. The host might be resolved via external
	// DNS in some apiservers (e.g., `kube-apiserver` cannot resolve
	// in-cluster DNS as that would be a layering violation). `host` may
	// also be an IP address.
	//
	// Please note that using `localhost` or `127.0.0.1` as a `host` is
	// risky unless you take great care to run this webhook on all hosts
	// which run an apiserver which might need to make calls to this
	// webhook. Such installs are likely to be non-portable, i.e., not easy
	// to turn up in a new cluster.
	//
	// The scheme must be "https"; the URL must begin with "https://".
	//
	// A path is optional, and if present may be any string permissible in
	// a URL. You may use the path to pass an arbitrary string to the
	// webhook, for example, a cluster identifier.
	//
	// Attempting to use a user or basic auth e.g. "user:password@" is not
	// allowed. Fragments ("#...") and query parameters ("?...") are not
	// allowed, either.
	//
	// +optional
	URL *string

	// `service` is a reference to the service for this webhook. Either
	// `service` or `url` must be specified.
	//
	// If the webhook is running within the cluster, then you should use `service`.
	//
	// +optional
	Service *ServiceReference

	// `caBundle` is a PEM encoded CA bundle which will be used to validate the webhook's server certificate.
	// If unspecified, system trust roots on the apiserver are used.
	// +optional
	CABundle []byte
}

// ServiceReference holds a reference to Service.legacy.k8s.io
type ServiceReference struct {
	// `namespace` is the namespace of the service.
	// Required
	Namespace string

	// `name` is the name of the service.
	// Required
	Name string

	// `path` is an optional URL path which will be sent in any request to
	// this service.
	// +optional
	Path *string

	// If specified, the port on the service that hosting webhook.
	// `port` should be a valid port number (1-65535, inclusive).
	// +optional
	Port int32
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AuditClass is a set of rules that categorize requests
//
// This should be considered a highly privileged object, as modifying it
// will change what is logged.
type AuditClass struct {
	metav1.TypeMeta
	// +optional
	metav1.ObjectMeta

	// `spec` is the spec for the `auditClass`.
	Spec AuditClassSpec
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AuditClassList is a list of `auditClass` items.
type AuditClassList struct {
	metav1.TypeMeta
	// +optional
	metav1.ListMeta

	// `items` is a list of `auditClass` objects.
	Items []AuditClass
}

// AuditClassSpec is the spec for the `auditClass` object.
type AuditClassSpec struct {
	// `requestSelectors` defines a list of `requestSelector` objects. Only one must match.
	// +optional
	RequestSelectors []RequestSelector
	// +optional
	ResourceSelectors []ResourceSelector
	// +optional
	ClusterResourceSelectors []ClusterResourceSelector
	// +optional
	NonResourceSelectors []NonResourceSelector
}

// RequestSelector selects requests by matching on the given fields. Selectors are
// used to compose `auditClass` objects.
type RequestSelector struct {
	// `users` (by authenticated user name) in this selector.
	// An empty list implies every user.
	// This list should not exceed 100 items, if you need more, use another request selector.
	// +optional
	Users []string

	// `userGroups` in this selector. A user is considered matching
	// if it is a member of any of the `userGroups`.
	// An empty list implies every user group.
	// +optional
	UserGroups []string

	// `verbs` included in this selector.
	// An empty list implies every verb.
	// +optional
	Verbs []string
}
type ResourceSelector struct {
	// +optional
	RequestSelector

	// `namespaces` in this selector.
	// The empty string "" matches non-namespaced resources.
	// An empty list implies every namespace.
	// Non-namespaced resources will be matched if and only if the empty string is present in the list
	// +optional
	Namespaces []string

	// `resources` in this selector. An empty list implies all kinds in all API groups.
	// +optional
	Resources []GroupResources
}

type ClusterResourceSelector struct {
	// +optional
	RequestSelector

	// `resources` in this selector. An empty list implies all kinds in all API groups.
	// +optional
	Resources []GroupResources
}

type NonResourceSelector struct {
	// +optional
	RequestSelector

	// `nonResourceURLs` is a set of URL paths that should be audited.
	// *s are allowed, but only as the full, final step in the path, and are delimited by the path separator
	// Examples:
	//  "/metrics" - Log requests for apiserver metrics
	//  "/healthz/*" - Log all health checks
	// +optional
	NonResourceURLs []string
}

// GroupResources represents resource kinds in an API group.
type GroupResources struct {
	// `group` is the name of the API group that contains the resources.
	// The empty string represents the core API group.
	// +optional
	Group string

	// `resources` is a list of resources in this group.
	//
	// For example:
	// 'pods' matches pods.
	// 'pods/log' matches the log subresource of pods.
	// '*' matches all resources and their subresources.
	// 'pods/*' matches all subresources of pods.
	// 'pods*' would not match any subresources of pods because it is before the path separator.
	// '*/scale' matches all scale subresources.
	//
	// If wildcard is present, the validation rule will ensure resources do not
	// overlap with each other.
	//
	// An empty list implies all resources and subresources in these API groups apply.
	// +optional
	Resources []string

	// `objectNames` is a list of resource instance names in this group.
	// Using this field requires resources to be specified.
	// An empty list implies that every instance of the resource is matched.
	// +optional
	ObjectNames []string
}