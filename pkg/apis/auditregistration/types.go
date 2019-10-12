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

// AuditClass is a set of rules that match a category of requests. Policies 
// can reference and set a level of detail for auditing request categories 
// defined by such AuditClass objects. 
//
// Operations on AuditClass objects should be considered highly privileged, 
// as modifying them affects which requests are audited.
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
	// 'rules' is the list of rules for matching a category of requests.
	Rules []Rule
}

// Rule defines a criteria for selecting requests for a class, by matching
// their attributes.
type Rule struct {
	// +optional
	BaseSelector
	// +optional
	RequestSelector
}

// BaseSelector is a criteria for matching requests by common attributes.
type BaseSelector struct {
	// `users` is a list of user names used to match 
	// authenticated user in a request, e.g. `system:anonymous`.
	// An empty list or undefined `users` implies any user.
	// +optional
	Users []string
	// `userGroups` is a list of user group names used to
	//  match the user group of a user in a request, e.g.
	// `system:unauthenticated`. A user is considered 
	// matching if it is a member of any of the `userGroups`.
	// An empty list implies any user group.
	// +optional
	UserGroups []string
	// `verbs` is a list of verb names used to match the request verb,
	// e.g. `list`, `create`, `watch` for resource requests or `post`,
	// `put`,`delete` for non-resource requests.
	// An empty list implies any verb.
	// +optional
	Verbs []string
}

// RequestSelector is a union of different types of request matching criteria lists.
// Only one must be defined at a time, i.e. either `groupResourceSelectors` or 
// `nonResourceSelectors` element is permitted.
// +union
type RequestSelector struct {
	// `groupResourceSelectors` is a list of selectors matching 
	// API group resources.
	// +optional
	GroupResourceSelectors []GroupResourceSelector
	// `nonResourceSelectors` is a list of selectors matching 
	// requests that are not related to API resources.
	// +optional
	NonResourceSelectors []NonResourceSelector
}

// GroupResourceSelector matches requests for resources in API groups.
type GroupResourceSelector struct {
	// `group` is an API resource group, e.g. `rbac.authorization.k8s.io`.
	// Use "" for the default group.
	// +optional
	Group string
	// `resources` is a list of selectors matching various API 
	// resource kinds. If `group` is defined, they must be members
	// of the group.
	// +optional
	Resources []ResourceSelector
	// `scope` indicates the scope of the request - cluster, namespaced or any. 
	// In case of namespaced resource requests, it can specify the namespace too.
	// +optional
	Scope ScopeSelector
}

// ResourceSelector matches requests by API resources.
type ResourceSelector struct {
	// `kind` is the resource kind, e.g. "pods"
	Kind string
	// `subresources` is a list of subresources of this resource `kind`.
	// nil = no subresources.
	// '*' = all subresources & non-subresourced
	// '' = non-subresourced
	// +optional
	Subresources []string
	// `objectNames` is a list of names of the objects of this resource kind.
	// +optional
	ObjectNames []string
}

// ScopeSelector matches requests by API resources scope.
// +union
type ScopeSelector struct {
	// `scope` is scope of a resource. It can be one of `Cluster`, 
	// `Namespaced` or `Any` (default if scope is not defined).
	// +unionDiscriminator
	// +optional 
	Scope ScopeType
	// `namespaces` is a list of namespace selectors
	// +optional
	Namespaces []NamespaceSelector
}

type ScopeType string

const (
	ScopeTypeAny = "Any" // default
	ScopeTypeCluster = "Cluster"
	ScopeTypeNamespaced = "Namespaced"
)

// NamespaceSelector matches requests by namespace.
type NamespaceSelector struct {
	// `name` is the namespace name
	Name string
}

// NonResourceSelector selects requests that do not target API resources.
type NonResourceSelector struct {
	// `urls` is a set of URL paths that should be audited.
	// The `*` is allowed, but only as the full, final step in the path, 
	// and it is delimited by the path separator
	// Examples:
	//  "/metrics" - Log requests for apiserver metrics
	//  "/healthz/*" - Log all health checks
	// +optional
	URLs []string
}